package main

import (
	"encoding/json"
	"net/http"
	"os"
)

func main() {
	Scrape()

	http.HandleFunc("GET /links", func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle preflight request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		file, err := os.Open("links.json")
		if err != nil {
			http.Error(w, "Unable to open links.json", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		var links interface{}
		if err := json.NewDecoder(file).Decode(&links); err != nil {
			http.Error(w, "Unable to parse links.json", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(links); err != nil {
			http.Error(w, "Unable to encode response", http.StatusInternalServerError)
		}
	})
	// Serve static files from the `ui` directory
	http.Handle("/", http.FileServer(http.Dir("ui")))

	http.ListenAndServe(":8081", nil)
}
