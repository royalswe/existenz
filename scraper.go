package main

import (
	"fmt"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
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

// func writeLinksToJSON(linkMap map[string][]*Link) {
// 	// Filter out empty date arrays
// 	filteredLinkMap := make(map[string][]*Link)
// 	for date, links := range linkMap {
// 		if len(links) > 0 {
// 			filteredLinkMap[date] = links
// 		}
// 	}

// 	// Extract keys and sort them in reverse order
// 	keys := make([]string, 0, len(filteredLinkMap))
// 	for date := range filteredLinkMap {
// 		keys = append(keys, date)
// 	}
// 	sort.Sort(sort.Reverse(sort.StringSlice(keys)))

// 	// Create a new ordered slice
// 	orderedEntries := make([]struct {
// 		Date  string  `json:"date"`
// 		Links []*Link `json:"links"`
// 	}, len(keys))

// 	for i, date := range keys {
// 		orderedEntries[i].Date = date
// 		orderedEntries[i].Links = filteredLinkMap[date]
// 	}

// 	// Write the ordered entries to a JSON file
// 	file, err := os.Create("links.json")
// 	if err != nil {
// 		log.Fatalln("Failed to create output JSON file", err)
// 	}
// 	defer file.Close()

// 	encoder := json.NewEncoder(file)
// 	encoder.SetIndent("", "  ")
// 	if err := encoder.Encode(orderedEntries); err != nil {
// 		log.Fatalln("Failed to write JSON data to file", err)
// 	}
// }

func Scrape() {
	fmt.Println("Run scrape function...")
	u := launcher.NewUserMode().
		Leakless(true).
		UserDataDir("tmp/t").
		Set("disable-default-apps").
		Set("no-first-run").
		Set("no-sandbox").
		Set("headless").
		MustLaunch()

	fmt.Println("Connected to https://existenz.se/")
	browser := rod.New().ControlURL(u).MustConnect().NoDefaultDevice()
	page := browser.MustPage("https://existenz.se/")
	page.MustWaitLoad()
	fmt.Println("Browser is connected and page is fully loaded")

	// launcher := launcher.New().Bin("/usr/bin/chromium-browser")
	// launcher.Set("--disable-web-security", "--no-sandbox")
	// url := launcher.MustLaunch()
	// browser := rod.New().ControlURL(url).MustConnect()
	// //browser := rod.New().Timeout(time.Second * 5).MustConnect()

	// //fmt.Printf("js: %x\n\n", md5.Sum([]byte(stealth.JS)))
	// page := stealth.MustPage(browser)
	// page.MustEvalOnNewDocument(stealth.JS)
	// page.MustSetUserAgent(&proto.NetworkSetUserAgentOverride{
	// 	UserAgent:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	// 	Platform:       "Windows",
	// 	AcceptLanguage: "en-US,en;q=0.9",
	// })

	// page.MustNavigate("https://existenz.se/")
	// page.MustWaitStable() // Wait for page to stabilize
	// page.MustWaitLoad()   // Wait for full load
	// fmt.Println("Connected to https://existenz.se/")

	// // Handle Cloudflare challenge if present
	// if challenge := page.MustElement("#challenge-form"); challenge != nil {
	// 	fmt.Println("Cloudflare challenge detected. Waiting for challenge to complete...")
	// 	time.Sleep(5 * time.Second) // Wait for challenge
	// 	page.MustWaitStable()
	// 	fmt.Println("Cloudflare challenge completed.")
	// }

	defer browser.MustClose()
	links := page.MustElement(".links")
	fmt.Println("After links", links)
	//browser := rod.New().MustConnect()

	//page := browser.MustPage("https://existenz.se/")
	// page.MustSetUserAgent(&proto.NetworkSetUserAgentOverride{
	// 	UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	// })

	// defer browser.MustClose()

	// page.MustElement(`input[value="Användarnamn"]`).MustInput("royalswe")
	// page.MustElement(`input[value="Lösenord"]`).MustInput("tr0llet")
	// page.MustElement(`input[type="submit"]`).MustClick()
	// page.MustWaitLoad()
	// page.MustElement(".user-menu")

	// fmt.Println("Logged in")
	// // check if cookie is set
	// linkMap := make(map[string][]*Link)
	// var currentDate string = "Idag"
	// maxLinks := 5
	// count := 0

	// links := page.MustElements(".link")
	// for _, el := range links {
	// 	//href := el.MustElement(`a[target="_blank"]`).MustProperty("href").String()
	// 	//absoluteURL := href

	// 	title := el.MustElement(".text").MustText()
	// 	icon := el.MustElement("img.type").MustProperty("alt").String()
	// 	commentUrl := el.MustElement(".comment-info a").MustProperty("href").String()
	// 	commentNumber := el.MustElement(".comment-info a").MustText()
	// 	nsfw := el.MustHas("img[alt='18+']")

	// 	link := &Link{
	// 		Title:         title,
	// 		Icon:          icon,
	// 		CommentUrl:    commentUrl,
	// 		CommentNumber: commentNumber,
	// 		Nsfw:          nsfw,
	// 	}

	// 	if currentDate != "" {
	// 		linkMap[currentDate] = append(linkMap[currentDate], link)
	// 	}

	// 	// Handle sibling element for comment-date
	// 	// nextSibling := el.MustNext()
	// 	// if nextSibling.HasClass("comment-date") {
	// 	// 	currentDate = nextSibling.MustText()
	// 	// }

	// 	if count++; count >= maxLinks {
	// 		break
	// 	}

	// 	writeLinksToJSON(linkMap)

	// }
	// for date, links := range linkMap {
	// 	fmt.Printf("Date: %s\n", date)
	// 	for _, link := range links {
	// 		fmt.Printf("Title: %s, Icon: %s, Type: %s, Src: %s, CommentUrl: %s, CommentNumber: %s, Nsfw: %t\n",
	// 			link.Title, link.Icon, link.Type, link.Src, link.CommentUrl, link.CommentNumber, link.Nsfw)
	// 	}
	// }
}
