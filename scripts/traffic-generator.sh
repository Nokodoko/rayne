#!/usr/bin/env bash

# Rayne Traffic Generator
# Generates realistic API traffic for APM demo purposes
# Usage: ./traffic-generator.sh start [url] | stop | status

set -e

SCRIPT_DIR="$(dirname "$0")"
PID_FILE="/tmp/rayne-traffic-generator.pid"
LOG_FILE="/tmp/rayne-traffic-generator.log"
DEFAULT_URL="http://localhost:8080"
FAILURE_RATE="${FAILURE_RATE:-10}" # Percentage chance of failure per cycle (default 10%)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

generate_failure() {
  local BASE_URL="$1"
  local FAILURE_TYPE=$((RANDOM % 15))

  # Mix of real parsing errors and intentional demo errors
  case $FAILURE_TYPE in
  # 5xx Server Errors - Using /v1/demo/error endpoint
  0) # 500 - Internal server error
    curl -s "$BASE_URL/v1/demo/error?type=server" >/dev/null 2>&1 ;;
  1) # 502 - Bad gateway
    curl -s "$BASE_URL/v1/demo/error?type=upstream" >/dev/null 2>&1 ;;
  2) # 503 - Service unavailable
    curl -s "$BASE_URL/v1/demo/error?type=unavailable" >/dev/null 2>&1 ;;
  3) # 504 - Gateway timeout
    curl -s "$BASE_URL/v1/demo/error?type=timeout" >/dev/null 2>&1 ;;
  4) # 500 - Database error
    curl -s "$BASE_URL/v1/demo/error?type=database" >/dev/null 2>&1 ;;
  5) # Random error type
    curl -s "$BASE_URL/v1/demo/error?type=random" >/dev/null 2>&1 ;;
  # 4xx Client Errors - Using /v1/demo/error endpoint
  6) # 400 - Validation error
    curl -s "$BASE_URL/v1/demo/error?type=validation" >/dev/null 2>&1 ;;
  7) # 401 - Auth error
    curl -s "$BASE_URL/v1/demo/error?type=auth" >/dev/null 2>&1 ;;
  8) # 403 - Forbidden
    curl -s "$BASE_URL/v1/demo/error?type=forbidden" >/dev/null 2>&1 ;;
  9) # 404 - Not found
    curl -s "$BASE_URL/v1/demo/error?type=notfound" >/dev/null 2>&1 ;;
  10) # 429 - Rate limit
    curl -s "$BASE_URL/v1/demo/error?type=ratelimit" >/dev/null 2>&1 ;;
  # Real parsing errors (400)
  11) # 400 - Malformed JSON to logs/search
    curl -s -X POST "$BASE_URL/v1/logs/search" -H "Content-Type: application/json" -d '{invalid json}' >/dev/null 2>&1 ;;
  12) # 400 - Malformed JSON to webhooks/create
    curl -s -X POST "$BASE_URL/v1/webhooks/create" -H "Content-Type: application/json" -d '{broken}' >/dev/null 2>&1 ;;
  # Real method errors (405)
  13) # 405 - POST to GET-only endpoint
    curl -s -X POST "$BASE_URL/v1/monitors" >/dev/null 2>&1 ;;
  14) # 405 - DELETE to GET-only endpoint
    curl -s -X DELETE "$BASE_URL/v1/hosts" >/dev/null 2>&1 ;;
  esac
}

maybe_inject_failure() {
  local BASE_URL="$1"
  if [ $((RANDOM % 100)) -lt "$FAILURE_RATE" ]; then
    generate_failure "$BASE_URL" &
  fi
}

