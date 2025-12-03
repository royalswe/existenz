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
type FlareSolverrRequest struct {
	Cmd        string `json:"cmd"`
	URL        string `json:"url"`
	MaxTimeout int    `json:"maxTimeout"`
}

type FlareSolverrResponse struct {
	Solution FlareSolverrSolution `json:"solution"`
	Status   string               `json:"status"`
	Message  string               `json:"message"`
}

type FlareSolverrSolution struct {
	URL       string `json:"url"`
	Status    int    `json:"status"`
	UserAgent string `json:"userAgent"`
	Response  string `json:"response"`
}

func getSiteHTML() (string, error) {
	fmt.Println("Getting site HTML from FlareSolverr...")
	reqBody := FlareSolverrRequest{
		Cmd:        "request.get",
		URL:        "https://existenz.se/",
		MaxTimeout: 60000,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal flaresolverr request: %w", err)
	}

	resp, err := http.Post("http://flaresolverr:8191/v1", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to send request to flaresolverr: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read flaresolverr response body: %w", err)
	}

	var flaresolverrResponse FlareSolverrResponse
	if err := json.Unmarshal(body, &flaresolverrResponse); err != nil {
		return "", fmt.Errorf("failed to unmarshal flaresolverr response: %w", err)
	}

	if flaresolverrResponse.Status != "ok" {
		return "", fmt.Errorf("flaresolverr returned an error: %s", flaresolverrResponse.Message)
	}

	fmt.Println("Successfully got site HTML from FlareSolverr")
	return flaresolverrResponse.Solution.Response, nil
}

func Scrape() {
	// initialize the map that will contain the scraped data
	linkMap := make(map[string][]*Link)
	var currentDate string = "Idag"
	maxLinks := 100
	count := 0
	
	// Temporary store for links that need their redirect URLs followed
	tempLinkStore := make(map[string]*Link)

	// Colly collector
	c := colly.NewCollector(
		colly.AllowURLRevisit(),
		colly.MaxDepth(2), // We need to go one level deeper to follow redirects
	)
	
	// Set the User-Agent to mimic a real browser
	c.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36"

	// triggered when the scraper encounters an error
	c.OnError(func(r *colly.Response, err error) {
		fmt.Printf("Something went wrong on %s: %v\n", r.Request.URL.String(), err)
	})
	
	// ... (rest of the code will be modified in subsequent steps)

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
		redirectionUrl := e.ChildAttr("a", "href")
		if redirectionUrl == "" {
			return // Skip if no redirection URL found
		}

		// Make the redirection URL absolute
		absoluteRedirectionUrl := e.Request.AbsoluteURL(redirectionUrl)

		// Store the link in tempLinkStore, keyed by its absolute redirection URL
		tempLinkStore[absoluteRedirectionUrl] = link

		// Add to the main linkMap (will be fully populated after redirect)
		if currentDate != "" {
			linkMap[currentDate] = append(linkMap[currentDate], link)
		}
		count++

		// Visit the redirection URL to get the final content URL
		e.Request.Visit(absoluteRedirectionUrl)
	})

	// This handler processes the redirected pages to extract the final content URL
	c.OnHTML("html", func(e *colly.HTMLElement) {
		requestURL := e.Request.URL.String()

		// Only process if this URL is one we are tracking in tempLinkStore
		if link, exists := tempLinkStore[requestURL]; exists {
			// Look for an iframe containing the content
			e.ForEach("iframe", func(_ int, el *colly.HTMLElement) {
				src := el.Attr("src")
				if src != "" && link.Src == "" { // Only set if not already set by a previous iframe
					link.Src = src
					// Refine type based on src content if needed
					if strings.Contains(src, "youtube.com/embed/") || strings.Contains(src, "youtube.com/watch?v=") {
						link.Type = "youtube"
					} else if strings.Contains(src, "player.vimeo.com/video/") {
						link.Type = "video"
					} else {
						link.Type = "iframe" // Default to iframe type if found
					}
				}
			})

			// Look for scripts that might contain video IDs or redirect URLs
			e.ForEach("script", func(_ int, el *colly.HTMLElement) {
				scriptText := el.Text
				if link.Src == "" { // Only process if src is not already found by an iframe
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
								link.Src = urlParts[0]
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
				}
			})
		}
	})

	// This handler is for initializing the map for a new date.
	c.OnHTML(".comment-date", func(e *colly.HTMLElement) {
		currentDate = e.Text
		if _, exists := linkMap[currentDate]; !exists {
			linkMap[currentDate] = []*Link{}
		}
	})

	// triggered once scraping is done
	c.OnScraped(func(r *colly.Response) {
		// Filter out empty date arrays
		filteredLinkMap := make(map[string][]*Link)
		for date, links := range linkMap {
			if len(links) > 0 {
				filteredLinkMap[date] = links
			}
		}

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
}

func UpdateCommentNumbers() {
	data, _ := os.ReadFile("links.json")

	var entries []struct {
		Date  string  `json:"date"`
		Links []*Link `json:"links"`
	}

	json.Unmarshal(data, &entries)

	htmlContent, err := getSiteHTML()
	if err != nil {
		log.Printf("Failed to get site HTML for comments: %v", err)
		return
	}

	tmpFile, err := os.Create("tmp/comments.html")
	if err != nil {
		log.Printf("Failed to create temporary comments file: %v", err)
		return
	}
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(htmlContent)
	if err != nil {
		log.Printf("Failed to write to temporary comments file: %v", err)
		return
	}
	tmpFile.Close()


	commentMap := make(map[string]string)

	t := &http.Transport{}
	t.RegisterProtocol("file", http.NewFileTransport(http.Dir("/app")))
	c := colly.NewCollector()
	c.WithTransport(t)


	c.OnHTML(".link", func(e *colly.HTMLElement) {
		commentNumber := e.ChildText(".comment-info a")
		commentUrl := e.ChildAttr(".comment-info a", "href")
		commentMap[commentUrl] = commentNumber
	})

	c.Visit("file:///tmp/comments.html")

	c.Wait()
	fmt.Println("Updating comment numbers...")
	for _, entry := range entries {
		for _, link := range entry.Links {
			if number, exists := commentMap[link.CommentUrl]; exists {
				link.CommentNumber = number
			}
		}
	}

	file, _ := os.Create("links.json")
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.Encode(entries)
}
