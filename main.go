package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
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

type Config struct {
	Username       string
	OutputFile     string
	TelegramToken  string
	TelegramChatID string
	SentIDsFile    string
	AddFooter      bool
}

type App struct {
	client    *http.Client
	config    Config
	sentIDs   map[string]bool
	bot       *tgbotapi.BotAPI
	isChannel bool
	chatID    int64
	logger    *log.Logger
}

func NewApp(config Config) (*App, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	client := &http.Client{Jar: jar}
	sentIDs, err := loadSentPostIDs(config.SentIDsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load sent post IDs: %w", err)
	}

	client.Transport = &http.Transport{
		ForceAttemptHTTP2: false,
	}

	return &App{
		client:  client,
		config:  config,
		sentIDs: sentIDs,
		logger:  log.New(os.Stdout, "INFO: ", log.LstdFlags),
	}, nil
}

func (app *App) InitTelegram() error {
	isChannel := strings.HasPrefix(app.config.TelegramChatID, "@")
	var chatID int64
	if !isChannel {
		var err error
		chatID, err = strconv.ParseInt(app.config.TelegramChatID, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid telegram chat ID: %w", err)
		}
	}

	bot, err := tgbotapi.NewBotAPI(app.config.TelegramToken)
	if err != nil {
		return fmt.Errorf("failed to initialize telegram bot: %w", err)
	}

	app.bot = bot
	app.isChannel = isChannel
	app.chatID = chatID
	return nil
}

func (app *App) Run() error {
	posts, err := app.fetchAndExtractPosts()
	if err != nil {
		return fmt.Errorf("failed to fetch and extract posts: %w", err)
	}

	if err := app.savePostsAndMedia(posts); err != nil {
		return fmt.Errorf("failed to save posts and media: %w", err)
	}

	if err := app.InitTelegram(); err != nil {
		return fmt.Errorf("failed to initialize telegram: %w", err)
	}

	return app.processAndSendPosts(posts)
}