generate_traffic() {
  local BASE_URL="$1"

  echo "$(date): Starting traffic generation to $BASE_URL (failure rate: ${FAILURE_RATE}%)" >>"$LOG_FILE"

  while true; do
    # Health checks (frequent)
    curl -s "$BASE_URL/health" >/dev/null 2>&1 &
    maybe_inject_failure "$BASE_URL"
    sleep 0.$((RANDOM % 4 + 1))

    # Downtimes endpoint
    curl -s "$BASE_URL/v1/downtimes" >/dev/null 2>&1 &
    maybe_inject_failure "$BASE_URL"
    sleep 0.$((RANDOM % 3 + 1))

    # Events endpoint
    curl -s "$BASE_URL/v1/events" >/dev/null 2>&1 &
    maybe_inject_failure "$BASE_URL"
    sleep 0.$((RANDOM % 3 + 1))

    # Hosts endpoints
    curl -s "$BASE_URL/v1/hosts" >/dev/null 2>&1 &
    maybe_inject_failure "$BASE_URL"
    sleep 0.$((RANDOM % 2 + 1))

    curl -s "$BASE_URL/v1/hosts/active" >/dev/null 2>&1 &
    maybe_inject_failure "$BASE_URL"
    sleep 0.$((RANDOM % 3 + 1))

    # Monitors endpoints
    curl -s "$BASE_URL/v1/monitors" >/dev/null 2>&1 &
    maybe_inject_failure "$BASE_URL"
    sleep 0.$((RANDOM % 3 + 1))

    curl -s "$BASE_URL/v1/monitors/triggered" >/dev/null 2>&1 &
    maybe_inject_failure "$BASE_URL"
    sleep 0.$((RANDOM % 2 + 1))

    # Webhooks stats
    curl -s "$BASE_URL/v1/webhooks/stats" >/dev/null 2>&1 &
    maybe_inject_failure "$BASE_URL"
    sleep 0.$((RANDOM % 3 + 1))

    curl -s "$BASE_URL/v1/webhooks/events" >/dev/null 2>&1 &
    maybe_inject_failure "$BASE_URL"
    sleep 0.$((RANDOM % 2 + 1))

    # RUM endpoints
    curl -s "$BASE_URL/v1/rum/visitors" >/dev/null 2>&1 &
    maybe_inject_failure "$BASE_URL"
    sleep 0.$((RANDOM % 3 + 1))

    curl -s "$BASE_URL/v1/rum/analytics" >/dev/null 2>&1 &
    maybe_inject_failure "$BASE_URL"
    sleep 0.$((RANDOM % 2 + 1))

    curl -s "$BASE_URL/v1/rum/sessions" >/dev/null 2>&1 &
    maybe_inject_failure "$BASE_URL"
    sleep 0.$((RANDOM % 3 + 1))

    # Service catalog
    curl -s "$BASE_URL/v1/services" >/dev/null 2>&1 &
    maybe_inject_failure "$BASE_URL"
    sleep 0.$((RANDOM % 3 + 1))

    # Demo status
    curl -s "$BASE_URL/v1/demo/status" >/dev/null 2>&1 &
    maybe_inject_failure "$BASE_URL"
    sleep 0.$((RANDOM % 2 + 1))

    # Simulate RUM visitor init (POST)
    curl -s -X POST "$BASE_URL/v1/rum/init" \
      -H "Content-Type: application/json" \
      -d '{"user_agent":"TrafficGenerator/1.0","referrer":"https://demo.example.com"}' >/dev/null 2>&1 &
    maybe_inject_failure "$BASE_URL"
    sleep 0.$((RANDOM % 4 + 1))

    # Simulate log search (POST)
    curl -s -X POST "$BASE_URL/v1/logs/search" \
      -H "Content-Type: application/json" \
      -d '{"query":"service:rayne","from":"-1h","to":"now"}' >/dev/null 2>&1 &
    maybe_inject_failure "$BASE_URL"
    sleep 0.$((RANDOM % 3 + 1))

    # Wait for background jobs to complete before next cycle
    wait

    # Cycle delay (1-3 seconds between full cycles)
    sleep $((RANDOM % 3 + 1))

    echo "$(date): Completed traffic cycle" >>"$LOG_FILE"
  done
}

