package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	var port int
	flag.IntVar(&port, "port", 8080, "port to listen on")
	flag.Parse()

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.HandleFunc("/info", serveData)
	http.HandleFunc("/", defaultHandler)
	http.HandleFunc("/favicon.ico", faviconHandler)

	log.Println("Listening on port", fmt.Sprint(port))
	http.ListenAndServe(":"+fmt.Sprint(port), nil)
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/favicon.ico")
}

func serveData(w http.ResponseWriter, r *http.Request) {
	names := strings.Split(r.FormValue("characters"), "\n")
	profiles := make([]CharacterData, len(names))
	ch := make(chan *CharacterResponse)
	for _, name := range names {
		go FetchCharacterData(name, ch)
	}
	count := 0
	for range names {
		select {
		case r := <-ch:
			if r.Err != nil {
				log.Println("error:", r.Err)
			} else {
				profiles[count] = *r.Char
				count++
			}
		}
	}
	js, err := json.Marshal(profiles[0:count])
	if err != nil {
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
