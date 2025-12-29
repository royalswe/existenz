package main

import (
	"encoding/json"
	"fmt"
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

var cookies = []*http.Cookie{
	{
		Name:   "PHPSESSID",
		Value:  "doo8jg8va64r2bhs2a557guf47",
		Domain: "existenz.se",
	},
}

func Scrape() {
	// initialize the map that will contain the scraped data
	linkMap := make(map[string][]*Link)
	var currentDate string = "Idag"
	maxLinks := 500
	count := 0

	//... scraping logic
	c := colly.NewCollector(
		colly.AllowURLRevisit(),
		colly.AllowedDomains(),
	)
	//c.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36"
	// set proxy with colly

	// Rotate two socks5 proxies
	// rp, err := proxy.RoundRobinProxySwitcher("socks5://127.0.0.1:9050", "socks5://127.0.0.1:9051")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// c.SetProxyFunc(rp)

	// Set the PHPSESSID cookie
	c.OnRequest(func(r *colly.Request) {
		c.SetCookies("https://existenz.se", cookies)

		// for _, cookie := range cookies {
		// 	r.Headers.Set("Cookie", fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
		// }
	})

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
			err := c.Visit(src)
			if err != nil {
				log.Println("Failed to visit URL:", err)
			}
		}
	})

	c.OnHTML("script", func(e *colly.HTMLElement) {
		link := &Link{}
		redirectSrc := false

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
				// link.src has two urls, keep the second one where https begins second time
				link.Src = "https" + strings.Split(link.Src, "https")[2]
				link.Type = "image"
			} else if strings.Contains(link.Src, "https://snuskhummer.com") {
				// next url is the video which will be picked up by iframe
				redirectSrc = true
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

			if redirectSrc {
				absoluteURL := e.Request.AbsoluteURL(link.Src)
				err := c.Visit(absoluteURL)
				if err != nil {
					log.Println("Failed to visit URL:", err)
				}
			}
		}
	})

	// triggered when a CSS selector matches an element
	c.OnHTML(".link", func(e *colly.HTMLElement) {
		if count >= maxLinks {
			return
		}
		count++

		// Get the href attribute
		href := e.ChildAttr(`a[target="_blank"]`, "href")
		absoluteURL := e.Request.AbsoluteURL(href)

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

		// Check the next sibling element for the comment-date class
		nextSibling := e.DOM.Next()
		if nextSibling.HasClass("comment-date") {
			currentDate = nextSibling.Text()
		}

		err := c.Visit(absoluteURL)
		if err != nil {
			log.Println("Failed to visit URL:", err)
		}
	})

	// triggered when a CSS selector matches a comment date element
	c.OnHTML(".comment-date", func(e *colly.HTMLElement) {
		currentDate = e.Text
		if _, exists := linkMap[currentDate]; !exists {
			linkMap[currentDate] = []*Link{}
		}
	})

	// triggered once scraping is done (e.g., write the data to a JSON file)
	c.OnScraped(func(r *colly.Response) {
		// Filter out empty date arrays
		filteredLinkMap := make(map[string][]*Link)
		for date, links := range linkMap {
			if len(links) > 0 {
				filteredLinkMap[date] = links
			}
		}

		// Extract keys and sort them in reverse order
		keys := make([]string, 0, len(filteredLinkMap))
		for date := range filteredLinkMap {
			keys = append(keys, date)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(keys)))

		// Create a new ordered slice
		orderedEntries := make([]struct {
			Date  string  `json:"date"`
			Links []*Link `json:"links"`
		}, len(keys))

		for i, date := range keys {
			orderedEntries[i].Date = date
			orderedEntries[i].Links = filteredLinkMap[date]
		}

		// Write the ordered entries to a JSON file
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
	// start scraping
	c.Visit("https://existenz.se/")
}

func UpdateCommentNumbers() {
	// Read current links.json

	data, _ := os.ReadFile("links.json")

	var entries []struct {
		Date  string  `json:"date"`
		Links []*Link `json:"links"`
	}

	json.Unmarshal(data, &entries)

	// Create temporary map to store all comments
	commentMap := make(map[string]string)

	c := colly.NewCollector()

	// Set the PHPSESSID cookie
	c.OnRequest(func(r *colly.Request) {
		c.SetCookies("https://existenz.se", cookies)
	})

	c.OnHTML(".link", func(e *colly.HTMLElement) {
		commentNumber := e.ChildText(".comment-info a")
		commentUrl := e.ChildAttr(".comment-info a", "href")
		commentMap[commentUrl] = commentNumber
	})

	c.Visit("https://existenz.se")

	c.Wait()
	// Update links.json with collected comments
	fmt.Println("Updating comment numbers...")
	for _, entry := range entries {
		for _, link := range entry.Links {
			// Check if the link has a comment URL
			if number, exists := commentMap[link.CommentUrl]; exists {
				link.CommentNumber = number
			}
		}
	}

	// Write updated data back to links.json
	file, _ := os.Create("links.json")
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.Encode(entries)
}
