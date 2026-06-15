#!/usr/bin/env bash
# Hivemind Gateway — End-to-end smoke test against live BatKave
# Usage: ./scripts/smoke-test.sh [config_path]
# Exit 0 = all pass, 1 = failures

set -euo pipefail

CONFIG="${1:-config/batkave.toml}"
PROXY_PORT=8400
ADMIN_PORT=8401
METRICS_PORT=9090
PROXY_BASE="http://localhost:${PROXY_PORT}"
ADMIN_BASE="http://localhost:${ADMIN_PORT}"
METRICS_BASE="http://localhost:${METRICS_PORT}"

PASS=0
FAIL=0
FAILURES=()

# ── Helpers ───────────────────────────────────────────────────────────────────

pass() { echo "  [PASS] $1"; ((PASS++)); }
fail() { echo "  [FAIL] $1"; FAILURES+=("$1"); ((FAIL++)); }

check_status() {
  local label="$1" expected="$2" actual="$3"
  if [[ "$actual" == "$expected" ]]; then
    pass "$label (HTTP $actual)"
  else
    fail "$label — expected HTTP $expected, got $actual"
  fi
}

http_status() {
  curl -s -o /dev/null -w "%{http_code}" --max-time 10 "$@"
}

http_body() {
  curl -s --max-time 15 "$@"
}

# ── Pre-flight ─────────────────────────────────────────────────────────────────

echo ""
echo "=== Hivemind Gateway Smoke Test ==="
echo "  Config : $CONFIG"
echo "  Proxy  : $PROXY_BASE"
echo "  Admin  : $ADMIN_BASE"
echo "  Metrics: $METRICS_BASE"
echo ""

# Verify gateway is reachable before running tests
if ! curl -s --max-time 3 "${PROXY_BASE}/health" >/dev/null 2>&1 && \
   ! curl -s --max-time 3 "${ADMIN_BASE}/health" >/dev/null 2>&1; then
  echo "ERROR: Gateway not reachable. Start with:"
  echo "  ./hivemind-gw --config ${CONFIG}"
  exit 1
fi

# ── Test 1: /health (admin port) ──────────────────────────────────────────────

echo "--- Health ---"
STATUS=$(http_status "${ADMIN_BASE}/health")
check_status "/health returns 200" "200" "$STATUS"

BODY=$(http_body "${ADMIN_BASE}/health")
if echo "$BODY" | grep -qi '"status"'; then
  pass "/health response has status field"
else
  fail "/health response missing status field — got: ${BODY:0:200}"
fi

# ── Test 2: /metrics (metrics port) ──────────────────────────────────────────

echo ""
echo "--- Metrics ---"
STATUS=$(http_status "${METRICS_BASE}/metrics")
check_status "/metrics returns 200" "200" "$STATUS"

BODY=$(http_body "${METRICS_BASE}/metrics")
if echo "$BODY" | grep -q "hivemind_"; then
  pass "/metrics contains hivemind_ metrics"
else
  fail "/metrics missing hivemind_ prefix — prometheus scrape may be broken"
fi

# ── Test 3: /admin/usage ──────────────────────────────────────────────────────

echo ""
echo "--- Usage Tracking ---"
STATUS=$(http_status "${ADMIN_BASE}/admin/usage")
check_status "/admin/usage returns 200" "200" "$STATUS"

BODY=$(http_body "${ADMIN_BASE}/admin/usage")
if echo "$BODY" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null; then
  pass "/admin/usage returns valid JSON"
else
  fail "/admin/usage returned invalid JSON — got: ${BODY:0:200}"
fi

# ── Test 4: /v1/models ───────────────────────────────────────────────────────

echo ""
echo "--- Model Listing ---"
STATUS=$(http_status \
  -H "X-HiveMind-Consumer: smoke-test" \
  "${PROXY_BASE}/v1/models")
check_status "/v1/models returns 200" "200" "$STATUS"

# ── Test 5: /v1/chat/completions (Qwen model) ────────────────────────────────

echo ""
echo "--- Chat Completions ---"
CHAT_BODY='{
  "model": "qwen2.5:0.5b",
  "messages": [{"role": "user", "content": "Reply with one word: pong"}],
  "max_tokens": 10
}'

RESP=$(curl -s --max-time 60 \
  -X POST "${PROXY_BASE}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "X-HiveMind-Consumer: smoke-test" \
  -d "$CHAT_BODY" 2>&1)

if echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'choices' in d" 2>/dev/null; then
  pass "/v1/chat/completions returned choices"
else
  # Check if it's a backend-unavailable error (not a gateway bug)
  if echo "$RESP" | grep -qi '"error"'; then
    fail "/v1/chat/completions returned error — backends may be down: ${RESP:0:300}"
  else
    fail "/v1/chat/completions missing choices — got: ${RESP:0:300}"
  fi
fi

# ── Test 6: PII scan — fake SSN should be blocked or flagged ─────────────────

echo ""
echo "--- PII Shield ---"
PII_BODY='{
  "model": "qwen2.5:0.5b",
  "messages": [{"role": "user", "content": "My SSN is 123-45-6789. What is it?"}],
  "max_tokens": 10
}'

PII_STATUS=$(http_status \
  -X POST "${PROXY_BASE}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "X-HiveMind-Consumer: smoke-test" \
  -d "$PII_BODY")

# PII shield should reject (400/422/451) or the gateway proceeds (200) with PII stripped.
# Either is acceptable — what's NOT acceptable is a 5xx gateway error.
if [[ "$PII_STATUS" =~ ^(200|400|422|451)$ ]]; then
  pass "PII scan: gateway handled SSN-containing request (HTTP $PII_STATUS — no 5xx)"
elif [[ "$PII_STATUS" == "503" ]]; then
  # PII shield down with bypass_on_failure=false → 503 is expected behavior
  pass "PII scan: shield offline, gateway returned 503 (bypass_on_failure=false)"
else
  fail "PII scan: unexpected HTTP $PII_STATUS for SSN-containing request"
fi

# ── Test 7: Rate limiting ─────────────────────────────────────────────────────

echo ""
echo "--- Rate Limiting ---"
# ralph-swarm: 60 rpm, burst=20 — hammer 25 requests rapid-fire to trigger limit
RL_BODY='{"model":"qwen2.5:0.5b","messages":[{"role":"user","content":"x"}],"max_tokens":1}'
GOT_429=false

for i in $(seq 1 25); do
  CODE=$(http_status \
    -X POST "${PROXY_BASE}/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -H "X-HiveMind-Consumer: ralph-swarm" \
    -d "$RL_BODY")
  if [[ "$CODE" == "429" ]]; then
    GOT_429=true
    break
  fi
done

if $GOT_429; then
  pass "Rate limit: 429 returned after burst exhausted for ralph-swarm"
else
  # Rate limiter may not trigger if burst allows 25 — check config value
  # burst=20 means 25 requests should exceed it; flag as warning not hard fail
  fail "Rate limit: no 429 after 25 rapid requests for ralph-swarm (burst=20 configured)"
fi

# ── Summary ───────────────────────────────────────────────────────────────────

echo ""
echo "=== Results: ${PASS} passed, ${FAIL} failed ==="

if [[ ${#FAILURES[@]} -gt 0 ]]; then
  echo ""
  echo "Failed tests:"
  for f in "${FAILURES[@]}"; do
    echo "  - $f"
  done
  echo ""
  exit 1
fi

echo ""
exit 0
