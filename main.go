package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

type Post struct {
	ID                string   `json:"id"`
	Text              string   `json:"text,omitempty"`
	Images            []string `json:"images,omitempty"`
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

func loadSentPostIDs(filename string) (map[string]bool, error) {
	sentIDs := make(map[string]bool)
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return sentIDs, nil
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

func saveSentPostID(filename, postID string) error {
	sentIDs, err := loadSentPostIDs(filename)
	if err != nil {
		return err
	}
	if sentIDs[postID] {
		return nil
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
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found or failed to load it")
	}

	usernameFlag := flag.String("username", "", "Eitaa channel username (e.g., m_ahlebeit)")
	outputFlag := flag.String("output", "", "Output JSON file")
	telegramTokenFlag := flag.String("telegram-token", "", "Telegram bot token")
	telegramChatIDFlag := flag.String("telegram-chat-id", "", "Telegram chat ID")
	sentIDsFileFlag := flag.String("sent-ids-file", "", "File to store sent post IDs")
	flag.Parse()

	username := getEnvOrFlag(*usernameFlag, "USERNAME")
	output := getEnvOrFlag(*outputFlag, "OUTPUT", "posts.json")
	telegramToken := getEnvOrFlag(*telegramTokenFlag, "TELEGRAM_TOKEN")
	telegramChatID := getEnvOrFlag(*telegramChatIDFlag, "TELEGRAM_CHAT_ID")
	sentIDsFile := getEnvOrFlag(*sentIDsFileFlag, "SENT_IDS_FILE", "sent_ids.json")

	var telegramChatIDInt int64
	isChannel := strings.HasPrefix(telegramChatID, "@")
	if !isChannel {
		telegramChatIDInt, err = strconv.ParseInt(telegramChatID, 10, 64)
		if err != nil {
			log.Fatalf("Invalid Telegram chat ID: %s", telegramChatID)
		}
	}

	sentIDs, err := loadSentPostIDs(sentIDsFile)
	if err != nil {
		log.Fatalf("Failed to load sent post IDs: %v", err)
	}

	resp, err := http.Get(fmt.Sprintf("https://eitaa.com/%s", username))
	if err != nil {
		log.Fatalf("Failed to fetch channel page: %v", err)
	}
	defer resp.Body.Close()

	rawHTMLFile := "channel_page.html"
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response body: %v", err)
	}

	if err := os.WriteFile(rawHTMLFile, bodyBytes, 0644); err != nil {
		log.Fatalf("Failed to save raw HTML to file: %v", err)
	}

	resp.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Non-OK HTTP status: %s", resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatalf("Failed to parse HTML: %v", err)
	}

	posts, err := extractPosts(doc, username)
	if err != nil {
		log.Fatalf("Failed to extract posts: %v", err)
	}

	if err := os.WriteFile(output, prettyJSON(posts), 0644); err != nil {
		log.Fatalf("Failed to write JSON: %v", err)
	}

	bot, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		log.Fatalf("Telegram bot init failed: %v", err)
	}

	for _, post := range posts {
		if sentIDs[post.ID] {
			fmt.Printf("Skipping post %s: already sent\n", post.ID)
			continue
		}

		messageText := buildMessageText(post, username)

		if len(post.Images) == 0 && messageText != "" {
			var msg tgbotapi.MessageConfig
			if isChannel {
				msg = tgbotapi.NewMessageToChannel(telegramChatID, messageText)
			} else {
				msg = tgbotapi.NewMessage(telegramChatIDInt, messageText)
			}
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Failed to send text: %v", err)
				continue
			}
		}

		if len(post.Images) > 0 {
			var mediaGroup []interface{}
			for i, imgURL := range post.Images {
				photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FileURL(imgURL))
				if i == 0 && messageText != "" {
					photo.Caption = messageText
				}
				mediaGroup = append(mediaGroup, photo)
			}

			var cfg tgbotapi.MediaGroupConfig
			if isChannel {
				cfg = tgbotapi.MediaGroupConfig{ChannelUsername: telegramChatID, Media: mediaGroup}
			} else {
				cfg = tgbotapi.MediaGroupConfig{ChatID: telegramChatIDInt, Media: mediaGroup}
			}

			if _, err := bot.SendMediaGroup(cfg); err != nil {
				log.Printf("Failed to send media group for post %s: %v", post.ID, err)
				continue
			}
		}

		if err := saveSentPostID(sentIDsFile, post.ID); err != nil {
			log.Printf("Failed to save post ID %s: %v", post.ID, err)
		} else {
			fmt.Printf("✅ Sent post %s\n", post.ID)
		}
	}
}

func getEnvOrFlag(flagVal, envVar string, defaultVal ...string) string {
	if flagVal != "" {
		return flagVal
	}
	if val := os.Getenv(envVar); val != "" {
		return val
	}
	if len(defaultVal) > 0 {
		return defaultVal[0]
	}
	log.Fatalf("Missing required value for %s", envVar)
	return ""
}

func buildMessageText(post Post, username string) string {
	var sb strings.Builder
	if post.Text != "" {
		sb.WriteString(post.Text)
	}
	if post.IsForwarded {
		sb.WriteString(fmt.Sprintf("\n\nForwarded from: %s (%s)", post.ForwardedFrom, post.ForwardedFromLink))
	}
	if post.IsReply {
		sb.WriteString(fmt.Sprintf("\n\nIn reply to: https://eitaa.com/%s/%s", username, post.ReplyToMessageID))
	}
	if post.Time != "" && post.Date != "" {
		sb.WriteString(fmt.Sprintf("\n\nPosted on: %s %s", post.Date, post.Time))
	}
	return sb.String()
}

func prettyJSON(data interface{}) []byte {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Printf("Failed to format JSON: %v", err)
		return []byte("[]")
	}
	return b
}
