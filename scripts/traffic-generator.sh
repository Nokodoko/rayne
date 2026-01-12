#!/usr/bin/env bash

# Rayne Traffic Generator
# Generates realistic API traffic for APM demo purposes
# Usage: ./traffic-generator.sh start [url] | stop | status

set -e

SCRIPT_DIR="$(dirname "$0")"
PID_FILE="/tmp/rayne-traffic-generator.pid"
LOG_FILE="/tmp/rayne-traffic-generator.log"
DEFAULT_URL="http://localhost:8080"
FAILURE_RATE="${FAILURE_RATE:-10}"  # Percentage chance of failure per cycle (default 10%)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

generate_failure() {
    local BASE_URL="$1"
    local FAILURE_TYPE=$(( RANDOM % 10 ))

    # All cases verified to return actual 4xx/5xx error codes
    case $FAILURE_TYPE in
        # 4xx Client Errors - Bad JSON payloads (400)
        0) # 400 - Malformed JSON to logs/search
           curl -s -X POST "$BASE_URL/v1/logs/search" -H "Content-Type: application/json" -d '{invalid json}' > /dev/null 2>&1 ;;
        1) # 400 - Malformed JSON to webhooks/create
           curl -s -X POST "$BASE_URL/v1/webhooks/create" -H "Content-Type: application/json" -d '{broken}' > /dev/null 2>&1 ;;
        2) # 400 - Malformed JSON to services/definitions
           curl -s -X POST "$BASE_URL/v1/services/definitions" -H "Content-Type: application/json" -d '{"schema-version": "invalid", "dd-service": ""}' > /dev/null 2>&1 ;;
        3) # 400 - Malformed JSON to rum/track
           curl -s -X POST "$BASE_URL/v1/rum/track" -H "Content-Type: application/json" -d '{"visitor_uuid": "bad-uuid", "session_id": -1}' > /dev/null 2>&1 ;;
        # 4xx Client Errors - Wrong HTTP methods (405)
        4) # 405 - POST to GET-only endpoint (monitors)
           curl -s -X POST "$BASE_URL/v1/monitors" > /dev/null 2>&1 ;;
        5) # 405 - DELETE to GET-only endpoint (hosts)
           curl -s -X DELETE "$BASE_URL/v1/hosts" > /dev/null 2>&1 ;;
        6) # 405 - PUT to POST-only endpoint (logs/search)
           curl -s -X PUT "$BASE_URL/v1/logs/search" -H "Content-Type: application/json" -d '{}' > /dev/null 2>&1 ;;
        # 4xx Client Errors - Other errors
        7) # 403 - Advanced log search with invalid params
           curl -s -X POST "$BASE_URL/v1/logs/search/advanced" -H "Content-Type: application/json" -d '{"filter": {"query": "", "from": "invalid-time"}}' > /dev/null 2>&1 ;;
        8) # 400 - Malformed JSON to webhooks/config
           curl -s -X POST "$BASE_URL/v1/webhooks/config" -H "Content-Type: application/json" -d '{not valid json at all' > /dev/null 2>&1 ;;
        9) # 400 - Malformed JSON to services/definitions/advanced
           curl -s -X POST "$BASE_URL/v1/services/definitions/advanced" -H "Content-Type: application/json" -d '{"broken":' > /dev/null 2>&1 ;;
    esac
}

