# HiveMind Deployment Contract — BatKave Production

**Date**: 2026-05-16
**Target**: BatKave (192.168.183.108 / 100.64.0.3)
**Operator**: pook
**Status**: PRE-DEPLOY

---

## Phase 1: Pre-Deploy Validation (local, before touching production)

### M1.1 — Fix production config (batkave.toml)
**Why**: Current config has wrong model names, port conflicts, missing sections.

| Fix | Current | Correct |
|-----|---------|---------|
| Primary model | `glm-flash:latest` | `qwen3.6-27b` |
| Secondary model | `qwen2.5:0.5b` | `glm-flash` |
| VPS fallback URL | `http://100.64.0.3:11434` | `http://100.64.0.2:11434` (VPS, not BatKave) |
| Metrics port | `9090` (conflicts with Prometheus) | `8402` |
| Health endpoints | `/api/tags` (Ollama) | `/health` (llama-server) for primary |
| Models dir | `/var/lib/hivemind/models` | `/mnt/olympus/LLMs/gguf` |
| Missing: `[embed]` | n/a | `endpoint = "http://localhost:11434"`, `model = "qwen2.5:0.5b"` |
| Missing: `[rag]` | n/a | consumer entries for openclaude, ralph-swarm |
| Missing: Qwen3.6 backend for `qwen3.6-27b` model routing | n/a | Add backend entry |

**Acceptance**: `hivemind-gw --config config/batkave.toml --dry-run` exits 0

### M1.2 — Full build + unit tests
```bash
go vet ./... && go build ./... && go test ./...
```
**Acceptance**: Zero errors, both binaries produced

### M1.3 — Binary smoke test (standalone, no install)
```bash
./hivemind-gw --config config/batkave.toml &
sleep 2
curl -sf http://localhost:8400/v1/models
curl -sf http://localhost:8401/health
curl -sf http://localhost:8402/metrics | head -5
kill %1
```
**Acceptance**: All three endpoints respond, health shows TurboQuant backend status

---

## Phase 2: System Preparation

### M2.1 — Create directories + user
```bash
sudo mkdir -p /etc/hivemind /var/log/hivemind /var/lib/hivemind
sudo cp config/batkave.toml /etc/hivemind/config.toml
```
**Acceptance**: Config readable at `/etc/hivemind/config.toml`

### M2.2 — Install binaries
```bash
go build -o /tmp/hivemind-gw ./cmd/hivemind-gw
go build -o /tmp/hivemind-ctl ./cmd/hivemind-ctl
sudo cp /tmp/hivemind-gw /usr/local/bin/hivemind-gw
sudo cp /tmp/hivemind-ctl /usr/local/bin/hivemind-ctl
sudo chmod +x /usr/local/bin/hivemind-gw /usr/local/bin/hivemind-ctl
```
**Acceptance**: `hivemind-gw --help` and `hivemind-ctl --help` work from PATH

### M2.3 — Resolve port conflicts
- Confirm :8400, :8401, :8402 are free
- If Ollama is on :11434, either: (a) move Ollama to :11435, or (b) share port with TurboQuant via model routing
- Confirm PII Shield on :5100 is healthy
- Confirm Qdrant on :6333 is healthy

**Acceptance**: `ss -tlnp | grep -E '8400|8401|8402'` shows nothing before install

### M2.4 — Prometheus config update
Add HiveMind scrape target to existing Prometheus config:
```yaml
- job_name: 'hivemind'
  static_configs:
    - targets: ['localhost:8402']
```
**Acceptance**: `curl http://localhost:9090/api/v1/targets | grep hivemind` shows UP after reload

---

## Phase 3: Service Deployment

### M3.1 — Install OpenRC service
```bash
sudo cp etc/openrc/hivemind /etc/init.d/hivemind
sudo chmod +x /etc/init.d/hivemind
sudo rc-update add hivemind default
```
**Acceptance**: `rc-status | grep hivemind` shows service registered

### M3.2 — Start service
```bash
sudo rc-service hivemind start
```
**Acceptance**: 
- `rc-service hivemind status` shows running
- `curl -sf http://localhost:8400/v1/models` returns model list
- `curl -sf http://localhost:8401/health` returns backend states
- `/var/log/hivemind/` has log output

### M3.3 — Verify PII Shield integration
```bash
# Test: send fake PII through gateway
curl -s http://localhost:8400/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-HiveMind-Consumer: test" \
  -d '{"model":"qwen3.6-27b","messages":[{"role":"user","content":"My SSN is 123-45-6789"}]}'
```
**Acceptance**: PII is redacted or request is blocked (depending on PII Shield config)

### M3.4 — Verify fallback chain
```bash
# Stop TurboQuant, verify gateway falls back
sudo kill $(cat /home/pook/local-llm/logs/turboquant-qwen36.pid)
sleep 5
curl -s http://localhost:8401/health  # should show turboquant unhealthy
# Restart TurboQuant
bash ~/local-llm/scripts/start-turboquant-qwen36.sh
```
**Acceptance**: Health endpoint reflects backend state changes within health_interval_secs

### M3.5 — Verify rate limiting
```bash
# Rapid-fire requests (should trigger 429)
for i in $(seq 1 70); do
  curl -s -o /dev/null -w "%{http_code}\n" \
    -H "X-HiveMind-Consumer: test-ratelimit" \
    http://localhost:8400/v1/models
done | sort | uniq -c
```
**Acceptance**: Some requests return 429, openclaude consumer is unlimited

