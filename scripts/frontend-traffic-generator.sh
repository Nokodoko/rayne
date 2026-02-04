#!/usr/bin/env bash

# Frontend Traffic Generator for Rayne RUM
# Generates realistic frontend traffic with proper RUM integration.
# 25% of visitors are new users (get UUIDs from backend), 75% are returning.
#
# Usage: ./frontend-traffic-generator.sh [OPTIONS] COMMAND

set -e

SCRIPT_DIR="$(dirname "$0")"
PID_FILE="/tmp/rayne-frontend-traffic.pid"
LOG_FILE="/tmp/rayne-frontend-traffic.log"
POOL_FILE="/tmp/rayne-visitor-pool.txt"
MAX_POOL_SIZE=100

# Default configuration (can be overridden by env vars or CLI args)
FRONTEND_URL="${FRONTEND_URL:-http://localhost:3000}"
BACKEND_URL="${BACKEND_URL:-http://localhost:8080}"
NEW_USER_RATE="${NEW_USER_RATE:-25}"
VERBOSE="${VERBOSE:-false}"
SEND_TO_DATADOG="${SEND_TO_DATADOG:-false}"

# Datadog RUM configuration (from frontend/static/js/datadog-rum-init.js)
DD_RUM_APPLICATION_ID="${DD_RUM_APPLICATION_ID:-6d730a61-be91-4cec-80fb-80848bb29d14}"
DD_RUM_CLIENT_TOKEN="${DD_RUM_CLIENT_TOKEN:-pub902cdeb5b6dd38e7179c22ec46cf6112}"
DD_RUM_SITE="${DD_RUM_SITE:-datadoghq.com}"
DD_RUM_SERVICE="${DD_RUM_SERVICE:-rayne-frontend}"
DD_RUM_ENV="${DD_RUM_ENV:-staging}"
DD_RUM_VERSION="${DD_RUM_VERSION:-2.1.0}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# User agents for realistic browser simulation
USER_AGENTS=(
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15"
    "Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0"
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0"
    "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
    "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
    "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36"
)

# Page URLs to track
PAGE_URLS=(
    "/"
    "/#about"
    "/#projects"
    "/#contact"
)

# Referrers for realistic traffic sources
REFERRERS=(
    "https://www.google.com/search?q=portfolio"
    "https://github.com/Nokodoko"
    "https://www.linkedin.com/in/"
    "https://twitter.com/"
    ""  # Direct traffic
    ""  # Direct traffic
)

# Action types to simulate
ACTION_TYPES=(
    "button_click"
    "link_hover"
    "scroll"
    "form_focus"
    "navigation"
)

log() {
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo "[$timestamp] $*" >> "$LOG_FILE"
    if [ "$VERBOSE" = "true" ]; then
        echo -e "${BLUE}[$timestamp]${NC} $*"
    fi
}

# Generate a random UUID without dashes (32 hex chars) - Datadog format
generate_dd_id() {
    cat /proc/sys/kernel/random/uuid | tr -d '-'
}

