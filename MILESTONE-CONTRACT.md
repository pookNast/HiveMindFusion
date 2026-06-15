# Milestone Contract: HiveMind — Local LLM Management Platform

**Project**: HiveMind
**Repo**: /home/pook/ralph/hivemind
**Owner**: pook
**Date**: 2026-05-16
**Confidence**: 0.92 (pending user approval of architecture)

---

## Architecture (LOCKED after approval)

| Component | Technology | Why |
|-----------|-----------|-----|
| Gateway (hivemind-gw) | Go + net/http | Go already in stack (anonymizer-go), low latency, single binary |
| Model Manager (hivemind-ctl) | Go CLI | Model lifecycle, VRAM calc, hot-swap orchestration |
| PII Middleware | Go (calls PII Shield :5100/scan) | Inline request/response filtering, zero new Python |
| VRAM Monitor | Python (NVML ctypes) | Already proven in start-turboquant scripts |
| Inference Backend | TurboQuant llama-server (existing) | No change — gateway wraps it |
| Metrics | Prometheus (existing :9090) | Already running, add hivemind exporters |
| Dashboard | Grafana (existing) | Add hivemind dashboard JSON |
| Knowledge Pipeline | Python + Qdrant (existing :6333) | Embedding ingestion, RAG endpoint |
| Service Management | OpenRC | Existing pattern on BatKave |
| Config | TOML at /etc/hivemind/config.toml | Single source of truth |

### Network Topology

```
Clients (OpenClaude, RALPH, SiteOps, revenue products)
    │
    ▼
[hivemind-gw :8400]  ← single OpenAI-compatible endpoint
    │
    ├─→ [PII Shield :5100/scan]  (inline middleware)
    │
    ├─→ [TurboQuant llama-server :11434]  (primary, Qwen3.6-27B)
    ├─→ [Ollama :11435]  (secondary, multi-model)
    ├─→ [VPS cloud fallback]  (via Headscale 100.64.0.2)
    │
    ├─→ [Qdrant :6333]  (RAG context injection)
    │
    └─→ [Prometheus :9090]  (metrics export)
```

### Port Assignments

| Service | Port | Notes |
|---------|------|-------|
| hivemind-gw | 8400 | Primary API gateway |
| hivemind-gw admin | 8401 | Admin API (model mgmt, health, config) |
| hivemind-gw metrics | 8402 | Prometheus /metrics |
| TurboQuant | 11434 | Unchanged |
| PII Shield | 5100 | Unchanged |
| Sanitizer | 5000 | Unchanged |
| Qdrant | 6333 | Unchanged (Headscale) |

---

## Stack Constraints

- Go 1.22+ for gateway and CLI (match existing go.mod toolchain)
- NO new Python services (only VRAM monitor script + existing PII Shield)
- NO Docker for gateway (native binary + OpenRC, matches BatKave pattern)
- ALL changes must pass `go vet && go build` 
- Existing oc-start/claude-glm/oc-proxy MUST continue working during migration
- VRAM budget: 24GB total, ~20GB for model+KV, ~4GB headroom
- Config changes require gateway reload, NOT restart

---

## Success Criteria

### MUST (blocks ship)
- [ ] `hivemind-gw` serves OpenAI-compatible `/v1/chat/completions` on :8400
- [ ] Requests are PII-scanned via :5100 before reaching inference backend
- [ ] Responses are PII-scanned before returning to client
- [ ] Smart routing: model field maps to backend (qwen3.6→TurboQuant, glm-flash→Ollama)
- [ ] Fallback chain: if primary backend unhealthy, route to next available
- [ ] `hivemind-ctl models list` shows all GGUFs in /mnt/olympus/LLMs/gguf/
- [ ] `hivemind-ctl models load <name>` starts TurboQuant with correct params
- [ ] `hivemind-ctl models unload` gracefully stops inference server
- [ ] VRAM calculator: `hivemind-ctl vram estimate --model X --context Y --quant Z`
- [ ] Per-consumer rate limiting (configurable in config.toml)
- [ ] Prometheus metrics: request count, latency p50/p99, tokens/sec, VRAM usage
- [ ] OpenRC service file: `rc-service hivemind start/stop/restart`
- [ ] oc-start migrated to use :8400 gateway (with fallback to direct :11434)
- [ ] Zero data leakage: PII scan blocks requests containing API keys, PII, secrets
- [ ] Health endpoint: GET /health returns backend status, VRAM, model loaded

### SHOULD (ship with known gaps)
- [ ] Hot-swap models without gateway restart (backend rotation)
- [ ] Auto-tune spec decoding params based on model metadata
- [ ] Grafana dashboard JSON provisioned automatically
- [ ] Usage analytics: per-consumer token counts, cost attribution
- [ ] RAG context injection via Qdrant for configured consumers
- [ ] Streaming SSE pass-through with PII scan on chunks

### NICE (defer to next sprint)
- [ ] Web admin UI for model management
- [ ] Auto-pull new model versions from HuggingFace
- [ ] Client-facing white-label API tier with auth
- [ ] Self-healing: auto-restart on OOM, VRAM leak detection
- [ ] Multi-node: route to BatKave or VPS based on load

---

## Env Vars Required

| Var | Default | Notes |
|-----|---------|-------|
| HIVEMIND_CONFIG | /etc/hivemind/config.toml | Config file path |
| HIVEMIND_PORT | 8400 | Gateway listen port |
| HIVEMIND_ADMIN_PORT | 8401 | Admin API port |
| HIVEMIND_METRICS_PORT | 8402 | Prometheus port |
| HIVEMIND_MODEL_DIR | /mnt/olympus/LLMs/gguf | GGUF storage |
| HIVEMIND_PII_ENDPOINT | http://127.0.0.1:5100 | PII Shield |
| HIVEMIND_QDRANT_ENDPOINT | http://100.64.0.3:6333 | Qdrant |
| HIVEMIND_LOG_LEVEL | info | Log verbosity |

---

## Risk Register

| Risk | Impact | Mitigation |
|------|--------|------------|
| PII Shield :5100 down → all requests blocked | HIGH | Circuit breaker: if PII unhealthy >30s, configurable bypass/block policy |
| TurboQuant OOM crash | HIGH | VRAM monitor detects, auto-restart with lower context |
| Gateway adds latency to inference | MED | PII scan async where possible, connection pooling |
| Ollama port conflict with TurboQuant | MED | Move Ollama to :11435, update config |
| Migration breaks oc-start | MED | Phased: oc-start uses gateway with fallback to direct |
