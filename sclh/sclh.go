package main

// uses: go get github.com/patrickmn/go-cache

import (
	. "character"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.HandleFunc("/info", serveData)
	http.HandleFunc("/", defaultHandler)

	log.Println("Listening...")
	http.ListenAndServe(":8080", nil)
}

func serveData(w http.ResponseWriter, r *http.Request) {
	names := strings.Split(r.FormValue("characters"), "\n")
	profiles := make([]CharacterData, len(names))
	index := 0
	for _, name := range names {
		cd, err := FetchCharacterData(name)
		if err != nil {
			log.Println("error:", err)
		} else {
			profiles[index] = cd
			index++
		}
	}
	js, err := json.Marshal(profiles[0:index])
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

	tmpl, _ := template.ParseFiles(lp, fp)
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