# Send RUM event directly to Datadog intake API
# Uses the same format as the browser SDK
send_to_datadog_rum() {
    local event_type="$1"
    local visitor_uuid="$2"
    local session_id="$3"
    local user_agent="$4"
    local page_url="$5"
    local view_id="$6"
    local view_data="$7"

    if [ "$SEND_TO_DATADOG" != "true" ]; then
        return 0
    fi

    local now_ms=$(($(date +%s) * 1000))
    # Convert session_id to Datadog format (32 hex chars, no dashes)
    local dd_session_id=$(echo -n "$session_id" | tr -d '-')
    local dd_view_id="${view_id:-$(generate_dd_id)}"

    # Parse URL for host and path
    local url_host=$(echo "$page_url" | sed -E 's|https?://([^/]+).*|\1|')
    local url_path=$(echo "$page_url" | sed -E 's|https?://[^/]+(/[^?#]*)?.*|\1|')
    [ -z "$url_path" ] && url_path="/"

    # Detect device type from user agent
    local device_type="desktop"
    if echo "$user_agent" | grep -qiE "mobile|android|iphone|ipad"; then
        device_type="mobile"
    fi

    # Detect OS from user agent
    local os_name="Linux"
    if echo "$user_agent" | grep -qi "windows"; then
        os_name="Windows"
    elif echo "$user_agent" | grep -qi "mac"; then
        os_name="Mac OS X"
    elif echo "$user_agent" | grep -qiE "iphone|ipad"; then
        os_name="iOS"
    elif echo "$user_agent" | grep -qi "android"; then
        os_name="Android"
    fi

    # Detect browser from user agent
    local browser_name="Chrome"
    if echo "$user_agent" | grep -qi "firefox"; then
        browser_name="Firefox"
    elif echo "$user_agent" | grep -qi "safari" && ! echo "$user_agent" | grep -qi "chrome"; then
        browser_name="Safari"
    elif echo "$user_agent" | grep -qi "edge"; then
        browser_name="Edge"
    fi

    # Build the RUM event payload - matches SDK format exactly
    # Required fields: view.loading_type, _dd.document_version, _dd.configuration
    local payload="{\"application\":{\"id\":\"$DD_RUM_APPLICATION_ID\"},\"date\":$now_ms,\"service\":\"$DD_RUM_SERVICE\",\"version\":\"$DD_RUM_VERSION\",\"source\":\"browser\",\"env\":\"$DD_RUM_ENV\",\"session\":{\"id\":\"$dd_session_id\",\"type\":\"user\",\"has_replay\":false},\"type\":\"$event_type\",\"view\":{\"id\":\"$dd_view_id\",\"url\":\"$page_url\",\"url_host\":\"$url_host\",\"url_path\":\"$url_path\",\"referrer\":\"\",\"loading_type\":\"initial_load\"$view_data},\"usr\":{\"id\":\"$visitor_uuid\"},\"_dd\":{\"format_version\":2,\"drift\":0,\"session\":{\"plan\":1},\"document_version\":1,\"configuration\":{\"start_session_replay_recording_manually\":false},\"browser_sdk_version\":\"6.0.0\"},\"device\":{\"type\":\"$device_type\"},\"os\":{\"name\":\"$os_name\"},\"browser\":{\"name\":\"$browser_name\"},\"connectivity\":{\"status\":\"connected\"}}"

    # Send to Datadog RUM intake - use gzip compression like the real SDK
    local request_id=$(generate_dd_id)
    echo "$payload" | gzip | curl -s -X POST "https://browser-intake-${DD_RUM_SITE}/api/v2/rum?ddsource=browser&dd-api-key=${DD_RUM_CLIENT_TOKEN}&dd-evp-origin=browser&dd-evp-origin-version=6.0.0&dd-request-id=${request_id}&batch_time=${now_ms}&_dd.api=fetch" \
        -H "Content-Type: text/plain;charset=UTF-8" \
        -H "Content-Encoding: gzip" \
        -H "Origin: $FRONTEND_URL" \
        -H "User-Agent: $user_agent" \
        --data-binary @- >/dev/null 2>&1 &

    log "    [DD] Sent $event_type to Datadog RUM (session: ${dd_session_id:0:8}...)"
}

# Send a RUM view event to Datadog
send_dd_view() {
    local visitor_uuid="$1"
    local session_id="$2"
    local user_agent="$3"
    local page_url="$4"
    local page_title="$5"
    local view_id="$6"
    local loading_time=$((RANDOM % 2000 + 500))
    local time_spent=$((RANDOM % 30000 + 5000))

    local view_data=",\"name\":\"$page_title\",\"loading_time\":$loading_time,\"time_spent\":$time_spent,\"is_active\":false,\"action\":{\"count\":$((RANDOM % 5))},\"resource\":{\"count\":$((RANDOM % 20 + 5))},\"error\":{\"count\":0},\"long_task\":{\"count\":0},\"frustration\":{\"count\":0}"

    send_to_datadog_rum "view" "$visitor_uuid" "$session_id" "$user_agent" "$page_url" "$view_id" "$view_data"
}

