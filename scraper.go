package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/gocolly/colly/v2"
)

type Link struct {
	Title         string `json:"title"`
	Icon          string `json:"icon"`
	Type          string `json:"type"`
	Src           string `json:"src"`
	CommentUrl    string `json:"comment_url"`
	CommentNumber string `json:"comment_number"`
	Nsfw          bool   `json:"nsfw"`
}

// Structs for FlareSolverr
type FlareSolverrCookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain,omitempty"`
}

type FlareSolverrRequest struct {
	Cmd        string               `json:"cmd"`
	URL        string               `json:"url"`
	MaxTimeout int                  `json:"maxTimeout"`
	Cookies    []FlareSolverrCookie `json:"cookies,omitempty"`
}

type FlareSolverrResponse struct {
	Solution FlareSolverrSolution `json:"solution"`
	Status   string               `json:"status"`
	Message  string               `json:"message"`
}

type FlareSolverrSolution struct {
	URL       string            `json:"url"`
	Status    int               `json:"status"`
	Headers   map[string]string `json:"headers"`
	Response  string            `json:"response"`
	Cookies   []any             `json:"cookies"`
	UserAgent string            `json:"userAgent"`
}

// FlareSolverrTransport implements http.RoundTripper to proxy requests through FlareSolverr
type FlareSolverrTransport struct{}

var cookies = []*http.Cookie{
	{
		Name:   "PHPSESSID",
		Value:  "doo8jg8va64r2bhs2a557guf47",
		Domain: "existenz.se",
	},
}

func (t *FlareSolverrTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	fmt.Printf("FlareSolverr: attempting to proxy request for %s\n", req.URL.String())

	// 1. Create FlareSolverr request payload
	var fsCookies []FlareSolverrCookie
	for _, c := range cookies {
		fsCookies = append(fsCookies, FlareSolverrCookie{
			Name:   c.Name,
			Value:  c.Value,
			Domain: c.Domain,
		})
	}

	fsReq := FlareSolverrRequest{
		Cmd:        "request." + strings.ToLower(req.Method),
		URL:        req.URL.String(),
		MaxTimeout: 60000,
		Cookies:    fsCookies,
	}

	jsonData, err := json.Marshal(fsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal flaresolverr request: %w", err)
	}

	// 2. Send request to FlareSolverr
	fsResp, err := http.Post("http://flaresolverr:8191/v1", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to send request to flaresolverr: %w", err)
	}
	defer fsResp.Body.Close()

	fsBody, err := io.ReadAll(fsResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read flaresolverr response body: %w", err)
	}

	var flaresolverrResponse FlareSolverrResponse
	if err := json.Unmarshal(fsBody, &flaresolverrResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal flaresolverr response: %s", string(fsBody))
	}

	if flaresolverrResponse.Status != "ok" {
		return nil, fmt.Errorf("flaresolverr error for URL %s: %s", req.URL.String(), flaresolverrResponse.Message)
	}
	fmt.Printf("FlareSolverr: successfully got response for %s\n", req.URL.String())

	// 3. Create a valid http.Response from the FlareSolverr solution
	solution := flaresolverrResponse.Solution
	resp := &http.Response{
		StatusCode: solution.Status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(solution.Response)),
		Request:    req,
	}

	// It's important to set the Content-Type header for Colly to process the response correctly
	contentType := solution.Headers["content-type"]
	if contentType == "" {
		contentType = "text/html; charset=utf-8" // Default to HTML
	}
	resp.Header.Set("Content-Type", contentType)

	return resp, nil
}