func (app *App) fetchAndExtractPosts() ([]Post, error) {
	url := fmt.Sprintf("https://eitaa.com/%s", app.config.Username)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	app.setRequestHeaders(req, url)
	resp, err := app.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch channel page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-OK HTTP status: %s", resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if err := os.WriteFile("channel_page.html", bodyBytes, 0644); err != nil {
		app.logger.Printf("Warning: failed to save raw HTML: %v", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	posts, err := extractPosts(doc, app.config.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to extract posts: %w", err)
	}

	return posts, nil
}

func (app *App) setRequestHeaders(req *http.Request, referer string) {
	headers := map[string]string{
		"Referer":                   referer,
		"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0",
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language":           "en-US,en;q=0.5",
		"Accept-Encoding":           "gzip, deflate",
		"Upgrade-Insecure-Requests": "1",
		"Connection":                "close",
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "none",
		"Sec-Fetch-User":            "?1",
		"Priority":                  "u=0, i",
		"Pragma":                    "no-cache",
		"Cache-Control":             "no-cache",
		"TE":                        "trailers",
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
}

func (app *App) savePostsAndMedia(posts []Post) error {
	if err := app.savePostsToJSON(posts); err != nil {
		return fmt.Errorf("failed to save posts to JSON: %w", err)
	}

	refererURL := fmt.Sprintf("https://eitaa.com/%s", app.config.Username)

	for _, post := range posts {
		if len(post.Images) == 0 {
			continue
		}

		dir := filepath.Join("media", post.ID)
		if err := os.MkdirAll(dir, 0755); err != nil {
			app.logger.Printf("Failed to create directory %s: %v", dir, err)
			continue
		}

		for i, imgURL := range post.Images {
			filename := filepath.Join(dir, fmt.Sprintf("img%d.jpg", i+1))
			if err := app.downloadImage(imgURL, filename, refererURL); err != nil {
				app.logger.Printf("Failed to download image %s: %v", imgURL, err)
				continue
			}
		}
	}
	return nil
}

func (app *App) savePostsToJSON(posts []Post) error {
	data, err := json.MarshalIndent(posts, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal posts: %w", err)
	}
	return os.WriteFile(app.config.OutputFile, data, 0644)
}

func (app *App) processAndSendPosts(posts []Post) error {
	for _, post := range posts {
		if app.sentIDs[post.ID] {
			app.logger.Printf("Skipping post %s: already sent", post.ID)
			continue
		}

		if err := app.processPost(post); err != nil {
			app.logger.Printf("Failed to process post %s: %v", post.ID, err)
			continue
		}

		if err := saveSentPostID(app.config.SentIDsFile, post.ID); err != nil {
			app.logger.Printf("Failed to save post ID %s: %v", post.ID, err)
			continue
		}
		app.logger.Printf("Successfully sent post %s", post.ID)
	}
	return nil
}

func (app *App) processPost(post Post) error {
	messageText := buildMessageText(post, app.config.Username, app.config.AddFooter)
	dir := filepath.Join("media", post.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	if len(post.Images) == 0 && messageText != "" {
		return app.sendTextMessage(messageText)
	} else if len(post.Images) > 0 {
		return app.sendMediaGroup(post, messageText, dir)
	}
	app.logger.Printf("Skipping post %s: no content to send", post.ID)
	return nil
}

func (app *App) sendTextMessage(messageText string) error {
	var msg tgbotapi.MessageConfig
	if app.isChannel {
		msg = tgbotapi.NewMessageToChannel(app.config.TelegramChatID, escapeMarkdownV2(messageText))
	} else {
		msg = tgbotapi.NewMessage(app.chatID, escapeMarkdownV2(messageText))
	}
	msg.ParseMode = "MarkdownV2"
	_, err := app.bot.Send(msg)
	return err
}

func (app *App) sendMediaGroup(post Post, messageText, dir string) error {
	mediaGroup := make([]interface{}, 0, len(post.Images))
	for i := range post.Images {
		filename := filepath.Join(dir, fmt.Sprintf("img%d.jpg", i+1))
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			app.logger.Printf("Image not found %s for post %s", filename, post.ID)
			continue
		}

		photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(filename))
		if i == 0 && messageText != "" {
			photo.Caption = escapeMarkdownV2(messageText)
			photo.ParseMode = "MarkdownV2"
		}
		mediaGroup = append(mediaGroup, photo)
	}

	if len(mediaGroup) == 0 {
		return fmt.Errorf("no valid images to send for post %s", post.ID)
	}

	var cfg tgbotapi.MediaGroupConfig
	if app.isChannel {
		cfg = tgbotapi.MediaGroupConfig{ChannelUsername: app.config.TelegramChatID, Media: mediaGroup}
	} else {
		cfg = tgbotapi.MediaGroupConfig{ChatID: app.chatID, Media: mediaGroup}
	}

	_, err := app.bot.SendMediaGroup(cfg)
	return err
}

func (app *App) downloadImage(url, filename string, refererURL string) error {
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		return nil
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request for %s: %w", url, err)
	}
	req.Header.Del("Range")

	app.setRequestHeaders(req, refererURL)

	resp, err := app.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", url, err)
	}
	defer resp.Body.Close()

	app.logger.Printf("Response status for %s: %s", url, resp.Status)
	app.logger.Printf("Response headers for %s: %v", url, resp.Header)

	if resp.StatusCode == http.StatusPartialContent {
		app.logger.Printf("Received 206 Partial Content for %s", url)
		contentRange := resp.Header.Get("Content-Range")
		if contentRange != "" {
			app.logger.Printf("Content-Range: %s", contentRange)
		}
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status downloading %s: %s", url, resp.Status)
	}

	out, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filename, err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func escapeMarkdownV2(text string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	)
	return replacer.Replace(text)
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
		} else {
			return
		}

		if text := strings.TrimSpace(s.Find(".js-message_text").Text()); text != "" {
			post.Text = text
		}

		s.Find("a.etme_widget_message_photo_wrap").Each(func(i int, a *goquery.Selection) {
			if style, exists := a.Attr("style"); exists && strings.Contains(style, "background-image") {
				if url := extractBackgroundURL(style); url != "" && strings.Contains(url, "/download_") {
					fullURL := baseURL + url
					if !contains(post.Images, fullURL) {
						post.Images = append(post.Images, fullURL)
					}
				}
			}
		})

		if timeElem := s.Find("time").First(); timeElem.Length() > 0 {
			post.Time = timeElem.Text()
			if datetime, exists := timeElem.Attr("datetime"); exists {
				dateParts := strings.Split(datetime, "T")
				if len(dateParts) > 0 {
					post.Date = strings.Replace(dateParts[0], "-", "/", -1)
				}
			}
		}

		if forwardedDiv := s.Find(".etme_widget_message_forwarded_from"); forwardedDiv.Length() > 0 {
			post.IsForwarded = true
			if forwardedAnchor := forwardedDiv.Find("a.etme_widget_message_forwarded_from_name"); forwardedAnchor.Length() > 0 {
				post.ForwardedFrom = strings.TrimSpace(forwardedAnchor.Text())
				if href, exists := forwardedAnchor.Attr("href"); exists {
					post.ForwardedFromLink = href
				}
			}
		}

		if replyAnchor := s.Find("a.etme_widget_message_reply"); replyAnchor.Length() > 0 {
			post.IsReply = true
			if href, exists := replyAnchor.Attr("href"); exists && strings.HasPrefix(href, "/"+username+"/") {
				parts := strings.Split(href, "/")
				if len(parts) == 3 {
					post.ReplyToMessageID = parts[2]
				}
			}
		}

		posts = append(posts, post)
	})

	if len(posts) == 0 {
		return nil, fmt.Errorf("no posts found in channel page")
	}
	return posts, nil
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
	url := strings.Trim(style[start:start+end], "'\"")
	return url
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func loadSentPostIDs(filename string) (map[string]bool, error) {
	sentIDs := make(map[string]bool)
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return sentIDs, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read sent IDs file: %w", err)
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, fmt.Errorf("failed to unmarshal sent IDs: %w", err)
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
	ids := make([]string, 0, len(sentIDs))
	for id := range sentIDs {
		ids = append(ids, id)
	}
	data, err := json.MarshalIndent(ids, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal sent IDs: %w", err)
	}
	return os.WriteFile(filename, data, 0644)
}

