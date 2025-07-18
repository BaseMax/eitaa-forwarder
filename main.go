package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Post struct {
	ID    string `json:"id"`
	Text  string `json:"text,omitempty"`
	Image string `json:"image,omitempty"`
	Video string `json:"video,omitempty"`
	Time  string `json:"time,omitempty"`
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s <channel_username> [--output filename.json]\n", os.Args[0])
		flag.PrintDefaults()
	}
	outputPath := flag.String("output", "posts.json", "Output file to save posts JSON")
	flag.Parse()

	if flag.NArg() < 1 {
		log.Fatal("Channel username is required.")
	}
	channel := flag.Arg(0)

	url := fmt.Sprintf("https://eitaa.com/%s", channel)
	doc, err := fetchHTML(url)
	if err != nil {
		log.Fatalf("Failed to fetch channel: %v", err)
	}

	if !channelExists(doc) {
		log.Fatalf("Channel '%s' does not exist or is not public.", channel)
	}

	posts := extractPosts(doc)

	jsonData, err := json.MarshalIndent(posts, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling JSON: %v", err)
	}

	err = os.WriteFile(*outputPath, jsonData, 0644)
	if err != nil {
		log.Fatalf("Error writing to file: %v", err)
	}

	fmt.Printf("✅ %d posts saved to %s\n", len(posts), *outputPath)
}

func fetchHTML(url string) (*goquery.Document, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error fetching URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, errors.New("non-200 response from server")
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %v", err)
	}
	return doc, nil
}

func channelExists(doc *goquery.Document) bool {
	return doc.Find(".js-widget_message").Length() > 0
}

func extractPosts(doc *goquery.Document) []Post {
	var posts []Post

	doc.Find(".js-widget_message").Each(func(i int, s *goquery.Selection) {
		var post Post

		if postID, exists := s.Attr("data-post"); exists {
			post.ID = postID
		}

		text := strings.TrimSpace(s.Find(".js-message_text").Text())
		if text != "" {
			post.Text = text
		}

		baseURL := "https://eitaa.com"
		s.Find("img").Each(func(i int, img *goquery.Selection) {
			if src, exists := img.Attr("src"); exists && strings.Contains(src, "/download_") {
				post.Image = baseURL + src
			}
		})

		s.Find("video").Each(func(i int, vid *goquery.Selection) {
			if src, exists := vid.Attr("src"); exists {
				post.Video = baseURL + src
			}
		})

		time := s.Find("time").First().Text()
		if time != "" {
			post.Time = time
		}

		if post.ID != "" {
			posts = append(posts, post)
		}
	})

	return posts
}