func Scrape() {
	// initialize the map that will contain the scraped data
	linkMap := make(map[string][]*Link)
	var currentDate string = "Idag"
	maxLinks := 15
	count := 0

	// Temporary store for links that need their redirect URLs followed
	tempLinkStore := make(map[string]*Link)
	var storeMutex = &sync.Mutex{}

	// Colly collector
	c := colly.NewCollector(
		colly.AllowURLRevisit(),
		colly.MaxDepth(2), // We need to go one level deeper to follow redirects
		colly.Async(true), // Enable async for parallel scraping
	)

	// Limit the number of threads to 2 and add a random delay
	c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 2})

	// Set the custom transport to use FlareSolverr
	c.WithTransport(&FlareSolverrTransport{})

	// Set a real User-Agent, though FlareSolverr will likely use its own
	c.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36"

	// triggered when the scraper encounters an error
	c.OnError(func(r *colly.Response, err error) {
		fmt.Printf("Something went wrong on %s: %v\n", r.Request.URL.String(), err)
	})

	c.OnHTML(".link", func(e *colly.HTMLElement) {
		if count >= maxLinks {
			return
		}

		link := &Link{
			Title:         e.ChildText(".text"),
			Icon:          e.ChildAttr("img.type", "alt"), // Preserve original icon name
			CommentUrl:    e.ChildAttr(".comment-info a", "href"),
			CommentNumber: e.ChildText(".comment-info a"),
			Nsfw:          e.ChildAttr(`img[alt="18+"]`, "alt") != "",
		}

		// Determine initial type based on icon
		switch link.Icon {
		case "Film":
			link.Type = "video"
		case "Bild":
			link.Type = "image"
		case "Hemsida":
			link.Type = "website"
		case "Ljud":
			link.Type = "audio"
		case "Spel":
			link.Type = "game"
		default:
			link.Type = "unknown"
		}

		// Get the redirection URL from the <a> tag inside the .link div
		redirectionUrl := e.ChildAttr(`a[target="_blank"]`, "href")
		if redirectionUrl == "" {
			return // Skip if no redirection URL found
		}

		// Make the redirection URL absolute
		absoluteRedirectionUrl := e.Request.AbsoluteURL(redirectionUrl)

		storeMutex.Lock()
		// Store the link in tempLinkStore, keyed by its absolute redirection URL
		tempLinkStore[absoluteRedirectionUrl] = link

		// Add to the main linkMap (will be fully populated after redirect)
		if currentDate != "" {
			linkMap[currentDate] = append(linkMap[currentDate], link)
		}
		count++
		storeMutex.Unlock()

		// Visit the redirection URL to get the final content URL
		e.Request.Visit(absoluteRedirectionUrl)
	})

	// This handler processes the redirected pages to extract the final content URL
	c.OnHTML("html", func(e *colly.HTMLElement) {
		requestURL := e.Request.URL.String()

		storeMutex.Lock()
		// Only process if this URL is one we are tracking in tempLinkStore
		link, exists := tempLinkStore[requestURL]
		storeMutex.Unlock()

		if exists {
			var urlsToVisit []string

			// Look for an iframe containing the content
			e.ForEach("iframe", func(_ int, el *colly.HTMLElement) {
				src := el.Attr("src")
				if src == "" {
					return
				}
				absoluteSrc := e.Request.AbsoluteURL(src)

				if strings.Contains(absoluteSrc, "funfunfun.se") {
					storeMutex.Lock()
					if _, found := tempLinkStore[absoluteSrc]; !found {
						tempLinkStore[absoluteSrc] = link
						urlsToVisit = append(urlsToVisit, absoluteSrc)
					}
					storeMutex.Unlock()
				} else {
					storeMutex.Lock()
					if link.Src == "" { // Only set if not already set
						link.Src = absoluteSrc
						// Refine type based on src content
						if strings.Contains(absoluteSrc, "youtube.com/embed/") || strings.Contains(absoluteSrc, "youtube.com/watch?v=") {
							link.Type = "youtube"
						} else if strings.Contains(absoluteSrc, "player.vimeo.com/video/") {
							link.Type = "video"
						} else {
							link.Type = "iframe"
						}
					}
					storeMutex.Unlock()
				}
			})

			// Look for scripts that might contain video IDs or redirect URLs
			e.ForEach("script", func(_ int, el *colly.HTMLElement) {
				scriptText := el.Text

				storeMutex.Lock()
				// Allow script to overwrite a potentially generic iframe src
				// if link.Src == "" {
				if strings.Contains(scriptText, "videoId") {
					parts := strings.Split(scriptText, "videoId: '")
					if len(parts) > 1 {
						videoIDParts := strings.Split(parts[1], "',")
						if len(videoIDParts) > 0 {
							link.Type = "youtube"
							link.Src = videoIDParts[0]
						}
					}
				} else if strings.Contains(scriptText, "function countdown()") {
					parts := strings.Split(scriptText, "top.location.href = '")
					if len(parts) > 1 {
						urlParts := strings.Split(parts[1], "'")
						if len(urlParts) > 0 {
							newSrc := urlParts[0] // Keep it relative for the weird image URL parsing
							if strings.Contains(newSrc, "snuskhummer.com") {
								absoluteSrc := e.Request.AbsoluteURL(newSrc)
								if _, found := tempLinkStore[absoluteSrc]; !found {
									tempLinkStore[absoluteSrc] = link
									urlsToVisit = append(urlsToVisit, absoluteSrc)
								}
							} else {
								link.Src = newSrc
								link.Type = "redirect"
								if strings.Contains(link.Src, "https://existenz.se/amedia/?typ=bild&url=") {
									urlParts2 := strings.Split(link.Src, "https")
									if len(urlParts2) > 2 {
										link.Src = "https" + urlParts2[2]
										link.Type = "image"
									}
								}
							}
						}
					}
				} else if strings.Contains(scriptText, "top.location") {
					parts := strings.Split(scriptText, "top.location = '")
					if len(parts) > 1 {
						urlParts := strings.Split(parts[1], "'")
						if len(urlParts) > 0 {
							link.Src = urlParts[0]
							link.Type = "redirect"
							if strings.Contains(link.Src, "https://www.youtube.com/shorts/") {
								urlParts2 := strings.Split(link.Src, "https://www.youtube.com/shorts/")
								if len(urlParts2) > 1 {
									link.Src = urlParts2[1]
									link.Type = "youtube"
								}
							}
						}
					}
				}
				// }
				storeMutex.Unlock()
			})

			// Visit all collected URLs
			for _, url := range urlsToVisit {
				e.Request.Visit(url)
			}
		}
	})

	// This handler is for initializing the map for a new date.
	c.OnHTML(".comment-date", func(e *colly.HTMLElement) {
		storeMutex.Lock()
		currentDate = e.Text
		if _, exists := linkMap[currentDate]; !exists {
			linkMap[currentDate] = []*Link{}
		}
		storeMutex.Unlock()
	})

	// triggered once scraping is done
	c.OnScraped(func(r *colly.Response) {
		// Filter out empty date arrays
		filteredLinkMap := make(map[string][]*Link)
		storeMutex.Lock()
		for date, links := range linkMap {
			if len(links) > 0 {
				filteredLinkMap[date] = links
			}
		}
		storeMutex.Unlock()

		keys := make([]string, 0, len(filteredLinkMap))
		for date := range filteredLinkMap {
			keys = append(keys, date)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(keys)))

		orderedEntries := make([]struct {
			Date  string  `json:"date"`
			Links []*Link `json:"links"`
		}, len(keys))

		for i, date := range keys {
			orderedEntries[i].Date = date
			orderedEntries[i].Links = filteredLinkMap[date]
		}

		file, err := os.Create("links.json")
		if err != nil {
			log.Fatalln("Failed to create output JSON file", err)
		}
		defer file.Close()

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(orderedEntries); err != nil {
			log.Fatalln("Failed to write JSON data to file", err)
		}
	})

	c.Visit("https://existenz.se/")
	c.Wait() // Wait for all async scraping to complete
}

