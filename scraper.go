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
	htmlContent, err := getSiteHTML()
	if err != nil {
		log.Printf("Failed to get site HTML: %v.", err)
		return
	}

	// Write HTML to a temporary file
	tmpFile, err := os.Create("tmp/existenz.html")
	if err != nil {
		log.Printf("Failed to create temporary file: %v", err)
		return
	}
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(htmlContent)
	if err != nil {
		log.Printf("Failed to write to temporary file: %v", err)
		return
	}
	tmpFile.Close()

	// initialize the map that will contain the scraped data
	linkMap := make(map[string][]*Link)
	var currentDate string = "Idag"
	maxLinks := 100
	count := 0

	// Colly collector with file transport
	t := &http.Transport{}
	t.RegisterProtocol("file", http.NewFileTransport(http.Dir("/app")))
	c := colly.NewCollector(
		colly.AllowURLRevisit(),
	)
	c.WithTransport(t)

	// triggered when the scraper encounters an error
	c.OnError(func(_ *colly.Response, err error) {
		fmt.Println("Something went wrong: ", err)
	})

	c.OnHTML("body iframe", func(e *colly.HTMLElement) {
		src := e.Attr("src")
		if src == "" {
			return
		}
		if links, exists := linkMap[currentDate]; exists && len(links) > 0 {
			link := links[len(links)-1]
			link.Type = "iframe"
			link.Src = src
		}
		if strings.Contains(src, "https://funfunfun.se") {
			// Visiting external links might not work as expected with file transport
			// log.Println("Skipping external visit in file mode:", src)
		}
	})

	c.OnHTML("script", func(e *colly.HTMLElement) {
		link := &Link{}

		if strings.Contains(e.Text, "videoId") {
			videoID := strings.Split(e.Text, "videoId: '")[1]
			videoID = strings.Split(videoID, "',")[0]
			link.Type = "youtube"
			link.Src = videoID
		} else if strings.Contains(e.Text, "function countdown()") {
			start := strings.Index(e.Text, "top.location.href = '") + len("top.location.href = '")
			end := strings.Index(e.Text[start:], "'") + start
			link.Src = e.Text[start:end]
			link.Type = "redirect"

			if strings.Contains(link.Src, "https://existenz.se/amedia/?typ=bild&url=") {
				link.Src = "https" + strings.Split(link.Src, "https")[2]
				link.Type = "image"
			}
		} else if strings.Contains(e.Text, "top.location") {
			start := strings.Index(e.Text, "top.location = '") + len("top.location = '")
			end := strings.Index(e.Text[start:], "'") + start
			if end > start {
				link.Src = e.Text[start:end]
				link.Type = "redirect"

				if strings.Contains(link.Src, "https://www.youtube.com/shorts/") {
					link.Src = strings.Split(link.Src, "https://www.youtube.com/shorts/")[1]
					link.Type = "youtube"
				}
			}
		}

		if link.Type != "" && link.Src != "" {
			if links, exists := linkMap[currentDate]; exists && len(links) > 0 {
				links[len(links)-1].Type = link.Type
				links[len(links)-1].Src = link.Src
			}
		}
	})

	// triggered when a CSS selector matches an element
	c.OnHTML(".link", func(e *colly.HTMLElement) {
		if count >= maxLinks {
			return
		}
		count++

		link := &Link{
			Title:         e.ChildText(".text"),
			Icon:          e.ChildAttr("img.type", "alt"),
			CommentUrl:    e.ChildAttr(".comment-info a", "href"),
			CommentNumber: e.ChildText(".comment-info a"),
			Nsfw:          e.ChildAttr(`img[alt="18+"]`, "alt") != "",
		}

		if currentDate != "" {
			linkMap[currentDate] = append(linkMap[currentDate], link)
		}

		nextSibling := e.DOM.Next()
		if nextSibling.HasClass("comment-date") {
			currentDate = nextSibling.Text()
		}
	})

	// triggered when a CSS selector matches a comment date element
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

	c.Visit("file:///tmp/existenz.html")
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
