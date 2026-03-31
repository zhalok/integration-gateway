#!/usr/bin/env bash
# test_create_enrichment.sh — exercises the create enrichment API end-to-end.
#
# Usage:
#   ./scripts/test_create_enrichment.sh [BASE_URL]
#
# Defaults to http://localhost:8080 if BASE_URL is not provided.
# Requires: curl. Uses jq for pretty-printing if available.

set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"
POLL_INTERVAL=3   # seconds between status polls
POLL_TIMEOUT=60   # max seconds to wait for a case to finish

# ---------------------------------------------------------------------------
# Logging — tee all output to a timestamped log file
# ---------------------------------------------------------------------------

LOG_DIR="$(cd "$(dirname "$0")" && pwd)/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/test_$(date +%Y%m%d_%H%M%S).log"
exec > >(tee "$LOG_FILE") 2>&1
echo "Session log: $LOG_FILE"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

print_json() {
  if command -v jq &>/dev/null; then
    echo "$1" | jq .
  else
    echo "$1"
  fi
}

get_field() {
  # get_field <json> <key>  — returns the value of a top-level string key
  if command -v jq &>/dev/null; then
    echo "$1" | jq -r ".$2 // empty"
  else
    echo "$1" | grep -o "\"$2\":\"[^\"]*\"" | head -1 | cut -d'"' -f4
  fi
}

separator() {
  echo ""
  echo "================================================================"
  echo "$1"
  echo "================================================================"
}

# ---------------------------------------------------------------------------
# Health check
# ---------------------------------------------------------------------------

separator "Health check"
health=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/health")
if [ "$health" != "200" ]; then
  echo "ERROR: server not reachable at $BASE_URL (HTTP $health)"
  echo "Make sure the gateway is running: docker-compose up"
  exit 1
fi
echo "Server is up at $BASE_URL"

# ---------------------------------------------------------------------------
# poll_until_done <case_id>
#   Polls GET /api/cases/{id}/enrichment every POLL_INTERVAL seconds until
#   the status is no longer "pending", or until POLL_TIMEOUT is reached.
# ---------------------------------------------------------------------------

poll_until_done() {
  local case_id="$1"
  local elapsed=0

  while [ "$elapsed" -lt "$POLL_TIMEOUT" ]; do
    response=$(curl -s "$BASE_URL/api/cases/$case_id/enrichment")
    status=$(get_field "$response" "status")

    echo "  [+${elapsed}s] status=$status"

    if [ "$status" != "pending" ]; then
      echo ""
      echo "Final enrichment state:"
      print_json "$response"
      return 0
    fi

    sleep "$POLL_INTERVAL"
    elapsed=$((elapsed + POLL_INTERVAL))
  done

  echo "TIMEOUT: enrichment still pending after ${POLL_TIMEOUT}s"
  return 1
}

# ---------------------------------------------------------------------------
# Test 1 — case-001: no court case number
#   Expected: PR=success, CR=not_applicable, SCRA=success
# ---------------------------------------------------------------------------

separator "Test 1: case-001 (no court case number — CR should be not_applicable)"

echo "POST /api/cases/case-001/enrich"
resp=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/api/cases/case-001/enrich")
body=$(echo "$resp" | sed '/HTTP_STATUS/d')
http_status=$(echo "$resp" | grep "HTTP_STATUS" | cut -d: -f2)

echo "Response HTTP $http_status:"
print_json "$body"

echo ""
echo "Polling until done..."
poll_until_done "case-001"

# ---------------------------------------------------------------------------
# Test 2 — case-002: has court case number
#   Expected: all three sources attempted
# ---------------------------------------------------------------------------

separator "Test 2: case-002 (has court case number — all three sources)"

echo "POST /api/cases/case-002/enrich"
resp=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/api/cases/case-002/enrich")
body=$(echo "$resp" | sed '/HTTP_STATUS/d')
http_status=$(echo "$resp" | grep "HTTP_STATUS" | cut -d: -f2)

echo "Response HTTP $http_status:"
print_json "$body"

echo ""
echo "Polling until done..."
poll_until_done "case-002"

# ---------------------------------------------------------------------------
# Test 3 — case-006: property records returns 404 (permanent failure)
#   Expected: PR=failed (permanent), CR=success, SCRA=success
# ---------------------------------------------------------------------------

separator "Test 3: case-006 (property records 404 — permanent failure path)"

echo "POST /api/cases/case-006/enrich"
resp=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/api/cases/case-006/enrich")
body=$(echo "$resp" | sed '/HTTP_STATUS/d')
http_status=$(echo "$resp" | grep "HTTP_STATUS" | cut -d: -f2)

echo "Response HTTP $http_status:"
print_json "$body"

echo ""
echo "Polling until done..."
poll_until_done "case-006"

# ---------------------------------------------------------------------------
# Test 4 — idempotency: POST to case-001 again after it completed
#   Expected: 200 (not 202), same enrichment returned without re-queuing
# ---------------------------------------------------------------------------

separator "Test 4: idempotency — POST to case-001 again (should return 200 complete)"

echo "POST /api/cases/case-001/enrich (second call)"
resp=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/api/cases/case-001/enrich")
body=$(echo "$resp" | sed '/HTTP_STATUS/d')
http_status=$(echo "$resp" | grep "HTTP_STATUS" | cut -d: -f2)

echo "Response HTTP $http_status:"
print_json "$body"

if [ "$http_status" = "200" ]; then
  echo "OK: got 200 as expected for a completed enrichment"
else
  echo "UNEXPECTED: expected 200, got $http_status"
fi

# ---------------------------------------------------------------------------
# Test 5 — unknown case: 404 response
# ---------------------------------------------------------------------------

separator "Test 5: unknown case — should return 404"

echo "POST /api/cases/case-does-not-exist/enrich"
resp=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/api/cases/case-does-not-exist/enrich")
body=$(echo "$resp" | sed '/HTTP_STATUS/d')
http_status=$(echo "$resp" | grep "HTTP_STATUS" | cut -d: -f2)

echo "Response HTTP $http_status:"
print_json "$body"

if [ "$http_status" = "404" ]; then
  echo "OK: got 404 as expected"
else
  echo "UNEXPECTED: expected 404, got $http_status"
fi

# ---------------------------------------------------------------------------

separator "Done"
echo "All tests completed. Check server logs for the async job activity."
