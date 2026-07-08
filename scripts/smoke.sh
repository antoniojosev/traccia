#!/usr/bin/env bash
# End-to-end check against a running Traccia instance (see `make smoke`,
# which builds/starts docker-compose and tears it down around this).
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
ADMIN_TOKEN="${ADMIN_TOKEN:?set ADMIN_TOKEN to the same value docker-compose is using (see .env)}"

fail() { echo "FAIL: $1" >&2; exit 1; }
pass() { echo "ok - $1"; }

echo "waiting for $BASE_URL/healthz..."
ready=false
for _ in $(seq 1 30); do
  if curl -sf "$BASE_URL/healthz" >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 1
done
[ "$ready" = true ] || fail "server never became healthy after 30s"
pass "healthz"

CREATE_RESPONSE=$(curl -sf -X POST "$BASE_URL/api/v1/projects" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Smoke Test","domain":"example.com"}') || fail "create project request failed"

PROJECT_ID=$(echo "$CREATE_RESPONSE" | grep -o '"project_id":"[^"]*"' | cut -d'"' -f4)
API_KEY=$(echo "$CREATE_RESPONSE" | grep -o '"api_key":"[^"]*"' | cut -d'"' -f4)
[ -n "$PROJECT_ID" ] || fail "no project_id in response: $CREATE_RESPONSE"
[ -n "$API_KEY" ] || fail "no api_key in response: $CREATE_RESPONSE"
pass "create project -> $PROJECT_ID"

curl -sf "$BASE_URL/t.js" | grep -q "traccia" || fail "/t.js did not return the tracking script"
pass "GET /t.js"

BROWSER_UA="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"

# A plain curl call (default UA) gets classified as bot traffic by the UA
# parser's heuristic (it looks for "curl"/"wget"/etc, deliberately) and
# excluded from stats by default — real traffic never looks like that, but
# this script's own requests would, so every track() call impersonates a
# real browser to actually exercise the pageview/stats path being tested.
track() {
  local status
  status=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE_URL/api/v1/track" \
    -H "Content-Type: application/json" -H "User-Agent: $BROWSER_UA" -d "$1")
  [ "$status" = "202" ] || fail "track returned $status for payload: $1"
}

track "{\"project_id\":\"$PROJECT_ID\",\"visitor_id\":\"11111111-1111-1111-1111-111111111111\",\"path\":\"/\"}"
pass "track pageview"

track "{\"project_id\":\"$PROJECT_ID\",\"visitor_id\":\"22222222-2222-2222-2222-222222222222\",\"type\":\"custom\",\"name\":\"calculator_used\",\"metadata\":{\"amount\":100}}"
pass "track custom event"

track "{\"project_id\":\"$PROJECT_ID\",\"visitor_id\":\"33333333-3333-3333-3333-333333333333\",\"type\":\"error\",\"name\":\"unhandled_exception\"}"
pass "track error event"

IDENTIFY_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE_URL/api/v1/identify" \
  -H "Content-Type: application/json" \
  -d "{\"project_id\":\"$PROJECT_ID\",\"visitor_id\":\"11111111-1111-1111-1111-111111111111\",\"name\":\"Antonio (yo mismo)\"}")
[ "$IDENTIFY_STATUS" = "202" ] || fail "identify returned $IDENTIFY_STATUS"
pass "identify"

STATS=$(curl -sf "$BASE_URL/api/v1/stats?since=1970-01-01T00:00:00Z" \
  -H "Authorization: Bearer $API_KEY") || fail "stats request failed"
echo "$STATS" | grep -q '"total_events":3' || fail "expected total_events=3, got: $STATS"
pass "stats (no filters) -> total_events=3"

STATS_EXCL=$(curl -sf "$BASE_URL/api/v1/stats?since=1970-01-01T00:00:00Z&exclude_named=true" \
  -H "Authorization: Bearer $API_KEY") || fail "stats (exclude_named) request failed"
echo "$STATS_EXCL" | grep -q '"total_events":2' || fail "expected total_events=2 with exclude_named=true, got: $STATS_EXCL"
pass "stats (exclude_named=true) -> total_events=2"

UNAUTH_CODE=$(curl -s -o /dev/null -w '%{http_code}' "$BASE_URL/api/v1/stats")
[ "$UNAUTH_CODE" = "401" ] || fail "expected 401 for stats without api key, got $UNAUTH_CODE"
pass "stats without api key -> 401"

curl -sf "$BASE_URL/dashboard/login" | grep -q "Traccia" || fail "/dashboard/login did not render"
pass "GET /dashboard/login"

DASHBOARD_REDIRECT=$(curl -s -o /dev/null -w '%{http_code}' "$BASE_URL/dashboard")
[ "$DASHBOARD_REDIRECT" = "303" ] || fail "expected 303 redirect to login for /dashboard without a session, got $DASHBOARD_REDIRECT"
pass "GET /dashboard without session -> 303 redirect to login"

DASHBOARD_LOGIN_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -c /tmp/traccia-smoke-cookies.txt -X POST "$BASE_URL/dashboard/login" \
  --data-urlencode "api_key=$API_KEY")
[ "$DASHBOARD_LOGIN_STATUS" = "303" ] || fail "expected 303 after dashboard login, got $DASHBOARD_LOGIN_STATUS"
curl -sf -b /tmp/traccia-smoke-cookies.txt "$BASE_URL/dashboard" | grep -q "Eventos totales" || fail "dashboard did not render stats after login"
rm -f /tmp/traccia-smoke-cookies.txt
pass "dashboard login + overview render"

ADMIN_REDIRECT=$(curl -s -o /dev/null -w '%{http_code}' "$BASE_URL/admin")
[ "$ADMIN_REDIRECT" = "303" ] || fail "expected 303 redirect (to setup or login) for /admin without a session, got $ADMIN_REDIRECT"
pass "GET /admin without session -> 303 redirect"