---

## Phase 4: Migration — Route Consumers Through Gateway

### M4.1 — Migrate oc-start
Update `/home/pook/bin/oc-start` to use gateway:
```bash
# Change:
export OPENAI_BASE_URL=http://127.0.0.1:11434/v1
# To:
export OPENAI_BASE_URL=http://127.0.0.1:8400/v1
```
Add fallback logic: if :8400 not healthy, revert to :11434.
Add consumer header via env or OpenClaude config.

**Acceptance**: `oc-start` launches, connects through gateway, model responds correctly

### M4.2 — Migrate RALPH swarm agents
Update mega-chief.sh / swarm-host.sh to route through :8400 with `X-HiveMind-Consumer: ralph-swarm`.

**Acceptance**: Swarm agents can complete tasks through gateway

### M4.3 — Verify oc-proxy still works
`oc-proxy` routes to GLM-5.1 (cloud) — should be unaffected by gateway. Verify no regression.

**Acceptance**: `oc-proxy "test"` returns GLM response

### M4.4 — Verify claude-glm still works  
`claude-glm` routes to Z.AI — completely independent of gateway. Verify no regression.

**Acceptance**: `claude-glm` launches without error

---

## Phase 5: Observability

### M5.1 — Grafana dashboard
```bash
# Import dashboard
curl -s -X POST http://localhost:3000/api/dashboards/db \
  -H "Content-Type: application/json" \
  -d @config/grafana-hivemind.json
```
**Acceptance**: Dashboard visible at Grafana, panels populated with data

### M5.2 — Verify metrics flow
After 5 minutes of operation:
- `hivemind_requests_total` incrementing
- `hivemind_request_duration_seconds` has histogram data
- `hivemind_backend_health` shows 1 for healthy backends
- `hivemind_vram_usage_bytes` shows current VRAM

**Acceptance**: `curl localhost:8402/metrics | grep hivemind_` shows non-zero values

### M5.3 — Log rotation
```bash
# Add logrotate config
sudo tee /etc/logrotate.d/hivemind << 'LOGEOF'
/var/log/hivemind/*.log {
    daily
    rotate 7
    compress
    missingok
    notifempty
    postrotate
        rc-service hivemind reload 2>/dev/null || true
    endscript
}
LOGEOF
```
**Acceptance**: `logrotate -d /etc/logrotate.d/hivemind` shows no errors

---

## Phase 6: Hardening + Smoke Test

### M6.1 — Full smoke test
```bash
bash scripts/smoke-test.sh
```
**Acceptance**: Script exits 0, all checks pass

### M6.2 — OOM recovery test
```bash
# Verify gateway survives TurboQuant OOM
hivemind-ctl vram estimate --model Qwen3.6-27B-MTP-UD-Q4_K_XL --context 160000 --kv turbo3
# Should show ~20GB, fits=YES
hivemind-ctl vram estimate --model Qwen3.6-27B-MTP-UD-Q4_K_XL --context 160000 --kv turbo3 --spec-n-max 6
# Should show NO-FIT
```
**Acceptance**: VRAM calculator correctly predicts fit/no-fit

### M6.3 — Reboot survival
```bash
sudo reboot
# After reboot:
rc-service hivemind status  # should auto-start
curl -sf http://localhost:8400/v1/models  # should respond
curl -sf http://localhost:8401/health     # should show backends
```
**Acceptance**: Gateway auto-starts on boot, all endpoints healthy within 60s

### M6.4 — hivemind-ctl model operations
```bash
hivemind-ctl models list
hivemind-ctl vram status
hivemind-ctl vram budget
```
**Acceptance**: All commands produce correct output

---

## Phase 7: Documentation + Handoff

### M7.1 — Update oc-start comments
Reflect new gateway routing in script comments.

### M7.2 — Update local-llm/CLAUDE.md
Add HiveMind gateway to the project context doc.

### M7.3 — Memory update
Save HiveMind project memory with architecture, ports, config path.

---

## Rollback Plan

If gateway causes issues at any phase:
1. `sudo rc-service hivemind stop`
2. Revert oc-start: `OPENAI_BASE_URL=http://127.0.0.1:11434/v1`
3. All consumers fall back to direct backend connections
4. No data loss — gateway is stateless proxy

---

## Success Criteria Summary

| Gate | Check | Blocks Ship |
|------|-------|-------------|
| Build | `go vet && go build && go test` | YES |
| Config | `--dry-run` exits 0 | YES |
| Gateway up | :8400, :8401, :8402 respond | YES |
| PII scan | Requests sanitized | YES |
| Health check | Backend state tracked | YES |
| Rate limit | 429 on excess | YES |
| oc-start works | OpenClaude through gateway | YES |
| Fallback | Routes to secondary on primary down | YES |
| Metrics | Prometheus scraping hivemind | SHOULD |
| Grafana | Dashboard imported | SHOULD |
| Reboot | Auto-start on boot | SHOULD |
| RAG | Context injection for configured consumers | NICE |
| Model swap | `hivemind-ctl models load/unload` | NICE |

**SHIP decision**: All YES gates pass → SHIP. SHOULD gates can ship with known gaps. NICE gates deferred to next sprint.
