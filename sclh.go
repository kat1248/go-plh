package main

import (
	"context"
	"regexp"

	"crypto/tls"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
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
	maximumNames = 100
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
	// parse flags here so tests do not trip over test flags
	flag.Parse()

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
	start := time.Now()

	regex := regexp.MustCompile(`\r?\n|\r`)
	nameList := regex.Split(r.FormValue("characters"), -1)
	names := nameList[0:min(maximumNames, len(nameList))]

	log.Info("Requested Names [" + strings.Join(names, ", ") + "]")

	defer func(num int) {
		elapsed := time.Since(start).Seconds()
		log.WithFields(log.Fields{
			"count":   num,
			"elapsed": elapsed,
		}).Info("Handled request")
	}(len(names))

	// ---- streaming headers ----
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Transfer-Encoding", "chunked")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	encoder := json.NewEncoder(w)

	// announce total
	encoder.Encode(map[string]any{
		"_meta": "start",
		"total": len(names),
	})
	flusher.Flush()

	ctx := r.Context()

	success, err := loadCharacterIds(ctx, names)
	if !success {
		log.Error(err)
	}

	ch := make(chan *characterResponse)
	var wg sync.WaitGroup

	// ---- worker pool ----
	for _, name := range names {
		name := strings.TrimSpace(name)
		if len(name) < 3 {
			continue
		}

		wg.Add(1)
		go func(n string) {
			defer wg.Done()

			select {
			case ch <- fetchCharacterData(ctx, n):
			case <-ctx.Done():
				return
			}
		}(name)
	}

	// ---- close channel when workers finish ----
	go func() {
		wg.Wait()
		close(ch)
	}()

	//encoder := json.NewEncoder(w)

	sent := 0
	// ---- stream rows as they arrive ----
	for resp := range ch {
		if resp.err != nil {
			log.Error(resp.err)
			continue
		}

		if err := encoder.Encode(resp.char); err != nil {
			// Client disconnected — stop work
			log.Warn("client disconnected during stream")
			return
		}

		// 🚨 critical for real-time delivery
		flusher.Flush()

		sent++

		encoder.Encode(map[string]any{
			"_meta": "progress",
			"sent":  sent,
			"total": len(names),
		})
		flusher.Flush()
	}

	encoder.Encode(map[string]any{
		"_meta": "done",
		"sent":  sent,
		"total": len(names),
	})
	flusher.Flush()
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