# The admin panel has its own one-time account setup (username/password),
# separate from ADMIN_TOKEN (which stays purely an API credential — see
# the Admin panel section of the README). Only the first run needs
# /admin/setup; a re-run against a database that already has an account
# would hit /admin/login instead, so this script tolerates either.
ADMIN_USERNAME="smoketest"
ADMIN_PASSWORD="smoke-test-password-123"

NEEDS_SETUP=$(curl -s -o /dev/null -w '%{http_code}' "$BASE_URL/admin/setup")
if [ "$NEEDS_SETUP" = "200" ]; then
  ADMIN_LOGIN_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -c /tmp/traccia-smoke-admin-cookies.txt -X POST "$BASE_URL/admin/setup" \
    --data-urlencode "username=$ADMIN_USERNAME" --data-urlencode "password=$ADMIN_PASSWORD" --data-urlencode "password_confirm=$ADMIN_PASSWORD")
  [ "$ADMIN_LOGIN_STATUS" = "303" ] || fail "expected 303 after admin setup, got $ADMIN_LOGIN_STATUS"
  pass "admin setup creates the first account"
else
  ADMIN_LOGIN_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -c /tmp/traccia-smoke-admin-cookies.txt -X POST "$BASE_URL/admin/login" \
    --data-urlencode "username=$ADMIN_USERNAME" --data-urlencode "password=$ADMIN_PASSWORD")
  [ "$ADMIN_LOGIN_STATUS" = "303" ] || fail "expected 303 after admin login, got $ADMIN_LOGIN_STATUS (an account from a prior run may use different credentials)"
  pass "admin login"
fi

curl -sf -b /tmp/traccia-smoke-admin-cookies.txt "$BASE_URL/admin" | grep -q "Smoke Test" || fail "admin panel did not list the project created earlier"
pass "admin project list"

ADMIN_CREATE_HTML=$(curl -sf -b /tmp/traccia-smoke-admin-cookies.txt -X POST "$BASE_URL/admin/projects/new" \
  --data-urlencode "name=Admin Panel Smoke" --data-urlencode "domain=admin-smoke.example.com")
echo "$ADMIN_CREATE_HTML" | grep -q "trc_" || fail "admin project creation did not reveal an API key"
ADMIN_PROJECT_ID=$(echo "$ADMIN_CREATE_HTML" | grep -oE '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' | head -1)
[ -n "$ADMIN_PROJECT_ID" ] || fail "could not extract project_id from admin creation response"
pass "admin creates project -> $ADMIN_PROJECT_ID"

ADMIN_VIEW_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -b /tmp/traccia-smoke-admin-cookies.txt -c /tmp/traccia-smoke-admin-cookies.txt \
  -X POST "$BASE_URL/admin/projects/$ADMIN_PROJECT_ID/view")
[ "$ADMIN_VIEW_STATUS" = "303" ] || fail "expected 303 from admin's view-dashboard action, got $ADMIN_VIEW_STATUS"
curl -sf -b /tmp/traccia-smoke-admin-cookies.txt "$BASE_URL/dashboard" | grep -q "Eventos totales" || fail "dashboard did not render after admin's view-dashboard jump"
curl -sf -b /tmp/traccia-smoke-admin-cookies.txt "$BASE_URL/dashboard" | grep -q 'href="/admin"' || fail "dashboard did not show a link back to the admin panel"
pass "admin 'ver dashboard' jump mints a working dashboard session, with a nav link back"

curl -sf -o /dev/null -w '%{http_code}\n' "$BASE_URL/assets/theme.css" | grep -q "200" || fail "/assets/theme.css did not serve the shared stylesheet"
pass "shared theme asset served at /assets/theme.css"

# --- users management ---
curl -sf -b /tmp/traccia-smoke-admin-cookies.txt "$BASE_URL/admin/users" | grep -q "$ADMIN_USERNAME" || fail "/admin/users did not list the current account"
pass "GET /admin/users lists the current account"

ADD_USER_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -b /tmp/traccia-smoke-admin-cookies.txt -X POST "$BASE_URL/admin/users/new" \
  --data-urlencode "username=smoke_teammate" --data-urlencode "password=another-smoke-password")
[ "$ADD_USER_STATUS" = "303" ] || fail "expected 303 after adding a second admin user, got $ADD_USER_STATUS"
curl -sf -b /tmp/traccia-smoke-admin-cookies.txt "$BASE_URL/admin/users" | grep -q "smoke_teammate" || fail "second admin account did not appear in the list"
pass "admin can add a second account"

# --- delete project ---
DELETE_CONFIRM_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -b /tmp/traccia-smoke-admin-cookies.txt "$BASE_URL/admin/projects/$ADMIN_PROJECT_ID/delete")
[ "$DELETE_CONFIRM_STATUS" = "200" ] || fail "expected 200 on delete confirmation page, got $DELETE_CONFIRM_STATUS"
DELETE_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -b /tmp/traccia-smoke-admin-cookies.txt -X POST "$BASE_URL/admin/projects/$ADMIN_PROJECT_ID/delete")
[ "$DELETE_STATUS" = "303" ] || fail "expected 303 after deleting a project, got $DELETE_STATUS"
if curl -sf -b /tmp/traccia-smoke-admin-cookies.txt "$BASE_URL/admin" | grep -q "Admin Panel Smoke"; then
  fail "deleted project still appears in the list"
fi
pass "admin can delete a project and it disappears from the list"

rm -f /tmp/traccia-smoke-admin-cookies.txt

echo
echo "ALL SMOKE CHECKS PASSED"
