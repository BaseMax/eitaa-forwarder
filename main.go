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
	ID                string   `json:"id"`
	Text              string   `json:"text,omitempty"`
	Images            []string `json:"images,omitempty"`
	Videos            []string `json:"videos,omitempty"`
	Time              string   `json:"time,omitempty"`
	Date              string   `json:"date,omitempty"`
	IsForwarded       bool     `json:"is_forwarded,omitempty"`
	ForwardedFrom     string   `json:"forwarded_from,omitempty"`
	ForwardedFromLink string   `json:"forwarded_from_link,omitempty"`
	IsReply           bool     `json:"is_reply,omitempty"`
	ReplyToMessageID  string   `json:"reply_to_message_id,omitempty"`
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func extractBackgroundURL(style string) string {
	start := strings.Index(style, "url(")
	if start == -1 {
		return ""
	}
	start += 4
	end := strings.Index(style[start:], ")")
	if end == -1 {
		return ""
	}
	url := style[start : start+end]
	url = strings.Trim(url, "'\"")
	return url
}

func extractPosts(doc *goquery.Document, username string) ([]Post, error) {
	var posts []Post
	baseURL := "https://eitaa.com"

	doc.Find(".js-widget_message_wrap").Each(func(i int, wrap *goquery.Selection) {
		s := wrap.Find(".js-widget_message")
		if s.Length() == 0 {
			return
		}

		var post Post

		if postID, exists := s.Attr("data-post"); exists {
			post.ID = postID
		}

		text := strings.TrimSpace(s.Find(".js-message_text").Text())
		if text != "" {
			post.Text = text
		}

		s.Find("img").Each(func(i int, img *goquery.Selection) {
			if src, exists := img.Attr("src"); exists && strings.Contains(src, "/download_") {
				fullURL := baseURL + src
				if !contains(post.Images, fullURL) {
					post.Images = append(post.Images, fullURL)
				}
			}
		})

		s.Find(".js-message_grouped_layer a").Each(func(i int, a *goquery.Selection) {
			style, exists := a.Attr("style")
			if exists && strings.Contains(style, "background-image") {
				url := extractBackgroundURL(style)
				if url != "" && strings.Contains(url, "/download_") {
					fullURL := baseURL + url
					if !contains(post.Images, fullURL) {
						post.Images = append(post.Images, fullURL)
					}
				}
			}
		})

		s.Find("video").Each(func(i int, vid *goquery.Selection) {
			if src, exists := vid.Attr("src"); exists {
				fullURL := baseURL + src
				if !contains(post.Videos, fullURL) {
					post.Videos = append(post.Videos, fullURL)
				}
			}
		})

		timeElem := s.Find("time").First()
		if timeElem.Length() > 0 {
			post.Time = timeElem.Text()
			if datetime, exists := timeElem.Attr("datetime"); exists {
				dateParts := strings.Split(datetime, "T")
				if len(dateParts) > 0 {
					post.Date = strings.Replace(dateParts[0], "-", "/", -1)
				}
			}
		}

		forwardedDiv := s.Find(".etme_widget_message_forwarded_from")
		if forwardedDiv.Length() > 0 {
			post.IsForwarded = true
			forwardedAnchor := forwardedDiv.Find("a.etme_widget_message_forwarded_from_name")
			if forwardedAnchor.Length() > 0 {
				post.ForwardedFrom = strings.TrimSpace(forwardedAnchor.Text())
				if href, exists := forwardedAnchor.Attr("href"); exists {
					post.ForwardedFromLink = href
				}
			}
		}

		replyAnchor := s.Find("a.etme_widget_message_reply")
		if replyAnchor.Length() > 0 {
			post.IsReply = true
			if href, exists := replyAnchor.Attr("href"); exists {
				if strings.HasPrefix(href, "/"+username+"/") {
					parts := strings.Split(href, "/")
					if len(parts) == 3 {
						post.ReplyToMessageID = parts[2]
					}
				}
			}
		}

		if post.ID != "" {
			posts = extractPosts(posts, post)
		}
	})

	if len(posts) == 0 {
		return nil, errors.New("no posts found in channel page")
	}

	return posts, nil
}

func main() {
	username := flag.String("username", "", "Eitaa channel username (e.g., m_ahlebeit)")
	output := flag.String("output", "posts.json", "Output JSON file")
	flag.Parse()

	if *username == "" {
		log.Fatal("Please provide a username with -username")
	}

	url := fmt.Sprintf("https://eitaa.com/%s", *username)
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Failed to fetch channel page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Received non-OK HTTP status: %s", resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatalf("Failed to parse HTML: %v", err)
	}

	posts, err := extractPosts(doc, *username)
	if err != nil {
		log.Fatalf("Failed to extract posts: %v", err)
	}

	jsonData, err := json.MarshalIndent(posts, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}

	err = os.WriteFile(*output, jsonData, 0644)
	if err != nil {
		log.Fatalf("Failed to write JSON file: %v", err)
	}

	fmt.Printf("Successfully scraped %d posts to %s\n", len(posts), *output)
}
