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
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
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
	maxWorkers   = 10
)

var (
	port         int
	debugMode    bool
	localMode    bool
	analyzeKills bool

	httpClient *http.Client
)

var newlineRegex = regexp.MustCompile(`\r?\n|\r`)

func init() {
	flag.IntVar(&port, "port", 80, "port to listen on")
	flag.BoolVar(&debugMode, "debug", false, "debug mode switch")
	flag.BoolVar(&localMode, "local", false, "run server locally without TLS")
	flag.BoolVar(&analyzeKills, "kills", false, "do more analysis on kills")
}

func setupHTTPClient() {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 3
	retryClient.HTTPClient.Timeout = 10 * time.Second

	cacheTransport := httpcache.NewTransport(httpcache.NewMemoryCache())
	cacheTransport.Transport = http.DefaultTransport

	retryClient.HTTPClient.Transport = cacheTransport
	httpClient = retryClient.HTTPClient
}

func setupLogging() {
	if debugMode {
		log.SetOutput(os.Stdout)
		log.SetLevel(log.DebugLevel)
	}
}

func waitForShutdown(ctx context.Context, server *http.Server) {
	<-ctx.Done()

	log.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.WithError(err).Error("graceful shutdown failed")
	} else {
		log.Info("server shut down cleanly")
	}
}

func parsePort() int {
	if ports := os.Getenv("PORT"); ports != "" {
		p, err := strconv.Atoi(ports)
		if err != nil || p <= 0 || p > 65535 {
			log.Fatalf("Invalid PORT value: %q", ports)
		}
		return p
	}
	return port
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// ---- TLS / transport ----
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		}

		// ---- MIME sniffing ----
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// ---- clickjacking ----
		w.Header().Set("X-Frame-Options", "DENY")

		// ---- XSS ----
		w.Header().Set("X-XSS-Protection", "0") // modern browsers use CSP

		// ---- content policy ----
		w.Header().Set(
			"Content-Security-Policy",
			"default-src 'self'; img-src 'self' https://images.evetech.net; style-src 'self' 'unsafe-inline'",
		)

		// ---- referrer ----
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		next.ServeHTTP(w, r)
	})
}

func main() {
	flag.Parse()
	port = parsePort()

	setupLogging()
	setupHTTPClient()

	// pprof server
	go func() {
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			log.WithError(err).Warn("pprof server stopped")
		}
	}()

	checkESIConnectivity()

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("images"))))
	mux.HandleFunc("/info", serveData)
	mux.HandleFunc("/", defaultHandler)
	mux.HandleFunc("/health", healthCheckHandler)
	mux.HandleFunc("/favicon.ico", faviconHandler)

	handler := securityHeaders(mux)

	certManager := &autocert.Manager{
		Cache:       autocert.DirCache("./certs"),
		Prompt:      autocert.AcceptTOS,
		Email:       "kat1248@gmail.com",
		HostPolicy:  autocert.HostWhitelist("tiggs.ddns.net", "sclh.ddns.net"),
		RenewBefore: 30 * 24 * time.Hour,
	}

	server := &http.Server{
		Addr:         ":443",
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0, // streaming
		IdleTimeout:  60 * time.Second,
		TLSConfig: &tls.Config{
			GetCertificate: certManager.GetCertificate,
			MinVersion:     tls.VersionTLS12,
			CurvePreferences: []tls.CurveID{
				tls.X25519,
				tls.CurveP256,
			},
			PreferServerCipherSuites: true,
		},
	}

	// ---- signal handling ----
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	// ---- start servers ----
	if localMode || debugMode {
		log.Infof("Listening on :%d (HTTP)", port)

		go func() {
			if err := http.ListenAndServe(fmt.Sprintf(":%d", port), mux); err != nil &&
				err != http.ErrServerClosed {
				log.WithError(err).Fatal("HTTP server failed")
			}
		}()

		waitForShutdown(ctx, server)
		return
	}

	// HTTP → HTTPS redirect + ACME challenge
	go func() {
		if err := http.ListenAndServe(":80", certManager.HTTPHandler(nil)); err != nil &&
			err != http.ErrServerClosed {
			log.WithError(err).Fatal("ACME HTTP server failed")
		}
	}()

	go func() {
		log.Info("Listening on :443 (HTTPS)")
		if err := server.ListenAndServeTLS("", ""); err != nil &&
			err != http.ErrServerClosed {
			log.WithError(err).Fatal("HTTPS server failed")
		}
	}()

	waitForShutdown(ctx, server)
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
	ctx := r.Context()

	raw := newlineRegex.Split(r.FormValue("characters"), -1)

	seen := make(map[string]struct{})
	names := make([]string, 0, len(raw))

	for _, n := range raw {
		n = strings.TrimSpace(n)
		if len(n) < 3 {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		names = append(names, n)
		if len(names) >= maximumNames {
			break
		}
	}

	log.WithField("count", len(names)).Info("request received")

	defer func() {
		log.WithFields(log.Fields{
			"count":   len(names),
			"elapsed": time.Since(start).Seconds(),
		}).Info("request completed")
	}()

	// Headers for NDJSON streaming
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	enc := json.NewEncoder(w)

	// announce start
	if err := enc.Encode(map[string]any{
		"_meta": "start",
		"total": len(names),
	}); err != nil {
		log.WithError(err).Warn("failed to write start response")
		return
	}
	flusher.Flush()

	if ok, err := loadCharacterIds(ctx, names); !ok {
		log.WithError(err).Warn("failed to preload character IDs")
	}

	jobs := make(chan string)
	results := make(chan *characterResponse, maxWorkers)

	var wg sync.WaitGroup

	// workers
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for name := range jobs {
				select {
				case results <- fetchCharacterData(ctx, name):
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// feed jobs
	go func() {
		for _, name := range names {
			select {
			case jobs <- name:
			case <-ctx.Done():
				break
			}
		}
		close(jobs)
	}()

	// close results
	go func() {
		wg.Wait()
		close(results)
	}()

	sent := 0

	for resp := range results {
		if resp.err != nil {
			log.WithError(resp.err).Error("fetch failed")
			continue
		}

		if err := enc.Encode(resp.char); err != nil {
			log.Warn("client disconnected")
			return
		}
		flusher.Flush()

		sent++
	}

	if err := enc.Encode(map[string]any{
		"_meta": "done",
		"sent":  sent,
		"total": len(names),
	}); err != nil {
		log.WithError(err).Warn("failed to write final response")
		return
	}
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

func checkESIConnectivity() {
	// create ESI client
	client := goesi.NewAPIClient(nil, userAgent)
	// call Status endpoint
	status, _, err := client.ESI.StatusApi.GetStatus(context.Background(), nil)
	if err != nil {
		log.WithError(err).Fatal("ESI unavailable")
	}
	// print current status
	fmt.Println("Players online: ", status.Players)
}
