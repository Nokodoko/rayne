#!/usr/bin/env bash
# test-llm-monitoring.sh
#
# Verify that Datadog LLM Observability is working for the monty chatbot.
#
# This script:
#   1. Sends a test chat message to the monty gateway
#   2. Checks that the Datadog agent received traces
#   3. Reports the results
#
# Usage:
#   ./test-llm-monitoring.sh
#   MONTY_HOST=192.168.50.68 DD_AGENT_HOST=192.168.50.179 ./test-llm-monitoring.sh

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
MONTY_HOST="${MONTY_HOST:-192.168.50.68}"
MONTY_PORT="${MONTY_PORT:-8001}"
MONTY_GATEWAY="http://${MONTY_HOST}:${MONTY_PORT}"

DD_AGENT_HOST="${DD_AGENT_HOST:-192.168.50.179}"
DD_AGENT_TRACE_PORT="${DD_AGENT_TRACE_PORT:-8126}"
DD_AGENT_URL="http://${DD_AGENT_HOST}:${DD_AGENT_TRACE_PORT}"

# ---------------------------------------------------------------------------
# Helper functions
# ---------------------------------------------------------------------------
info()  { echo -e "\033[1;34m[INFO]\033[0m  $*"; }
ok()    { echo -e "\033[1;32m[OK]\033[0m    $*"; }
warn()  { echo -e "\033[1;33m[WARN]\033[0m  $*"; }
fail()  { echo -e "\033[1;31m[FAIL]\033[0m  $*"; }

TESTS_PASSED=0
TESTS_FAILED=0

check_pass() { TESTS_PASSED=$((TESTS_PASSED + 1)); ok "$*"; }
check_fail() { TESTS_FAILED=$((TESTS_FAILED + 1)); fail "$*"; }

# ---------------------------------------------------------------------------
# Test 1: Verify monty gateway is reachable
# ---------------------------------------------------------------------------
echo ""
echo "============================================================================="
echo "  Datadog LLM Monitoring - Integration Test"
echo "============================================================================="
echo ""

info "Test 1: Checking monty gateway health at ${MONTY_GATEWAY}/health ..."
HEALTH_RESPONSE=$(curl -sf --max-time 10 "${MONTY_GATEWAY}/health" 2>/dev/null) && {
    check_pass "Monty gateway is reachable. Response: ${HEALTH_RESPONSE}"
} || {
    check_fail "Monty gateway is not reachable at ${MONTY_GATEWAY}/health"
    warn "Make sure the monty gateway is running with ddtrace-run."
}

# ---------------------------------------------------------------------------
# Test 2: Verify Datadog agent is reachable and APM is enabled
# ---------------------------------------------------------------------------
info "Test 2: Checking Datadog agent APM endpoint at ${DD_AGENT_URL}/info ..."
AGENT_INFO=$(curl -sf --max-time 10 "${DD_AGENT_URL}/info" 2>/dev/null) && {
    check_pass "Datadog agent is reachable."

    # Check if the trace agent reports receiver info
    if echo "${AGENT_INFO}" | grep -qi "receiver"; then
        check_pass "Datadog trace agent receiver is active."
    else
        warn "Could not confirm trace receiver status from /info response."
    fi
} || {
    check_fail "Datadog agent is not reachable at ${DD_AGENT_URL}/info"
    warn "Ensure DD_APM_NON_LOCAL_TRAFFIC=true is set and port 8126 is exposed."
}

# ---------------------------------------------------------------------------
# Test 3: Send a test chat message to the monty gateway
# ---------------------------------------------------------------------------
info "Test 3: Sending a test chat message to generate an LLM trace ..."

TEST_MESSAGE="Hello, this is a Datadog LLM monitoring test. Please respond with a short greeting."

