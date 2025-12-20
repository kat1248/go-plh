package main

import (
	"context"

	"crypto/tls"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"

	"github.com/antihax/goesi"
	json "github.com/goccy/go-json"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/sandrolain/httpcache"
	log "github.com/sirupsen/logrus"
)

const (
	maximumNames = 50
	userAgent    = "https://sclh.ddns.net Maintainer: kat1248@gmail.com"
)

var (
	port         int  // which port to listen on
	debugMode    bool // are we in debug mode
	localMode    bool // are we running locally
	analyzeKills bool // do more analysis on kills
	localClient  *http.Client
)

func init() {
	ports := os.Getenv("PORT")
	if ports != "" {
		port, _ = strconv.Atoi(ports)
		log.Println("Setting port to", fmt.Sprint(port))
	} else {
		flag.IntVar(&port, "port", 80, "port to listen on")
	}

	flag.BoolVar(&debugMode, "debug", false, "debug mode switch")
	flag.BoolVar(&localMode, "local", false, "run server locally without TLS")
	flag.BoolVar(&analyzeKills, "kills", false, "do more analysis on kills")
	flag.Parse()

	if debugMode {
		log.SetOutput(os.Stdout)
	}

	// 1. Create a retryable HTTP client (e.g., from hashicorp)
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 3                          // Configure max retries
	retryClient.HTTPClient.Timeout = 10 * time.Second // Set a timeout for individual requests

	// 2. Wrap the retryable client's transport with the cache transport
	// The httpcache package works as a RoundTripper
	cacheTransport := httpcache.NewTransport(httpcache.NewMemoryCache())

	// Assign the original transport of the retry client to be used by the cache when needed.
	// This means a cache *miss* will trigger the retry logic.
	cacheTransport.Transport = retryClient.HTTPClient.Transport

	// 3. Update the retryable client to use the new, wrapped transport
	retryClient.HTTPClient.Transport = cacheTransport

	// Use the *http.Client* from the retryable client for standard http operations
	localClient = retryClient.HTTPClient
}

func main() {
	// server for pprof
	go func() {
		fmt.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	testESI()

	certManager := &autocert.Manager{
		Cache:      autocert.DirCache("./certs"),
		Prompt:     autocert.AcceptTOS,
		Email:      "kat1248@gmail.com",
		HostPolicy: autocert.HostWhitelist("tiggs.ddns.net", "sclh.ddns.net"),
	}

	mux := http.NewServeMux()

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("images"))))
	mux.HandleFunc("/info", serveData)
	mux.HandleFunc("/", defaultHandler)
	mux.HandleFunc("/health", healthCheckHandler)
	mux.HandleFunc("/favicon.ico", faviconHandler)

	var httpsPort string = ":443"
	if localMode {
		httpsPort = ":8443"
	}
	server := &http.Server{
		Addr:    httpsPort,
		Handler: mux,
		TLSConfig: &tls.Config{
			GetCertificate: certManager.GetCertificate,
		},
	}

	if debugMode {
		log.Println("Listening on port", fmt.Sprint(port))
		log.Fatal(http.ListenAndServe(":"+fmt.Sprint(port), mux))
	} else {
		log.Println("Secure server", fmt.Sprint(port))
		go http.ListenAndServe(":"+fmt.Sprint(port), certManager.HTTPHandler(nil))

		log.Fatal(server.ListenAndServeTLS("", ""))
	}
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/favicon.ico")
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, `{"alive": true}`)
}

func serveData(w http.ResponseWriter, r *http.Request) {
	regex, _ := regexp.Compile(`\n{2,}`)
	temp := regex.ReplaceAllString(r.FormValue("characters"), "\n")
	nameList := strings.Split(temp, "\n")
	names := nameList[0:min(maximumNames, len(nameList)-1)]
	log.Info("Requested Names [" + strings.Join(names[:], ", ") + "]")

	defer func(start time.Time, num int) {
		elapsed := time.Since(start).Seconds()
		log.WithFields(log.Fields{
			"count":   num,
			"elapsed": elapsed,
		}).Info("Handled request")
	}(time.Now(), len(names))

	success, err := loadCharacterIds(r.Context(), names)
	if !success {
		log.Error(err)
	}

	profiles := make([]characterData, 0)
	ch := make(chan *characterResponse, len(names))
	var wg sync.WaitGroup

	for _, name := range names {
		name := name
		if len(name) < 3 {
			continue
		}
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			ch <- fetchCharacterData(r.Context(), n)
		}(name)
	}

	wg.Wait()
	close(ch)

	for r := range ch {
		if r.err == nil {
			profiles = append(profiles, *r.char)
		} else {
			log.Error(r.err)
		}
	}

	js, err := json.Marshal(profiles)
	if err != nil {
		log.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	lp := filepath.Join("templates", "layout.html")
	fp := filepath.Join("templates", "index.html")

	// Return a 404 if the template doesn't exist
	info, err := os.Stat(fp)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
	}

	// Return a 404 if the request is for a directory
	if info.IsDir() {
		http.NotFound(w, r)
		return
	}

	tmpl, err := template.ParseFiles(lp, fp)
	if err != nil {
		// Log the detailed error
		log.Error(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	if err := tmpl.ExecuteTemplate(w, "layout", nil); err != nil {
		log.Error(err.Error())
		http.Error(w, http.StatusText(500), 500)
	}
}

func testESI() {
	// create ESI client
	client := goesi.NewAPIClient(nil, userAgent)
	// call Status endpoint
	status, _, err := client.ESI.StatusApi.GetStatus(context.Background(), nil)
	if err != nil {
		panic(err)
	}
	// print current status
	fmt.Println("Players online: ", status.Players)
}
