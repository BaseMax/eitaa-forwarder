# Eitaa Extractor

Eitaa Extractor is a Go-based web scraper designed to extract posts from Eitaa channels. It retrieves post details such as text, images, videos, timestamps, and metadata (e.g., forwarded or reply information) and saves them to a JSON file. The project includes Docker and Docker Compose configurations for easy deployment and scheduled scraping via a cron job.

## Features

- Scrapes posts from a specified Eitaa channel.
- Extracts post details including text, images, videos, timestamps, forwarded posts, and replies.
- Saves scraped data to a JSON file.
- Supports configuration via CLI arguments or a .env file.
- Includes Docker Compose setup for running the scraper and scheduling tasks with cron.

## Dependencies

- `Goquery`: For HTML parsing and scraping.
- `go-telegram-bot-api`: For interacting with the Telegram Bot API.
- Go standard library for HTTP requests, JSON handling, and file operations.

## Prerequisites

- Go (version 1.20 or later) for building the application locally.
- Docker and Docker Compose for containerized deployment.
- An Eitaa channel username to scrape posts from.

## Installation

### Clone the Repository:

```
git clone https://github.com/BaseMax/eitaa-extractor.git
cd eitaa-extractor
```

### Create a `.env` File:

Create a `.env` file in the project root with the following content:

```
USERNAME=your_channel_username
OUTPUT=/app/output/posts.json
```

Replace `your_channel_username` with the Eitaa channel username (e.g., `m_ahlebeit`).

Set Up the Output Directory:Create an output directory to store the scraped JSON files:

```
mkdir output
```

## Usage

### Running Locally

Build and run the Go program:

```
go build -o eitaa-scraper main.go
./eitaa-scraper --username=m_ahlebeit --output=posts.json
```

Alternatively, use environment variables defined in the .env file:

```
./eitaa-scraper
```

The scraped posts will be saved to the specified output file (e.g., posts.json).

### Running with Docker

Build and run the Docker Compose setup:

```
docker-compose up --build
```

This starts the scraper service to scrape posts once and the cron service to schedule scraping every hour.

The scraped **JSON** files will be saved to the output directory on the host, mapped to `/app/output` in the container.


### Overriding .env Values

To override the `.env` file values, pass CLI arguments when running the container:
```
docker run --rm eitaa-scraper --username=other_username --output=/app/output/other_posts.json
```

## Configuration

### CLI Arguments:

- `--username`: The Eitaa channel username (e.g., m_ahlebeit). Overrides the USERNAME environment variable.
- `--output`: The output JSON file path (e.g., posts.json). Overrides the OUTPUT environment variable.

### Environment Variables (in .env):

- `USERNAME`: The default Eitaa channel username.
OUTPUT: The default output JSON file path (e.g., /app/output/posts.json).

If CLI arguments are not provided, the program uses the values from the .env file. If neither is provided, the output defaults to posts.json, and the username must be specified to avoid an error.

## Project Structure

```
eitaa-extractor/
├── .env                # Environment variables (USERNAME, OUTPUT)
├── docker-compose.yml  # Docker Compose configuration
├── Dockerfile          # Docker build instructions
├── go.mod              # Go module dependencies
├── go.sum              # Go module checksums
├── main.go             # Main Go program
└── output/             # Directory for output JSON files
```

## Dependencies

Goquery: For HTML parsing and scraping.
Go standard library for HTTP requests, JSON handling, and file operations.

## Contributing

Contributions are welcome! Please submit a pull request or open an issue on the GitHub repository.

## Contact

For questions or feedback, please open an issue on the GitHub repository.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

Copyright

© 2025 Max Base
