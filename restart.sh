#!/usr/bin/env bash
# restart-hivemind.sh — graceful rebuild + restart
set -e

# --- Graceful shutdown ---
PIDS=$(pgrep -f hivemind-gw 2>/dev/null || true)
if [ -n "$PIDS" ]; then
    echo "Sending SIGTERM to hivemind-gw (PIDs: $PIDS)..."
    echo "$PIDS" | xargs sudo kill 2>/dev/null || true
    # Wait up to 5s for graceful exit
    for _ in 1 2 3 4 5; do
        if ! pgrep -f hivemind-gw >/dev/null 2>&1; then
            echo "Process exited gracefully."
            break
        fi
        sleep 1
    done
    # Force kill if still running
    if pgrep -f hivemind-gw >/dev/null 2>&1; then
        echo "Force killing..."
        pkill -9 -f hivemind-gw 2>/dev/null || true
        sleep 1
    fi
else
    echo "No running hivemind-gw process found."
fi

# --- Secrets from Vaultwarden (with fallback to static secrets file) ---
export BW_SESSION=$(cat ~/.bw-session 2>/dev/null)
# bw CLI fixed 2026-06-28: /etc/hosts override removed (was routing to unreachable Headscale IP).
# Static cache remains as defense-in-depth if bw session expires between restarts.
_zai_from_bw=$(timeout 8 bw get notes "ZAI API Key" </dev/null 2>/dev/null | grep "^ZAI_API_KEY=" | cut -d= -f2-)
if [[ -n "$_zai_from_bw" ]]; then
  export ZAI_API_KEY="$_zai_from_bw"
elif [[ -f /opt/claude-glm/secrets ]]; then
  export ZAI_API_KEY=$(grep '^ZAI_API_KEY=' /opt/claude-glm/secrets | cut -d= -f2-)
fi
unset _zai_from_bw

# Auth token for HiveMind API auth middleware
_hivemind_from_bw=$(timeout 8 bw get notes "HiveMind Auth Token" </dev/null 2>/dev/null | grep "^HIVEMIND_AUTH_TOKEN=" | cut -d= -f2-)
if [[ -n "$_hivemind_from_bw" ]]; then
  export HIVEMIND_AUTH_TOKEN="$_hivemind_from_bw"
elif [[ -f /opt/claude-glm/secrets ]]; then
  export HIVEMIND_AUTH_TOKEN=$(grep '^HIVEMIND_AUTH_TOKEN=' /opt/claude-glm/secrets | cut -d= -f2-)
fi
unset _hivemind_from_bw
export HIVEMIND_REQUIRE_AUTH=true

echo "Secrets loaded: ZAI_API_KEY=$([ -n "$ZAI_API_KEY" ] && echo 'OK' || echo 'MISSING') HIVEMIND_AUTH=$([ -n "$HIVEMIND_AUTH_TOKEN" ] && echo 'OK' || echo 'MISSING')"

# --- Deploy ---
sudo cp /home/pook/ralph/hivemind/hivemind-gw /usr/local/bin/hivemind-gw
echo "Binary copied OK"
# Pass the Vaultwarden-sourced secret via the inherited environment (sudo --preserve-env),
# NOT argv. `env ZAI_API_KEY=val` would expose the key to any local `ps aux`; the inherited
# env lives in /proc/PID/environ, readable only by the process owner and root.
sudo -u pook --preserve-env=ZAI_API_KEY,HIVEMIND_AUTH_TOKEN,HIVEMIND_REQUIRE_AUTH /usr/local/bin/hivemind-gw --config /home/pook/ralph/hivemind/config/batkave.toml &>/tmp/hivemind-gw.log &
sleep 3

# --- Verify ---
echo "=== Models ==="
curl -sf -H "Authorization: Bearer $HIVEMIND_AUTH_TOKEN" http://127.0.0.1:8400/v1/models | python3 -c "import json,sys;[print(m['id']) for m in json.load(sys.stdin)['data']]" 2>/dev/null || echo "ERROR: Gateway not responding (or auth token missing)"

echo "=== Health ==="
curl -sf http://127.0.0.1:8401/health | python3 -m json.tool 2>/dev/null || echo "WARN: Admin health not responding (may be localhost-only now)"
