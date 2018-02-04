package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func main() {
	var port int
	flag.IntVar(&port, "port", 80, "port to listen on")
	flag.Parse()

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/info", serveData)
	http.HandleFunc("/", defaultHandler)
	http.HandleFunc("/health", healthCheckHandler)
	http.HandleFunc("/favicon.ico", faviconHandler)

	log.Println("Listening on port", fmt.Sprint(port))
	http.ListenAndServe(":"+fmt.Sprint(port), nil)
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
	names := strings.Split(r.FormValue("characters"), "\n")
	profiles := make([]CharacterData, 0)
	ch := make(chan *CharacterResponse, len(names))
	var wg sync.WaitGroup

	for _, name := range names {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			ch <- fetchCharacterData(name)
		}(name)
	}

	wg.Wait()
	close(ch)

	for r := range ch {
		if r.err == nil {
			profiles = append(profiles, *r.char)
		} else {
			log.Println("error:", r.err)
		}
	}

	js, err := json.Marshal(profiles)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)

	log.Println("Handled", fmt.Sprint(len(profiles)), "names")
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
		log.Println(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	if err := tmpl.ExecuteTemplate(w, "layout", nil); err != nil {
		log.Println(err.Error())
		http.Error(w, http.StatusText(500), 500)
	}
}
