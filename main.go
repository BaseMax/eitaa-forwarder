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
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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
			posts = append(posts, post)
		}
	})

	if len(posts) == 0 {
		return nil, errors.New("no posts found in channel page")
	}

	return posts, nil
}

// loadSentPostIDs reads the list of sent post IDs from a file
func loadSentPostIDs(filename string) (map[string]bool, error) {
	sentIDs := make(map[string]bool)
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return sentIDs, nil // File doesn't exist yet, return empty map
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read sent IDs file: %v", err)
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, fmt.Errorf("failed to unmarshal sent IDs: %v", err)
	}
	for _, id := range ids {
		sentIDs[id] = true
	}
	return sentIDs, nil
}

// saveSentPostID appends a post ID to the sent IDs file
func saveSentPostID(filename, postID string) error {
	sentIDs, err := loadSentPostIDs(filename)
	if err != nil {
		return err
	}
	if sentIDs[postID] {
		return nil // ID already exists, no need to save
	}
	sentIDs[postID] = true
	var ids []string
	for id := range sentIDs {
		ids = append(ids, id)
	}
	data, err := json.MarshalIndent(ids, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal sent IDs: %v", err)
	}
	return os.WriteFile(filename, data, 0644)
}

func main() {
	// Define CLI flags
	usernameFlag := flag.String("username", "", "Eitaa channel username (e.g., m_ahlebeit)")
	outputFlag := flag.String("output", "", "Output JSON file")
	telegramTokenFlag := flag.String("telegram-token", "", "Telegram bot token")
	telegramChatIDFlag := flag.String("telegram-chat-id", "", "Telegram chat ID")
	sentIDsFileFlag := flag.String("sent-ids-file", "", "File to store sent post IDs")
	flag.Parse()

	// Check CLI flags first, then fall back to environment variables
	username := *usernameFlag
	if username == "" {
		username = os.Getenv("USERNAME")
	}
	if username == "" {
		log.Fatal("Please provide a username via -username flag or USERNAME environment variable")
	}

	output := *outputFlag
	if output == "" {
		output = os.Getenv("OUTPUT")
	}
	if output == "" {
		output = "posts.json"
	}

	telegramToken := *telegramTokenFlag
	if telegramToken == "" {
		telegramToken = os.Getenv("TELEGRAM_TOKEN")
	}
	if telegramToken == "" {
		log.Fatal("Please provide a Telegram bot token via -telegram-token flag or TELEGRAM_TOKEN environment variable")
	}

	telegramChatID := *telegramChatIDFlag
	if telegramChatID == "" {
		telegramChatID = os.Getenv("TELEGRAM_CHAT_ID")
	}
	if telegramChatID == "" {
		log.Fatal("Please provide a Telegram chat ID via -telegram-chat-id flag or TELEGRAM_CHAT_ID environment variable")
	}

	sentIDsFile := *sentIDsFileFlag
	if sentIDsFile == "" {
		sentIDsFile = os.Getenv("SENT_IDS_FILE")
	}
	if sentIDsFile == "" {
		sentIDsFile = "sent_ids.json"
	}

	sentIDs, err := loadSentPostIDs(sentIDsFile)
	if err != nil {
		log.Fatalf("Failed to load sent post IDs: %v", err)
	}

	url := fmt.Sprintf("https://eitaa.com/%s", username)
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

	posts, err := extractPosts(doc, username)
	if err != nil {
		log.Fatalf("Failed to extract posts: %v", err)
	}

	jsonData, err := json.MarshalIndent(posts, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}

	err = os.WriteFile(output, jsonData, 0644)
	if err != nil {
		log.Fatalf("Failed to write JSON file: %v", err)
	}

	fmt.Printf("Successfully scraped %d posts to %s\n", len(posts), output)

	bot, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		log.Fatalf("Failed to initialize Telegram bot: %v", err)
	}

	for _, post := range posts {
		if sentIDs[post.ID] {
			fmt.Printf("Skipping post %s: already sent\n", post.ID)
			continue
		}

		messageText := post.Text
		if post.IsForwarded {
			messageText += fmt.Sprintf("\n\nForwarded from: %s (%s)", post.ForwardedFrom, post.ForwardedFromLink)
		}
		if post.IsReply {
			messageText += fmt.Sprintf("\n\nIn reply to: https://eitaa.com/%s/%s", username, post.ReplyToMessageID)
		}
		if post.Time != "" && post.Date != "" {
			messageText += fmt.Sprintf("\n\nPosted on: %s %s", post.Date, post.Time)
		}

		msg := tgbotapi.NewMessageToChannel(telegramChatID, messageText)
		sentMsg, err := bot.Send(msg)
		if err != nil {
			log.Printf("Failed to send post %s to Telegram: %v", post.ID, err)
			continue
		}

		if err := saveSentPostID(sentIDsFile, post.ID); err != nil {
			log.Printf("Failed to save sent post ID %s: %v", post.ID, err)
		} else {
			fmt.Printf("Successfully sent post %s to Telegram (Message ID: %d)\n", post.ID, sentMsg.MessageID)
		}

		for _, imgURL := range post.Images {
			photoMsg := tgbotapi.NewPhotoToChannel(telegramChatID, tgbotapi.FileURL(imgURL))
			_, err := bot.Send(photoMsg)
			if err != nil {
				log.Printf("Failed to send image %s for post %s: %v", imgURL, post.ID, err)
			}
		}

		for _, vidURL := range post.Videos {
			videoMsg := tgbotapi.NewVideoToChannel(telegramChatID, tgbotapi.FileURL(vidURL))
			_, err := bot.Send(videoMsg)
			if err != nil {
				log.Printf("Failed to send video %s for post %s: %v", vidURL, post.ID, err)
			}
		}
	}
}
