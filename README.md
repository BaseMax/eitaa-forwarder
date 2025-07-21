# Eitaa Forwarder

Eitaa Forwarder is a Telegram bot and web scraper built in Go that captures posts from Eitaa channels and automatically forwards them to a designated Telegram chat or channel. It collects various post elements including text, images, timestamps, and metadata like forwarding or reply details—stores this data in a JSON file, and ensures new posts are sent to Telegram without manual intervention.

## Features

* Scrapes posts from a specified Eitaa channel.
* Extracts post details including text, images, timestamps, forwarded posts, and replies.
* Forwards new posts to a Telegram chat or channel using a Telegram Bot.
* Keeps track of sent posts to avoid duplicates.
* Saves scraped data to a JSON file.
* Supports configuration via CLI arguments or a `.env` file.
* Includes Docker Compose setup for easy deployment and scheduled scraping via cron.

## Dependencies

* [`goquery`](https://github.com/PuerkitoBio/goquery) for HTML parsing and scraping.
* [`go-telegram-bot-api`](https://github.com/go-telegram-bot-api/telegram-bot-api) for interacting with the Telegram Bot API.
* Go standard library for HTTP requests, JSON handling, and file operations.

## Prerequisites

* Go (version 1.20 or later) for building the application locally.
* Docker and Docker Compose for containerized deployment.
* An Eitaa channel username to scrape posts from.
* A Telegram Bot token and target Telegram chat ID or channel username for forwarding.

## Installation

### Clone the Repository

```bash
git clone https://github.com/BaseMax/eitaa-extractor.git
cd eitaa-extractor
```

### Create a `.env` File

Create a `.env` file in the project root with the following content:

```env
EITAA_USERNAME=your_channel_username
TELEGRAM_TOKEN=your_telegram_bot_token
TELEGRAM_CHAT_ID=your_telegram_chat_id_or_channel_username
OUTPUT=/app/output/posts.json
SENT_IDS_FILE=/app/output/sent_ids.json
```

Replace the values accordingly:

* `EITAA_USERNAME`: The Eitaa channel username (e.g., `m_ahlebeit`).
* `TELEGRAM_TOKEN`: Your Telegram Bot token.
* `TELEGRAM_CHAT_ID`: Target Telegram chat ID (integer) or channel username (starting with `@`).
* `OUTPUT`: Path to save the scraped posts JSON file.
* `SENT_IDS_FILE`: Path to store IDs of posts already forwarded to Telegram.

### Set Up the Output Directory

Create an output directory to store the JSON files:

```bash
mkdir output
mkdir sent_ids
```

## Usage

### Running Locally

Build and run the Go program:

```bash
go build -o eitaa-forwarder main.go
./eitaa-forwarder --username=m_ahlebeit --telegram-token=your_bot_token --telegram-chat-id=@your_channel --output=posts.json --sent-ids-file=sent_ids.json
```

Alternatively, rely on environment variables defined in `.env`:

```bash
./eitaa-forwarder
```

The program will scrape posts from the specified Eitaa channel, forward new posts to Telegram, and save results to JSON files.

### Running with Docker

Build and run using Docker Compose:

```bash
docker-compose up --build
```

This starts the forwarder service to scrape posts and forward them once, and a cron service to schedule periodic scraping and forwarding.

The JSON files will be saved to the mapped output directory on the host.

### Overriding `.env` Values

Override environment variables with CLI arguments when running the container:

```bash
docker run --rm eitaa-forwarder --username=other_username --telegram-token=other_token --telegram-chat-id=@other_channel --output=/app/output/other_posts.json
```

## Configuration

### CLI Arguments

* `--username`: Eitaa channel username (e.g., `m_ahlebeit`). Overrides `EITAA_USERNAME` env variable.
* `--telegram-token`: Telegram bot token. Overrides `TELEGRAM_TOKEN`.
* `--telegram-chat-id`: Telegram chat ID or channel username. Overrides `TELEGRAM_CHAT_ID`.
* `--output`: Output JSON file path. Overrides `OUTPUT`.
* `--sent-ids-file`: File path to store sent post IDs. Overrides `SENT_IDS_FILE`.

### Environment Variables (in `.env`)

* `EITAA_USERNAME`: Default Eitaa channel username.
* `TELEGRAM_TOKEN`: Telegram bot token.
* `TELEGRAM_CHAT_ID`: Telegram chat ID or channel username.
* `OUTPUT`: Default output JSON file path.
* `SENT_IDS_FILE`: File path for sent post IDs.

## Project Structure

```
eitaa-extractor/
├── .env                # Environment variables (EITAA_USERNAME, TELEGRAM_TOKEN, etc.)
├── docker-compose.yml  # Docker Compose configuration
├── Dockerfile          # Docker build instructions
├── go.mod              # Go module dependencies
├── go.sum              # Go module checksums
├── main.go             # Main Go program
└── output/             # Directory for output JSON files
```

## Contributing

Contributions are welcome! Please submit a pull request or open an issue on the GitHub repository.

## Contact

For questions or feedback, please open an issue on the GitHub repository.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

---

© 2025 Max Base