generate_traffic() {
    local BASE_URL="$1"

    echo "$(date): Starting traffic generation to $BASE_URL (failure rate: ${FAILURE_RATE}%)" >> "$LOG_FILE"

    while true; do
        # Health checks (frequent)
        curl -s "$BASE_URL/health" > /dev/null 2>&1 &

        # Random delay between requests (100ms - 500ms)
        sleep 0.$(( RANDOM % 4 + 1 ))

        # Downtimes endpoint
        curl -s "$BASE_URL/v1/downtimes" > /dev/null 2>&1 &
        sleep 0.$(( RANDOM % 3 + 1 ))

        # Events endpoint
        curl -s "$BASE_URL/v1/events" > /dev/null 2>&1 &
        sleep 0.$(( RANDOM % 3 + 1 ))

        # Hosts endpoints
        curl -s "$BASE_URL/v1/hosts" > /dev/null 2>&1 &
        sleep 0.$(( RANDOM % 2 + 1 ))

        curl -s "$BASE_URL/v1/hosts/active" > /dev/null 2>&1 &
        sleep 0.$(( RANDOM % 3 + 1 ))

        # Monitors endpoints
        curl -s "$BASE_URL/v1/monitors" > /dev/null 2>&1 &
        sleep 0.$(( RANDOM % 3 + 1 ))

        curl -s "$BASE_URL/v1/monitors/triggered" > /dev/null 2>&1 &
        sleep 0.$(( RANDOM % 2 + 1 ))

        # Webhooks stats
        curl -s "$BASE_URL/v1/webhooks/stats" > /dev/null 2>&1 &
        sleep 0.$(( RANDOM % 3 + 1 ))

        curl -s "$BASE_URL/v1/webhooks/events" > /dev/null 2>&1 &
        sleep 0.$(( RANDOM % 2 + 1 ))

        # RUM endpoints
        curl -s "$BASE_URL/v1/rum/visitors" > /dev/null 2>&1 &
        sleep 0.$(( RANDOM % 3 + 1 ))

        curl -s "$BASE_URL/v1/rum/analytics" > /dev/null 2>&1 &
        sleep 0.$(( RANDOM % 2 + 1 ))

        curl -s "$BASE_URL/v1/rum/sessions" > /dev/null 2>&1 &
        sleep 0.$(( RANDOM % 3 + 1 ))

        # Service catalog
        curl -s "$BASE_URL/v1/services" > /dev/null 2>&1 &
        sleep 0.$(( RANDOM % 3 + 1 ))

        # Demo status
        curl -s "$BASE_URL/v1/demo/status" > /dev/null 2>&1 &
        sleep 0.$(( RANDOM % 2 + 1 ))

        # Simulate RUM visitor init (POST)
        curl -s -X POST "$BASE_URL/v1/rum/init" \
            -H "Content-Type: application/json" \
            -d '{"user_agent":"TrafficGenerator/1.0","referrer":"https://demo.example.com"}' > /dev/null 2>&1 &
        sleep 0.$(( RANDOM % 4 + 1 ))

        # Simulate log search (POST)
        curl -s -X POST "$BASE_URL/v1/logs/search" \
            -H "Content-Type: application/json" \
            -d '{"query":"service:rayne","from":"-1h","to":"now"}' > /dev/null 2>&1 &
        sleep 0.$(( RANDOM % 3 + 1 ))

        # Random failure injection based on FAILURE_RATE
        if [ $(( RANDOM % 100 )) -lt "$FAILURE_RATE" ]; then
            generate_failure "$BASE_URL" &
            echo "$(date): Injected test failure (rate: ${FAILURE_RATE}%)" >> "$LOG_FILE"
        fi

        # Wait for background jobs to complete before next cycle
        wait

        # Cycle delay (1-3 seconds between full cycles)
        sleep $(( RANDOM % 3 + 1 ))

        echo "$(date): Completed traffic cycle" >> "$LOG_FILE"
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
    if ! curl -s --connect-timeout 5 "$URL/health" > /dev/null 2>&1; then
        echo -e "${RED}Error: Cannot connect to $URL/health${NC}"
        echo "Make sure the Rayne server is running"
        exit 1
    fi
    echo -e "${GREEN}Connection successful!${NC}"

    # Clear old log
    > "$LOG_FILE"

    # Start traffic generator in background
    echo -e "${GREEN}Starting traffic generator...${NC}"
    generate_traffic "$URL" &
    local PID=$!
    echo $PID > "$PID_FILE"

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
    help|--help|-h)
        show_help
        ;;
    *)
        echo -e "${RED}Unknown command: $1${NC}"
        echo ""
        show_help
        exit 1
        ;;
esac