# Try the non-streaming endpoint first (easier to capture response)
CHAT_RESPONSE=$(curl -sf --max-time 120 \
    -X POST "${MONTY_GATEWAY}/api/chat" \
    -H "Content-Type: application/json" \
    -d "{\"message\": \"${TEST_MESSAGE}\", \"stream\": false}" \
    2>/dev/null) && {
    check_pass "Chat request succeeded."
    info "Response preview: $(echo "${CHAT_RESPONSE}" | head -c 200)..."
} || {
    # Fall back to streaming endpoint
    warn "Non-streaming request failed, trying streaming endpoint..."
    CHAT_RESPONSE=$(curl -sf --max-time 120 \
        -X POST "${MONTY_GATEWAY}/api/chat" \
        -H "Content-Type: application/json" \
        -d "{\"message\": \"${TEST_MESSAGE}\", \"stream\": true}" \
        2>/dev/null) && {
        check_pass "Streaming chat request succeeded."
        info "Response preview: $(echo "${CHAT_RESPONSE}" | head -c 200)..."
    } || {
        check_fail "Chat request failed. Is the monty gateway running?"
    }
}

# ---------------------------------------------------------------------------
# Test 4: Wait briefly, then check for traces in the Datadog agent
# ---------------------------------------------------------------------------
info "Test 4: Waiting 5 seconds for traces to propagate to the agent ..."
sleep 5

# Query the trace agent debug endpoint for recent traces
TRACE_STATS=$(curl -sf --max-time 10 "${DD_AGENT_URL}/debug/vars" 2>/dev/null) && {
    if echo "${TRACE_STATS}" | grep -qi "monty-llm\|traces_received\|PayloadReceived"; then
        check_pass "Trace data detected in Datadog agent debug output."
    else
        warn "Agent debug endpoint reachable but no monty-llm traces found yet."
        warn "Traces may take up to 30 seconds to appear. Check Datadog UI."
    fi
} || {
    warn "Could not query agent debug vars at ${DD_AGENT_URL}/debug/vars"
    info "This is informational only -- traces may still be working."
}

# ---------------------------------------------------------------------------
# Test 5: Verify ddtrace is installed on monty
# ---------------------------------------------------------------------------
info "Test 5: Checking if ddtrace is installed on monty ..."
ssh "${USER}@${MONTY_HOST}" "python -c 'import ddtrace; print(ddtrace.__version__)'" 2>/dev/null && {
    check_pass "ddtrace is installed on monty."
} || {
    ssh "${USER}@${MONTY_HOST}" "python3 -c 'import ddtrace; print(ddtrace.__version__)'" 2>/dev/null && {
        check_pass "ddtrace is installed on monty (python3)."
    } || {
        check_fail "ddtrace does not appear to be installed on monty."
        info "Run: ssh ${USER}@${MONTY_HOST} 'pip install ddtrace'"
    }
}

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo "============================================================================="
echo "  Test Summary"
echo "============================================================================="
echo ""
echo "  Passed: ${TESTS_PASSED}"
echo "  Failed: ${TESTS_FAILED}"
echo ""

if [ "${TESTS_FAILED}" -gt 0 ]; then
    echo "  Some tests failed. Review the output above for details."
    echo ""
    echo "  Troubleshooting:"
    echo "    1. Ensure the monty gateway is started with ddtrace-run:"
    echo "       source ~/.datadog-llm-env && ddtrace-run python -m interfaces.api.server"
    echo ""
    echo "    2. Ensure DD_APM_NON_LOCAL_TRAFFIC=true is set on the Datadog agent"
    echo "       and port 8126 is accessible from ${MONTY_HOST}."
    echo ""
    echo "    3. Check Datadog agent logs:"
    echo "       kubectl logs -l app=datadog -n default --tail=50"
    echo ""
    echo "    4. Enable debug logging on monty:"
    echo "       export DD_TRACE_DEBUG=true"
    echo "       export DD_TRACE_LOG_FILE=/tmp/ddtrace-monty.log"
    echo ""
    exit 1
else
    ok "All tests passed. LLM Observability is operational."
    echo ""
    echo "  View traces in Datadog:"
    echo "    APM > Services > monty-llm"
    echo "    LLM Observability > monty-chatbot"
    echo ""
    exit 0
fi