func buildMessageText(post Post, username string, addFooter bool) string {
	var sb strings.Builder
	if post.Text != "" {
		sb.WriteString(post.Text)
	}
	if !addFooter {
		return sb.String()
	}
	if post.IsForwarded {
		sb.WriteString(fmt.Sprintf("\n\nForwarded from: [%s](%s)", post.ForwardedFrom, post.ForwardedFromLink))
	}
	if post.IsReply {
		sb.WriteString(fmt.Sprintf("\n\nIn reply to: https://eitaa.com/%s/%s", username, post.ReplyToMessageID))
	}
	if post.Time != "" && post.Date != "" {
		sb.WriteString(fmt.Sprintf("\n\nPosted on: %s %s", post.Date, post.Time))
	}
	return sb.String()
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

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found or failed to load: %v", err)
	}

	usernameFlag := flag.String("username", "", "Eitaa channel username")
	outputFlag := flag.String("output", "", "Output JSON file")
	telegramTokenFlag := flag.String("telegram-token", "", "Telegram bot token")
	telegramChatIDFlag := flag.String("telegram-chat-id", "", "Telegram chat ID")
	sentIDsFileFlag := flag.String("sent-ids-file", "", "File to store sent post IDs")
	addFooterFlag := flag.Bool("add-footer", false, "Add footer to messages (true/false)")

	flag.Parse()

	config := Config{
		AddFooter:      *addFooterFlag || strings.ToLower(os.Getenv("ADD_FOOTER")) == "true",
		Username:       getEnvOrFlag(*usernameFlag, "EITAA_USERNAME"),
		OutputFile:     getEnvOrFlag(*outputFlag, "OUTPUT", "posts.json"),
		TelegramToken:  getEnvOrFlag(*telegramTokenFlag, "TELEGRAM_TOKEN"),
		TelegramChatID: getEnvOrFlag(*telegramChatIDFlag, "TELEGRAM_CHAT_ID"),
		SentIDsFile:    getEnvOrFlag(*sentIDsFileFlag, "SENT_IDS_FILE", "sent_ids.json"),
	}

	if config.Username == "" {
		log.Fatal("Username not provided. Set --username flag or EITAA_USERNAME in .env")
	}

	app, err := NewApp(config)
	if err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}

	if err := app.Run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
