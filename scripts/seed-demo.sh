#!/usr/bin/env bash
# Creates a demo project and floods it with a mix of pageviews, custom
# events, errors and bot traffic, so the dashboard has something to show.
# Requires a running instance (see `docker compose up`).
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
ADMIN_TOKEN="${ADMIN_TOKEN:?set ADMIN_TOKEN to the same value docker-compose is using (see .env)}"
EVENTS="${EVENTS:-300}"

gen_uuid() {
  printf '%08x-%04x-4%03x-%04x-%08x%04x\n' \
    "$RANDOM$RANDOM" "$RANDOM" "$((RANDOM % 4096))" "$((RANDOM % 16384 + 32768))" "$RANDOM$RANDOM" "$RANDOM"
}

CREATE_RESPONSE=$(curl -sf -X POST "$BASE_URL/api/v1/projects" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Demo Site","domain":"demo.example.com"}')

PROJECT_ID=$(echo "$CREATE_RESPONSE" | grep -o '"project_id":"[^"]*"' | cut -d'"' -f4)
API_KEY=$(echo "$CREATE_RESPONSE" | grep -o '"api_key":"[^"]*"' | cut -d'"' -f4)
[ -n "$PROJECT_ID" ] || { echo "failed to create demo project: $CREATE_RESPONSE" >&2; exit 1; }

echo "Created demo project $PROJECT_ID"
echo "API key (use this to log into the dashboard): $API_KEY"
echo

PATHS=("/" "/pricing" "/calculator" "/blog/post-1" "/about")
REFERRERS=("" "https://google.com" "https://twitter.com" "https://news.ycombinator.com")
UAS=(
  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"
  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1"
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"
  "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Mobile Safari/537.36"
)
BOT_UA="Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"

track() {
  curl -s -o /dev/null -X POST "$BASE_URL/api/v1/track" -H "Content-Type: application/json" -H "User-Agent: $2" -d "$1"
}

for i in $(seq 1 "$EVENTS"); do
  path=${PATHS[$RANDOM % ${#PATHS[@]}]}
  referrer=${REFERRERS[$RANDOM % ${#REFERRERS[@]}]}
  ua=${UAS[$RANDOM % ${#UAS[@]}]}
  visitor=$(gen_uuid)

  track "{\"project_id\":\"$PROJECT_ID\",\"visitor_id\":\"$visitor\",\"path\":\"$path\",\"referrer\":\"$referrer\"}" "$ua"

  if [ $((RANDOM % 5)) -eq 0 ]; then
    track "{\"project_id\":\"$PROJECT_ID\",\"visitor_id\":\"$visitor\",\"type\":\"custom\",\"name\":\"calculator_used\",\"metadata\":{\"amount\":$((RANDOM % 1000))}}" "$ua"
  fi

  if [ $((RANDOM % 20)) -eq 0 ]; then
    track "{\"project_id\":\"$PROJECT_ID\",\"visitor_id\":\"$visitor\",\"type\":\"error\",\"name\":\"unhandled_exception\"}" "$ua"
  fi

  if [ $((RANDOM % 15)) -eq 0 ]; then
    track "{\"project_id\":\"$PROJECT_ID\",\"visitor_id\":\"$(gen_uuid)\",\"path\":\"/\"}" "$BOT_UA"
  fi
done

echo "Seeded $EVENTS pageviews (plus a mix of custom/error/bot events) for project $PROJECT_ID"
echo "Visit $BASE_URL/dashboard and log in with the API key above"