start_traffic() {
  local URL="${1:-$DEFAULT_URL}"

  if [ -f "$PID_FILE" ]; then
    local OLD_PID=$(cat "$PID_FILE")
    if kill -0 "$OLD_PID" 2>/dev/null; then
      echo -e "${YELLOW}Traffic generator is already running (PID: $OLD_PID)${NC}"
      echo "Use './traffic-generator.sh stop' to stop it first"
      exit 1
    else
      rm -f "$PID_FILE"
    fi
  fi

  # Test connection first
  echo -e "${YELLOW}Testing connection to $URL...${NC}"
  if ! curl -s --connect-timeout 5 "$URL/health" >/dev/null 2>&1; then
    echo -e "${RED}Error: Cannot connect to $URL/health${NC}"
    echo "Make sure the Rayne server is running"
    exit 1
  fi
  echo -e "${GREEN}Connection successful!${NC}"

  # Clear old log
  >"$LOG_FILE"

  # Start traffic generator in background
  echo -e "${GREEN}Starting traffic generator...${NC}"
  generate_traffic "$URL" &
  local PID=$!
  echo $PID >"$PID_FILE"

  echo -e "${GREEN}Traffic generator started (PID: $PID)${NC}"
  echo "Target: $URL"
  echo "Failure rate: ${FAILURE_RATE}%"
  echo ""
  echo "Monitor with:"
  echo "  tail -f $LOG_FILE"
  echo ""
  echo "Stop with:"
  echo "  ./traffic-generator.sh stop"
}

stop_traffic() {
  if [ ! -f "$PID_FILE" ]; then
    echo -e "${YELLOW}Traffic generator is not running${NC}"
    exit 0
  fi

  local PID=$(cat "$PID_FILE")

  if kill -0 "$PID" 2>/dev/null; then
    echo -e "${YELLOW}Stopping traffic generator (PID: $PID)...${NC}"

    # Kill the main process and all child processes
    pkill -P "$PID" 2>/dev/null || true
    kill "$PID" 2>/dev/null || true

    # Wait a moment and force kill if needed
    sleep 1
    if kill -0 "$PID" 2>/dev/null; then
      kill -9 "$PID" 2>/dev/null || true
    fi

    rm -f "$PID_FILE"
    echo -e "${GREEN}Traffic generator stopped${NC}"
  else
    echo -e "${YELLOW}Process $PID is not running, cleaning up...${NC}"
    rm -f "$PID_FILE"
  fi
}

status_traffic() {
  if [ ! -f "$PID_FILE" ]; then
    echo -e "${YELLOW}Traffic generator is not running${NC}"
    exit 0
  fi

  local PID=$(cat "$PID_FILE")

  if kill -0 "$PID" 2>/dev/null; then
    echo -e "${GREEN}Traffic generator is running (PID: $PID)${NC}"
    echo ""
    echo "Recent activity:"
    tail -5 "$LOG_FILE" 2>/dev/null || echo "  No log entries yet"
  else
    echo -e "${YELLOW}Traffic generator is not running (stale PID file)${NC}"
    rm -f "$PID_FILE"
  fi
}

show_help() {
  echo "Rayne Traffic Generator"
  echo ""
  echo "Usage: $0 <command> [options]"
  echo ""
  echo "Commands:"
  echo "  start [url]    Start generating traffic (default: $DEFAULT_URL)"
  echo "  stop           Stop the traffic generator"
  echo "  status         Check if traffic generator is running"
  echo "  help           Show this help message"
  echo ""
  echo "Environment Variables:"
  echo "  FAILURE_RATE   Percentage chance of injecting 4xx/5xx errors per cycle (default: 10)"
  echo ""
  echo "Examples:"
  echo "  $0 start                              # Use default localhost:8080, 10% failure rate"
  echo "  $0 start http://192.168.49.2:30080    # Use minikube URL"
  echo "  FAILURE_RATE=20 $0 start              # 20% failure rate"
  echo "  FAILURE_RATE=0 $0 start               # No failures"
  echo "  $0 stop"
  echo "  $0 status"
}

# Main
case "${1:-help}" in
start)
  start_traffic "$2"
  ;;
stop)
  stop_traffic
  ;;
status)
  status_traffic
  ;;
help | --help | -h)
  show_help
  ;;
*)
  echo -e "${RED}Unknown command: $1${NC}"
  echo ""
  show_help
  exit 1
  ;;
esac
