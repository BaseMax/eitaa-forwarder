#!/bin/sh

echo "$CRON_SCHEDULE /app/eitaa-scraper \
  --username=${EITAA_USERNAME} \
  --output=${OUTPUT} \
  --telegram-token=${TELEGRAM_TOKEN} \
  --telegram-chat-id=${TELEGRAM_CHAT_ID} \
  --sent-ids-file=${SENT_IDS_FILE} >> /var/log/cron.log 2>&1" > /etc/crontabs/root

mkdir -p /app/output /app/sent_ids /var/log

echo "[INFO] Starting cron with schedule: $CRON_SCHEDULE"
crond -f