# Send a RUM action event to Datadog
send_dd_action() {
    local visitor_uuid="$1"
    local session_id="$2"
    local user_agent="$3"
    local page_url="$4"
    local action_name="$5"
    local view_id="$6"
    local action_id=$(generate_dd_id)

    # For actions, we need to include action data in the view section
    local view_data="},\"action\":{\"id\":\"$action_id\",\"type\":\"custom\",\"target\":{\"name\":\"$action_name\"},\"loading_time\":$((RANDOM % 500 + 100))"

    send_to_datadog_rum "action" "$visitor_uuid" "$session_id" "$user_agent" "$page_url" "$view_id" "$view_data"
}

# Send a RUM error event to Datadog
send_dd_error() {
    local visitor_uuid="$1"
    local session_id="$2"
    local user_agent="$3"
    local page_url="$4"
    local error_msg="$5"
    local view_id="$6"
    local error_id=$(generate_dd_id)

    # Escape the error message for JSON
    local escaped_msg=$(echo "$error_msg" | sed 's/"/\\"/g')

    # For errors, include error data
    local view_data="},\"error\":{\"id\":\"$error_id\",\"message\":\"$escaped_msg\",\"source\":\"source\",\"type\":\"TypeError\",\"handling\":\"unhandled\",\"stack\":\"at anonymous (script.js:$((RANDOM % 500)):$((RANDOM % 50)))\"}"

    send_to_datadog_rum "error" "$visitor_uuid" "$session_id" "$user_agent" "$page_url" "$view_id" "$view_data"
}

log_error() {
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo "[$timestamp] ERROR: $*" >> "$LOG_FILE"
    echo -e "${RED}[$timestamp] ERROR: $*${NC}"
}

# Initialize visitor pool file if it doesn't exist
init_pool() {
    if [ ! -f "$POOL_FILE" ]; then
        touch "$POOL_FILE"
        log "Created new visitor pool file: $POOL_FILE"
    fi
}

# Get a random UUID from the pool (for returning users)
get_pool_uuid() {
    if [ ! -s "$POOL_FILE" ]; then
        echo ""
        return
    fi
    local count=$(wc -l < "$POOL_FILE")
    if [ "$count" -eq 0 ]; then
        echo ""
        return
    fi
    local line=$((RANDOM % count + 1))
    sed -n "${line}p" "$POOL_FILE"
}

# Add a UUID to the pool (for new users)
add_to_pool() {
    local uuid="$1"
    if [ -z "$uuid" ]; then
        return
    fi

    # Check if UUID already exists in pool
    if grep -q "^${uuid}$" "$POOL_FILE" 2>/dev/null; then
        return
    fi

    echo "$uuid" >> "$POOL_FILE"
    log "Added new visitor UUID to pool: $uuid"

    # Trim pool if it exceeds max size (remove oldest entries)
    local count=$(wc -l < "$POOL_FILE")
    if [ "$count" -gt "$MAX_POOL_SIZE" ]; then
        local to_remove=$((count - MAX_POOL_SIZE))
        tail -n "+$((to_remove + 1))" "$POOL_FILE" > "${POOL_FILE}.tmp"
        mv "${POOL_FILE}.tmp" "$POOL_FILE"
        log "Trimmed pool to $MAX_POOL_SIZE entries (removed $to_remove oldest)"
    fi
}

