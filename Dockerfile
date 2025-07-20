FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o eitaa-scraper main.go

FROM alpine:latest

RUN apk add --no-cache bash busybox-suid

WORKDIR /app

COPY --from=builder /app/eitaa-scraper /app/eitaa-scraper

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh /app/eitaa-scraper

ENV CRON_SCHEDULE="*/5 * * * *"
ENV OUTPUT=/app/output/posts.json
ENV SENT_IDS_FILE=/app/sent_ids/sent_ids.json

ENTRYPOINT ["/entrypoint.sh"]
