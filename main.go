package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	useFlareSolverr, _ := strconv.ParseBool(os.Getenv("USE_FLARESOLVERR"))

	// Run scraper on startup, then every 24 hours
	go func() {
		for {
			Scrape(useFlareSolverr)
			now := time.Now()
			next := now.Add(time.Hour * 24)
			next = time.Date(next.Year(), next.Month(), next.Day(), 0, 10, 0, 0, next.Location())
			time.Sleep(time.Until(next))
		}
	}()

	// update comment numbers every 10 minute
	// go func() {
	// 	for {
	// 		UpdateCommentNumbers(useFlareSolverr)
	// 		time.Sleep(10 * time.Minute)
	// 	}
	// }()

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