func UpdateCommentNumbers() {
	data, err := os.ReadFile("links.json")
	if err != nil {
		log.Printf("Failed to read links.json for comment update: %v", err)
		return
	}

	var entries []struct {
		Date  string  `json:"date"`
		Links []*Link `json:"links"`
	}

	if err := json.Unmarshal(data, &entries); err != nil {
		log.Printf("Failed to unmarshal links.json for comment update: %v", err)
		return
	}

	commentMap := make(map[string]string)
	var mapMutex = &sync.Mutex{}

	c := colly.NewCollector(colly.Async(true))
	c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 2})
	c.WithTransport(&FlareSolverrTransport{}) // Use FlareSolverr for comment updates too

	c.OnError(func(r *colly.Response, err error) {
		fmt.Printf("Error during comment update scrape on %s: %v\n", r.Request.URL.String(), err)
	})

	c.OnHTML(".link", func(e *colly.HTMLElement) {
		commentNumber := e.ChildText(".comment-info a")
		commentUrl := e.ChildAttr(".comment-info a", "href")
		mapMutex.Lock()
		commentMap[commentUrl] = commentNumber
		mapMutex.Unlock()
	})

	// Visit the main page to get the latest comment numbers
	c.Visit("https://existenz.se/")

	c.Wait()
	fmt.Println("Updating comment numbers...")

	mapMutex.Lock()
	for _, entry := range entries {
		for _, link := range entry.Links {
			if number, exists := commentMap[link.CommentUrl]; exists {
				link.CommentNumber = number
			}
		}
	}
	mapMutex.Unlock()

	file, err := os.Create("links.json")
	if err != nil {
		log.Printf("Failed to create links.json for comment update: %v", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(entries); err != nil {
		log.Printf("Failed to write updated JSON data to file: %v", err)
	}
}