# Get a random user agent
get_random_user_agent() {
    local idx=$((RANDOM % ${#USER_AGENTS[@]}))
    echo "${USER_AGENTS[$idx]}"
}

# Get a random referrer
get_random_referrer() {
    local idx=$((RANDOM % ${#REFERRERS[@]}))
    echo "${REFERRERS[$idx]}"
}

# Get a random page URL
get_random_page() {
    local idx=$((RANDOM % ${#PAGE_URLS[@]}))
    echo "${PAGE_URLS[$idx]}"
}

# Get a random action type
get_random_action() {
    local idx=$((RANDOM % ${#ACTION_TYPES[@]}))
    echo "${ACTION_TYPES[$idx]}"
}

# Simulate a single user visit
simulate_user_visit() {
    local user_agent=$(get_random_user_agent)
    local referrer=$(get_random_referrer)
    local visitor_uuid=""
    local is_new_user=false

    # Determine if this is a new or returning user
    if [ $((RANDOM % 100)) -lt "$NEW_USER_RATE" ]; then
        is_new_user=true
        visitor_uuid=""  # Backend will assign new UUID
        log "Simulating NEW user visit"
    else
        visitor_uuid=$(get_pool_uuid)
        if [ -z "$visitor_uuid" ]; then
            # Pool is empty, force new user
            is_new_user=true
            log "Pool empty, simulating NEW user visit"
        else
            log "Simulating RETURNING user visit (UUID: ${visitor_uuid:0:8}...)"
        fi
    fi

    # Step 1: Fetch frontend page (like a real browser would)
    log "  Fetching frontend page: $FRONTEND_URL/"
    curl -s -o /dev/null -w "" \
        -H "User-Agent: $user_agent" \
        -H "Referer: $referrer" \
        "$FRONTEND_URL/" 2>/dev/null || true

    # Small delay to simulate page load
    sleep 0.$((RANDOM % 3 + 1))

    # Step 2: Initialize RUM session with backend
    log "  Initializing RUM session..."
    local init_payload
    if [ -z "$visitor_uuid" ]; then
        init_payload=$(cat <<EOF
{
    "user_agent": "$user_agent",
    "referrer": "$referrer",
    "page_url": "$FRONTEND_URL/"
}
EOF
)
    else
        init_payload=$(cat <<EOF
{
    "existing_uuid": "$visitor_uuid",
    "user_agent": "$user_agent",
    "referrer": "$referrer",
    "page_url": "$FRONTEND_URL/"
}
EOF
)
    fi

    local init_response
    init_response=$(curl -s -X POST "$BACKEND_URL/v1/rum/init" \
        -H "Content-Type: application/json" \
        -H "User-Agent: $user_agent" \
        -d "$init_payload" 2>/dev/null)

    if [ -z "$init_response" ]; then
        log_error "Failed to initialize RUM session"
        return 1
    fi

    # Parse response to get visitor_uuid and session_id
    visitor_uuid=$(echo "$init_response" | grep -o '"visitor_uuid":"[^"]*"' | cut -d'"' -f4)
    local session_id=$(echo "$init_response" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)
    local is_new=$(echo "$init_response" | grep -o '"is_new":[^,}]*' | cut -d':' -f2)

    if [ -z "$visitor_uuid" ] || [ -z "$session_id" ]; then
        log_error "Invalid response from /v1/rum/init: $init_response"
        return 1
    fi

    # Extract APM trace context for RUM-APM correlation
    local trace_id=$(echo "$init_response" | grep -o '"trace_id":"[^"]*"' | cut -d'"' -f4)
    local span_id=$(echo "$init_response" | grep -o '"span_id":"[^"]*"' | cut -d'"' -f4)

    log "  Got visitor_uuid: ${visitor_uuid:0:8}..., session_id: ${session_id:0:8}..., is_new: $is_new"
    if [ -n "$trace_id" ]; then
        log "  APM trace context: trace_id=${trace_id:0:12}..., span_id=${span_id:0:12}..."
    fi

    # If this is a new user, add to pool
    if [ "$is_new" = "true" ]; then
        add_to_pool "$visitor_uuid"
    fi

    # Record session start time
    local session_start=$(($(date +%s) * 1000))

    # Generate a view_id for Datadog correlation (reused across page views in this visit)
    local view_id=$(generate_dd_id)

    # Step 3: Track initial page view (pass trace context for correlation)
    log "  Tracking page view: /"
    track_event "$visitor_uuid" "$session_id" "view" "/" "n0ko" "$user_agent" "$trace_id" "$span_id"
    send_dd_view "$visitor_uuid" "$session_id" "$user_agent" "$FRONTEND_URL/" "n0ko" "$view_id"

    # Small delay
    sleep 0.$((RANDOM % 5 + 2))

    # Step 4: Simulate random navigation (1-4 page views)
    local num_pages=$((RANDOM % 4 + 1))
    for ((i=0; i<num_pages; i++)); do
        local page=$(get_random_page)
        view_id=$(generate_dd_id)  # New view_id for each page
        log "  Tracking page view: $page"
        track_event "$visitor_uuid" "$session_id" "view" "$page" "n0ko" "$user_agent" "$trace_id" "$span_id"
        send_dd_view "$visitor_uuid" "$session_id" "$user_agent" "$FRONTEND_URL$page" "n0ko" "$view_id"
        sleep 0.$((RANDOM % 8 + 3))

        # Sometimes simulate actions on the page
        if [ $((RANDOM % 3)) -eq 0 ]; then
            local action=$(get_random_action)
            log "  Tracking action: $action"
            track_action "$visitor_uuid" "$session_id" "$action" "$page" "$user_agent" "$trace_id" "$span_id"
            send_dd_action "$visitor_uuid" "$session_id" "$user_agent" "$FRONTEND_URL$page" "$action" "$view_id"
            sleep 0.$((RANDOM % 4 + 1))
        fi
    done

    # Step 5: Occasionally simulate errors (5% of sessions)
    if [ $((RANDOM % 100)) -lt 5 ]; then
        log "  Simulating JavaScript error"
        local errors=(
            "TypeError: Cannot read property 'map' of undefined"
            "ReferenceError: myFunction is not defined"
            "NetworkError: Failed to fetch"
        )
        local error_msg="${errors[$((RANDOM % ${#errors[@]}))]}"
        track_error "$visitor_uuid" "$session_id" "$user_agent" "$trace_id" "$span_id"
        send_dd_error "$visitor_uuid" "$session_id" "$user_agent" "$FRONTEND_URL/" "$error_msg" "$view_id"
    fi

    # Step 6: End session with realistic duration
    local session_end=$(($(date +%s) * 1000))
    local duration=$((session_end - session_start))
    # Add some randomness to make it more realistic (30s to 10min)
    duration=$((duration + RANDOM % 540000 + 30000))

    log "  Ending session (duration: ${duration}ms)"
    end_session "$session_id" "$duration" "$user_agent"

    log "  Visit completed for ${visitor_uuid:0:8}..."
}

# Track an event
track_event() {
    local visitor_uuid="$1"
    local session_id="$2"
    local event_type="$3"
    local page_url="$4"
    local page_title="$5"
    local user_agent="$6"
    local trace_id="$7"
    local span_id="$8"

    # Build payload with optional trace context for RUM-APM correlation
    local trace_fields=""
    if [ -n "$trace_id" ]; then
        trace_fields=",\"trace_id\": \"$trace_id\""
    fi
    if [ -n "$span_id" ]; then
        trace_fields="$trace_fields,\"span_id\": \"$span_id\""
    fi

    local payload=$(cat <<EOF
{
    "visitor_uuid": "$visitor_uuid",
    "session_id": "$session_id",
    "event_type": "$event_type",
    "page_url": "$FRONTEND_URL$page_url",
    "page_title": "$page_title",
    "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"$trace_fields
}
EOF
)

    curl -s -X POST "$BACKEND_URL/v1/rum/track" \
        -H "Content-Type: application/json" \
        -H "User-Agent: $user_agent" \
        -d "$payload" >/dev/null 2>&1 || true
}

# Track an action
track_action() {
    local visitor_uuid="$1"
    local session_id="$2"
    local action_name="$3"
    local page_url="$4"
    local user_agent="$5"
    local trace_id="$6"
    local span_id="$7"

    # Build payload with optional trace context for RUM-APM correlation
    local trace_fields=""
    if [ -n "$trace_id" ]; then
        trace_fields=",\"trace_id\": \"$trace_id\""
    fi
    if [ -n "$span_id" ]; then
        trace_fields="$trace_fields,\"span_id\": \"$span_id\""
    fi

    local payload=$(cat <<EOF
{
    "visitor_uuid": "$visitor_uuid",
    "session_id": "$session_id",
    "event_type": "action",
    "page_url": "$FRONTEND_URL$page_url",
    "metadata": {
        "action_name": "$action_name",
        "element_id": "element-$((RANDOM % 100))"
    },
    "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"$trace_fields
}
EOF
)

    curl -s -X POST "$BACKEND_URL/v1/rum/track" \
        -H "Content-Type: application/json" \
        -H "User-Agent: $user_agent" \
        -d "$payload" >/dev/null 2>&1 || true
}

# Track an error
track_error() {
    local visitor_uuid="$1"
    local session_id="$2"
    local user_agent="$3"
    local trace_id="$4"
    local span_id="$5"

    local errors=(
        "TypeError: Cannot read property 'map' of undefined"
        "ReferenceError: myFunction is not defined"
        "NetworkError: Failed to fetch"
        "SyntaxError: Unexpected token"
    )
    local error_idx=$((RANDOM % ${#errors[@]}))
    local error_msg="${errors[$error_idx]}"

    # Build payload with optional trace context for RUM-APM correlation
    local trace_fields=""
    if [ -n "$trace_id" ]; then
        trace_fields=",\"trace_id\": \"$trace_id\""
    fi
    if [ -n "$span_id" ]; then
        trace_fields="$trace_fields,\"span_id\": \"$span_id\""
    fi

    local payload=$(cat <<EOF
{
    "visitor_uuid": "$visitor_uuid",
    "session_id": "$session_id",
    "event_type": "error",
    "page_url": "$FRONTEND_URL/",
    "metadata": {
        "error_message": "$error_msg",
        "error_stack": "at anonymous (script.js:$((RANDOM % 500 + 1)):$((RANDOM % 50 + 1)))"
    },
    "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"$trace_fields
}
EOF
)

    curl -s -X POST "$BACKEND_URL/v1/rum/track" \
        -H "Content-Type: application/json" \
        -H "User-Agent: $user_agent" \
        -d "$payload" >/dev/null 2>&1 || true
}

# End a session
end_session() {
    local session_id="$1"
    local duration="$2"
    local user_agent="$3"

    local payload=$(cat <<EOF
{
    "session_id": "$session_id",
    "duration_ms": $duration,
    "exit_page": "$FRONTEND_URL/"
}
EOF
)

    curl -s -X POST "$BACKEND_URL/v1/rum/session/end" \
        -H "Content-Type: application/json" \
        -H "User-Agent: $user_agent" \
        -d "$payload" >/dev/null 2>&1 || true
}

# Main traffic generation loop
generate_traffic() {
    log "Starting frontend traffic generation"
    log "  Frontend URL: $FRONTEND_URL"
    log "  Backend URL: $BACKEND_URL"
    log "  New user rate: ${NEW_USER_RATE}%"
    log "  Pool file: $POOL_FILE"
    log "  Datadog RUM: $SEND_TO_DATADOG"

    init_pool

    while true; do
        simulate_user_visit

        # Wait between visits (2-8 seconds)
        local wait_time=$((RANDOM % 7 + 2))
        log "Waiting ${wait_time}s before next visit..."
        sleep $wait_time
    done
}

start_traffic() {
    if [ -f "$PID_FILE" ]; then
        local OLD_PID=$(cat "$PID_FILE")
        if kill -0 "$OLD_PID" 2>/dev/null; then
            echo -e "${YELLOW}Frontend traffic generator is already running (PID: $OLD_PID)${NC}"
            echo "Use './frontend-traffic-generator.sh stop' to stop it first"
            exit 1
        else
            rm -f "$PID_FILE"
        fi
    fi

    # Test connections first
    echo -e "${YELLOW}Testing connection to frontend ($FRONTEND_URL)...${NC}"
    if ! curl -s --connect-timeout 5 "$FRONTEND_URL/" >/dev/null 2>&1; then
        echo -e "${RED}Warning: Cannot connect to $FRONTEND_URL${NC}"
        echo "Frontend may not be running - traffic will still be generated to backend"
    else
        echo -e "${GREEN}Frontend connection successful!${NC}"
    fi

    echo -e "${YELLOW}Testing connection to backend ($BACKEND_URL)...${NC}"
    if ! curl -s --connect-timeout 5 "$BACKEND_URL/health" >/dev/null 2>&1; then
        echo -e "${RED}Error: Cannot connect to $BACKEND_URL/health${NC}"
        echo "Make sure the Rayne backend is running"
        exit 1
    fi
    echo -e "${GREEN}Backend connection successful!${NC}"

    # Clear old log
    >"$LOG_FILE"

    # Start traffic generator in background
    echo -e "${GREEN}Starting frontend traffic generator...${NC}"
    generate_traffic &
    local PID=$!
    echo $PID > "$PID_FILE"

    echo -e "${GREEN}Frontend traffic generator started (PID: $PID)${NC}"
    echo ""
    echo "Configuration:"
    echo "  Frontend URL: $FRONTEND_URL"
    echo "  Backend URL:  $BACKEND_URL"
    echo "  New user rate: ${NEW_USER_RATE}%"
    echo "  Visitor pool: $POOL_FILE"
    if [ "$SEND_TO_DATADOG" = "true" ]; then
        echo -e "  ${GREEN}Datadog RUM: ENABLED${NC} (sessions will appear in Datadog console)"
        echo "    Application ID: ${DD_RUM_APPLICATION_ID:0:8}..."
        echo "    Environment: $DD_RUM_ENV"
    else
        echo "  Datadog RUM: disabled (use -d to enable)"
    fi
    echo ""
    echo "Monitor with:"
    echo "  tail -f $LOG_FILE"
    echo ""
    echo "Stop with:"
    echo "  ./frontend-traffic-generator.sh stop"
}

stop_traffic() {
    if [ ! -f "$PID_FILE" ]; then
        echo -e "${YELLOW}Frontend traffic generator is not running${NC}"
        exit 0
    fi

    local PID=$(cat "$PID_FILE")

    if kill -0 "$PID" 2>/dev/null; then
        echo -e "${YELLOW}Stopping frontend traffic generator (PID: $PID)...${NC}"

        # Kill the main process and all child processes
        pkill -P "$PID" 2>/dev/null || true
        kill "$PID" 2>/dev/null || true

        # Wait a moment and force kill if needed
        sleep 1
        if kill -0 "$PID" 2>/dev/null; then
            kill -9 "$PID" 2>/dev/null || true
        fi

        rm -f "$PID_FILE"
        echo -e "${GREEN}Frontend traffic generator stopped${NC}"
    else
        echo -e "${YELLOW}Process $PID is not running, cleaning up...${NC}"
        rm -f "$PID_FILE"
    fi
}

status_traffic() {
    if [ ! -f "$PID_FILE" ]; then
        echo -e "${YELLOW}Frontend traffic generator is not running${NC}"
        exit 0
    fi

    local PID=$(cat "$PID_FILE")

    if kill -0 "$PID" 2>/dev/null; then
        echo -e "${GREEN}Frontend traffic generator is running (PID: $PID)${NC}"
        echo ""
        echo "Configuration:"
        echo "  Frontend URL: $FRONTEND_URL"
        echo "  Backend URL:  $BACKEND_URL"
        echo "  New user rate: ${NEW_USER_RATE}%"
        echo ""
        if [ -f "$POOL_FILE" ]; then
            local pool_count=$(wc -l < "$POOL_FILE")
            echo "Visitor pool: $pool_count UUIDs stored"
        fi
        echo ""
        echo "Recent activity:"
        tail -5 "$LOG_FILE" 2>/dev/null || echo "  No log entries yet"
    else
        echo -e "${YELLOW}Frontend traffic generator is not running (stale PID file)${NC}"
        rm -f "$PID_FILE"
    fi
}

show_help() {
    cat << 'EOF'
Frontend Traffic Generator for Rayne RUM

Generates realistic frontend traffic with proper RUM integration.
25% of visitors are new users (get UUIDs from backend), 75% are returning.

USAGE:
    ./frontend-traffic-generator.sh [OPTIONS] COMMAND

COMMANDS:
    start           Start the traffic generator
    stop            Stop the traffic generator
    status          Check if generator is running
    help            Show this help message

OPTIONS:
    -n, --new-rate PERCENT   Percentage of new users (default: 25)
    -f, --frontend URL       Frontend server URL (default: http://localhost:3000)
    -b, --backend URL        Backend API URL (default: http://localhost:8080)
    -d, --datadog            Send events to Datadog RUM (shows in Datadog console)
    -v, --verbose            Enable verbose logging
    -h, --help               Show this help message

EXAMPLES:
    # Start with defaults (25% new users, backend only)
    ./frontend-traffic-generator.sh start

    # Start with Datadog RUM integration (shows in Datadog console!)
    ./frontend-traffic-generator.sh -d start

    # Start with 40% new users and Datadog
    ./frontend-traffic-generator.sh -d -n 40 start

    # Start with custom URLs
    ./frontend-traffic-generator.sh -f http://frontend:3000 -b http://api:8080 start

    # Use environment variables
    SEND_TO_DATADOG=true ./frontend-traffic-generator.sh start

ENVIRONMENT VARIABLES:
    NEW_USER_RATE         Percentage of new users (default: 25)
    FRONTEND_URL          Frontend server URL (default: http://localhost:3000)
    BACKEND_URL           Backend API URL (default: http://localhost:8080)
    SEND_TO_DATADOG       Send events to Datadog RUM (default: false)
    DD_RUM_APPLICATION_ID Datadog RUM application ID
    DD_RUM_CLIENT_TOKEN   Datadog RUM client token
    DD_RUM_ENV            Datadog environment (default: staging)

HOW IT WORKS:
    New Users (25%):
        - No existing UUID sent to backend
        - Backend generates new UUID via /v1/rum/init
        - Response: {"is_new": true, "visitor_uuid": "<new-uuid>"}
        - UUID added to local pool for future "returning" visits

    Returning Users (75%):
        - Existing UUID from pool sent to backend
        - Backend recognizes UUID, creates new session
        - Response: {"is_new": false, "visitor_uuid": "<same-uuid>"}

INTEGRATING YOUR OWN SITE WITH RAYNE RUM:
    Any website can use Rayne for server-side visitor UUID management.

    Quick Setup:
        1. Add to your HTML (before </body>):
           <script>window.RAYNE_API_BASE = 'http://your-rayne-server:8080';</script>
           <script src="http://your-rayne-server:8080/static/js/datadog-rum-init.js"></script>

        2. Or implement manually:
           - POST /v1/rum/init     -> Get visitor UUID (is_new: true/false)
           - POST /v1/rum/track    -> Track page views and events
           - POST /v1/rum/session/end -> End session on page unload

    Response from /v1/rum/init:
        {
            "visitor_uuid": "uuid-here",
            "session_id": "session-uuid",
            "is_new": true,         <- true = first-time visitor
            "message": "Welcome, new visitor!",
            "trace_id": "12345...", <- APM trace ID for correlation
            "span_id": "67890..."   <- APM span ID for correlation
        }

APM TRACE INJECTION:
    The traffic generator captures APM trace context from /v1/rum/init responses
    and propagates trace_id/span_id to all subsequent /v1/rum/track calls.
    This allows you to correlate RUM sessions with backend APM traces in Datadog.

    In Datadog APM:
      - Filter by @usr.id to see traces for a specific visitor
      - Filter by session_id to see traces for a specific session
      - Use trace_id to jump from RUM event to APM trace

    See README.md for full integration guide.
EOF
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -n|--new-rate)
                NEW_USER_RATE="$2"
                shift 2
                ;;
            -f|--frontend)
                FRONTEND_URL="$2"
                shift 2
                ;;
            -b|--backend)
                BACKEND_URL="$2"
                shift 2
                ;;
            -d|--datadog)
                SEND_TO_DATADOG="true"
                shift
                ;;
            -v|--verbose)
                VERBOSE="true"
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            start|stop|status|help)
                COMMAND="$1"
                shift
                ;;
            *)
                if [ -z "$COMMAND" ]; then
                    echo -e "${RED}Unknown option: $1${NC}"
                    echo ""
                    show_help
                    exit 1
                fi
                shift
                ;;
        esac
    done
}

# Main
COMMAND=""
parse_args "$@"

case "${COMMAND:-help}" in
    start)
        start_traffic
        ;;
    stop)
        stop_traffic
        ;;
    status)
        status_traffic
        ;;
    help)
        show_help
        ;;
    *)
        echo -e "${RED}Unknown command: $COMMAND${NC}"
        echo ""
        show_help
        exit 1
        ;;
esac
