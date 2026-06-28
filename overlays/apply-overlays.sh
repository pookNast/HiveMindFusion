#!/usr/bin/env bash
# apply-overlays.sh — materialize HiveMind model overlays into consumer-tagged
# system-prompt files that homelab agents source as their literal `system` message.
#
# Why a build step (not a gateway admin call): HiveMind has no system-prompt
# injection endpoint. Consumer identity travels via the `X-HiveMind-Consumer`
# HTTP header (see internal/gateway/consumer.go:Identify). The overlay is
# therefore a client-side contract: each agent prepends its dist/<tag>.txt to
# the messages array AND sets its consumer header. This script keeps the two
# in sync and validates them against the live config.
#
# Run: on boot, on overlay change, or after `hivemind-gw` reload.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIST="$HERE/dist"
GATEWAY="${HIVEMIND_URL:-http://127.0.0.1:8400}"
CONFIG="${HIVEMIND_CONFIG:-/home/pook/ralph/hivemind/config/batkave.toml}"

# overlay file → consumer tag (must match [rag|rate_limit].consumers.<tag> in config)
declare -A MAP=(
  [ceo.md]="ceo-agent"
  [cfo.md]="cfo-agent"
  [cto.md]="cto-agent"
  [rosie.md]="rosie"
  [super-worker.md]="super-worker"
  [loop-maxxer.md]="loop-maxxer"
  [siteops.md]="siteops"
)

mkdir -p "$DIST"
ok=0; fail=0

log() { printf '[apply-overlays] %s\n' "$*" >&2; }

gateway_up() {
  curl -fsS --max-time 3 "$GATEWAY/v1/models" >/dev/null 2>&1
}

# Verify a consumer tag is known to the gateway config (rate_limit or rag section).
consumer_known() {
  local tag="$1"
  grep -qE "consumers\.$tag\b|api_keys\..*= \"$tag\"" "$CONFIG" 2>/dev/null
}

materialize() {
  local src="$HERE/$1" tag="$2" out="$DIST/$2.txt"
  if [[ ! -f "$src" ]]; then
    log "WARN  missing overlay: $src"; fail=$((fail+1)); return
  fi
  # ponytail: strip markdown headings of ##/### but keep structure — agents
  # get a compact prompt; the .md is the human source of truth.
  # Upgrade: render to a single paragraph if a model chokes on headings.
  sed -e 's/^###[[:space:]]*//' -e 's/^##[[:space:]]*//' "$src" > "$out"
  printf '\n' >> "$out"

  if gateway_up; then
    if consumer_known "$tag"; then
      log "OK    $1 → $tag  ($(wc -l < "$out") lines, gateway up, tag known)"
      ok=$((ok+1))
    else
      log "WARN  $1 → $tag: gateway up but tag NOT in $CONFIG (add [rate_limit.consumers.$tag])"
      fail=$((fail+1))
    fi
  else
    log "OK    $1 → $tag  ($(wc -l < "$out") lines)  [gateway unreachable — file staged]"
    ok=$((ok+1))
  fi
}

log "gateway: $GATEWAY   config: $CONFIG"
gateway_up && log "gateway reachable" || log "gateway UNREACHABLE — staging files only"

for src in "${!MAP[@]}"; do
  materialize "$src" "${MAP[$src]}"
done

# Emit a manifest agents/launchers can iterate.
{
  echo "# HiveMind overlay manifest — generated $(date -u +%FT%TZ)"
  echo "# format: consumer_tag<TAB>dist_file"
  for src in "${!MAP[@]}"; do
    printf '%s\t%s\n' "${MAP[$src]}" "$DIST/${MAP[$src]}.txt"
  done
} | sort -k1,1 > "$DIST/MANIFEST.tsv"

log "done: $ok ok, $fail warn/fail   manifest: $DIST/MANIFEST.tsv"
[[ $fail -eq 0 ]]
