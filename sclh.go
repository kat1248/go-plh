package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sethgrid/pester"
	log "github.com/sirupsen/logrus"
)

const (
	maximumNames = 50
)

var (
	port           int  // which port to listen on
	debugMode      bool // are we in debug mode
	localTransport = &http.Transport{DisableKeepAlives: true}
	localClient    *pester.Client
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
	flag.Parse()

	if debugMode {
		log.SetOutput(os.Stdout)
	}

	rand.Seed(time.Now().Unix())

	localClient = pester.New()
	localClient.Concurrency = 3
	localClient.MaxRetries = 5
	localClient.Backoff = pester.ExponentialJitterBackoff
	localClient.Transport = localTransport
	localClient.RetryOnHTTP429 = true

}

func main() {
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("images"))))
	http.HandleFunc("/info", serveData)
	http.HandleFunc("/", defaultHandler)
	http.HandleFunc("/health", healthCheckHandler)
	http.HandleFunc("/favicon.ico", faviconHandler)

	log.Println("Listening on port", fmt.Sprint(port))
	log.Println(http.ListenAndServe(":"+fmt.Sprint(port), nil))
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
	nameList := strings.Split(r.FormValue("characters"), "\n")
	names := nameList[0:min(maximumNames, len(nameList))]
	log.Info("Requested Names [" + strings.Join(names[:], ", ") + "]")
	defer func(start time.Time, num int) {
		elapsed := time.Since(start).Seconds()
		log.WithFields(log.Fields{
			"count":   num,
			"elapsed": elapsed,
		}).Info("Handled request")
	}(time.Now(), len(names))

	profiles := make([]characterData, 0)
	ch := make(chan *characterResponse, len(names))
	var wg sync.WaitGroup

	for _, name := range names {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			if len(name) < 3 {
				return
			}
			ch <- fetchCharacterData(name)
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
