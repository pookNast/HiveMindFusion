# Milestone Contract: HiveMind Consolidation — Unified AI Inference Plane

**Project:** hivemind-consolidation
**Created:** 2026-06-07
**Confidence Target:** 0.95+ all backends healthy, all consumers routed
**Repo:** `/home/pook/ralph/hivemind`
**Depends On:** HiveMind Gateway (DEPLOYED), PII Shield (UP), Qdrant (UP), Headroom Sidecar (UP)
**Audit Date:** 2026-06-07 — triggered by routing gap discovery

---

## Problem Statement

The HiveMind gateway (:8400) is deployed and healthy, but only **2 of 4 consumer systems** route through it. The RLM engine (heaviest workload — swarm agents) and CEO-Agent (C-Suite AWE — executive decisions) bypass hivemind entirely, missing PII filtering, RAG injection, metrics, compression, and unified fallback. Additionally, 2 of 7 backends are unhealthy, and 82 lines of config/proxy changes sit uncommitted.

### Current State

| System | Through Hivemind | Gap |
|--------|-----------------|-----|
| OpenClaude (oc-start) | YES | — |
| Departmental agents (legal, sales, ops, etc.) | YES | — |
| **RLM Engine / Super-Worker** | **NO** | Direct Anthropic → ChatGPT :8094 → Z.AI |
| **CEO-Agent (C-Suite AWE)** | **NO** | Direct Ollama :11434 → ChatGPT :8094 → Anthropic → OpenRouter |

### What Bypassed Systems Miss

- PII Shield (fail-closed scan before inference)
- RAG context injection from knowledge graph
- Prometheus per-consumer metrics
- Headroom compression (token savings)
- Rate limiting & circuit breakers
- Unified fallback chain with health checks
- `X-HiveMind-Backend` traceability headers

---

## Milestones

| ID | Milestone | Wave | Items | Target | Status | Confidence |
|----|-----------|------|-------|--------|--------|------------|
| M-1 | Fix Unhealthy Backends | W0 | 3 | 06/07 | TODO | 0.00 |
| M-2 | Hivemind Config Upgrades | W0 | 4 | 06/07 | TODO | 0.00 |
| M-3 | Wire RLM Engine Through Hivemind | W1 | 5 | 06/08 | TODO | 0.00 |
| M-4 | Wire CEO-Agent Through Hivemind | W1 | 4 | 06/08 | TODO | 0.00 |
| M-5 | Commit & Deploy | W2 | 3 | 06/08 | TODO | 0.00 |
| M-6 | End-to-End Validation | W3 | 5 | 06/09 | TODO | 0.00 |

---

## Execution Strategy

### Layer Parallelism (DAG)

```
Layer 0: M-1 + M-2              ← parallel: fix backends + upgrade config
Layer 1: M-3 + M-4              ← parallel: wire RLM + wire CEO-Agent (blocked by M-1, M-2)
Layer 2: M-5                    ← sequential: commit, rebuild, restart (blocked by M-3, M-4)
Layer 3: M-6                    ← sequential: end-to-end validation (blocked by M-5)
```

### Model Allocation

| Role | Model | Rationale |
|------|-------|-----------|
| Config edits, script changes | Sonnet via `/fast` | Structured, low ambiguity |
| Backend diagnosis | Haiku subagent | Exploration/status checks |
| Integration testing | Sonnet via `/fast` | Scripted validation |
| Architecture decisions | Opus (current session) | Judgment-intensive |

### Token Budget Estimate

| Component | Agents | Tokens/Agent | Total |
|-----------|--------|-------------|-------|
| M-1 Backend diagnosis + fix | 2 | ~8K | ~16K |
| M-2 Config edits | 1 | ~6K | ~6K |
| M-3 RLM routing changes | 2 | ~10K | ~20K |
| M-4 CEO-Agent routing changes | 2 | ~10K | ~20K |
| M-5 Build + deploy | 1 | ~5K | ~5K |
| M-6 Validation | 3 | ~8K | ~24K |
| **Total** | **11** | — | **~91K Sonnet tokens** |

---

## Acceptance Criteria (per milestone)

### M-1: Fix Unhealthy Backends

**Problem:** `ollama-secondary` (:11435) and `vps-fallback` (100.64.0.2:11434) are DOWN per hivemind health checks.

- [ ] **M-1.1** Diagnose ollama-secondary — check if Ollama process is running on :11435, restart if needed
- [ ] **M-1.2** Diagnose vps-fallback — check VPS Ollama via Headscale (`ssh vps "ss -tlnp | grep 11434"`), restart if needed
- [ ] **M-1.3** Verify hivemind admin health shows 7/7 backends healthy: `curl -s http://127.0.0.1:8401/health`

**Exit gate:** `curl -s http://127.0.0.1:8401/health | jq '.backends | map(select(.healthy == false)) | length'` returns `0`

---

### M-2: Hivemind Config Upgrades

**Problem:** Config needs new consumers for RLM and CEO-Agent, headroom sidecar is underutilized (5 requests total), and 82 lines of proxy/config changes are uncommitted.

- [ ] **M-2.1** Add `rlm-swarm` consumer to rate limits (120 req/min, burst 40 — highest throughput consumer)
- [ ] **M-2.2** Add `ceo-agent` consumer to rate limits (30 req/min, burst 10 — low-volume executive decisions)
- [ ] **M-2.3** Add RAG context injection for `ceo-agent` consumer (collection=knowledge_graph, top_k=5, min_score=0.7 — executive decisions benefit from full context)
- [ ] **M-2.4** Add RAG context injection for `rlm-swarm` consumer — already exists as `ralph-swarm`, verify naming alignment or add alias

**Files:** `ralph/hivemind/config/batkave.toml`

**Exit gate:** TOML validates with no parse errors, new consumers appear in config

---

### M-3: Wire RLM Engine Through Hivemind

**Problem:** `agent-launch.sh` calls Anthropic API directly with ChatGPT (:8094) and Z.AI as fallbacks. This bypasses PII shield, metrics, RAG, and compression.

**Strategy:** For local-model RLM jobs (research, scaffold roles), route through hivemind. For Anthropic API jobs (`claude --print`), hivemind can't proxy Anthropic's native API — but we add the `X-HiveMind-Consumer` header for any OpenAI-compatible calls and wire Toolmem/research agents through :8400.

- [ ] **M-3.1** Add hivemind routing option to `agent-launch.sh`: when `RLM_MODEL_OVERRIDE=local` or `RLM_MODEL_OVERRIDE=hivemind`, set `OPENAI_BASE_URL=http://127.0.0.1:8400/v1` with consumer header `rlm-swarm`
- [ ] **M-3.2** Wire Toolmem research recall through hivemind: change recall queries in `agent-launch.sh` (lines 27-82) to use :8400 when available, fallback to direct
- [ ] **M-3.3** Add `HIVEMIND_URL` env var to `gen-launchers.sh` template so generated launchers inherit the routing
- [ ] **M-3.4** Add `HIVEMIND_URL` env var to `mega-chief.sh` orchestrator environment
- [ ] **M-3.5** Update `rlm-preflight.sh` to health-check hivemind (:8400) as part of preflight, warn if down

**Files:**
- `ralph/rlm-engine/launchers/agent-launch.sh` (routing logic ~L190-298)
- `ralph/rlm-engine/launchers/gen-launchers.sh`
- `ralph/rlm-engine/launchers/mega-chief.sh`
- `ralph/rlm-engine/launchers/rlm-preflight.sh`

**Exit gate:** `RLM_MODEL_OVERRIDE=hivemind bash agent-launch.sh` routes through :8400 (verify via `X-HiveMind-Backend` response header)

---

### M-4: Wire CEO-Agent Through Hivemind

**Problem:** CEO-Agent in `project-incubator/ceo-agent/` has its own fallback chain (Ollama :11434 → ChatGPT :8094 → Anthropic → OpenRouter). It should use hivemind as primary for local inference, keeping cloud fallbacks as-is.

**Strategy:** Replace direct `OLLAMA_BASE_URL=http://localhost:11434` with `http://localhost:8400/v1` and set consumer header. The CEO-Agent's Go code uses an LLM registry with adapters — add a hivemind adapter or modify the Ollama adapter URL.

- [ ] **M-4.1** Modify `registerAdapters()` in `cmd/ceo-agent/main.go`: change Ollama adapter URL from `:11434` to `:8400` (hivemind), add `X-HiveMind-Consumer: ceo-agent` header
- [ ] **M-4.2** Update `OLLAMA_BASE_URL` default in CEO-Agent config/env to `http://localhost:8400/v1`
- [ ] **M-4.3** Keep cloud fallbacks (Anthropic, OpenRouter) unchanged — these are already internet-bound, not local inference
- [ ] **M-4.4** Update CEO-Agent PII URL from `http://100.64.0.3:5000` to remove redundancy — hivemind already does PII scanning inline, so CEO-Agent's own PII call is double-scanning

**Files:**
- `project-incubator/ceo-agent/cmd/ceo-agent/main.go` (~L137-160)
- `project-incubator/ceo-agent/` env/config files

**Exit gate:** CEO-Agent logs show `X-HiveMind-Backend` in response headers for local inference calls

---

### M-5: Commit & Deploy

**Problem:** 82 lines of uncommitted changes in hivemind repo + new changes from M-2/M-3/M-4. Gateway binary needs rebuild.

- [ ] **M-5.1** Commit all hivemind changes (config/batkave.toml, internal/config/config.go, internal/gateway/proxy.go + any new changes)
- [ ] **M-5.2** Rebuild hivemind-gw: `cd ~/ralph/hivemind && go build -o hivemind-gw ./cmd/hivemind-gw/`
- [ ] **M-5.3** Restart gateway: `~/ralph/hivemind/restart.sh` — verify :8400/:8401/:8402 all listening

**Exit gate:** `curl -s http://127.0.0.1:8401/health` returns healthy, new consumers visible in config

---

### M-6: End-to-End Validation

**Problem:** Need to verify all 4 consumer systems route correctly through hivemind after changes.

- [ ] **M-6.1** Validate OpenClaude still routes through hivemind: start oc-start, send test prompt, verify `X-HiveMind-Backend` header
- [ ] **M-6.2** Validate departmental agents: run a test legal/compliance agent, verify metrics at :8402 show `compliancebot` consumer
- [ ] **M-6.3** Validate RLM engine: `RLM_MODEL_OVERRIDE=hivemind` test agent, verify :8402 metrics show `rlm-swarm` consumer
- [ ] **M-6.4** Validate CEO-Agent: start CEO-Agent, trigger a decision, verify :8402 metrics show `ceo-agent` consumer
- [ ] **M-6.5** Validate headroom compression utilization increased: `curl -s http://127.0.0.1:9103/health` shows compress_count > 5 (baseline)

**Exit gate:** `curl -s http://127.0.0.1:8402/metrics | grep hivemind_requests_total` shows all 4 consumer labels: `openclaude`, `compliancebot` (or similar), `rlm-swarm`, `ceo-agent`

---

## Risk Register

| Risk | Impact | Mitigation |
|------|--------|-----------|
| RLM `claude --print` can't route through hivemind (native Anthropic API) | M-3 scope limited to local-model jobs | Document clearly: hivemind handles OpenAI-compatible local inference only; Anthropic API calls remain direct |
| CEO-Agent Go adapter may not support custom headers easily | M-4 delay | Fall back to env var `OPENAI_BASE_URL` + query param consumer identification |
| Ollama-secondary (:11435) may be intentionally stopped (VRAM conservation) | M-1 false alarm | Check if it's meant to be cold-standby; if so, mark as expected-down in config |
| VPS Ollama down due to maintenance | M-1 blocked | If VPS unavailable, mark backend as intentional-offline and proceed |
| Headroom sidecar adds latency | Perf regression | Monitor p99 latency before/after; sidecar already has 500ms timeout |
| Double PII scanning (hivemind + CEO-Agent's own) | Wasted compute | M-4.4 removes CEO-Agent's redundant PII call |

---

## Routing Topology (Target State)

```
┌─────────────────────────────────────────────────────────────┐
│                    ALL LOCAL AI INFERENCE                     │
│                                                              │
│  OpenClaude ─────┐                                           │
│  Dept. Agents ───┤                                           │
│  RLM Swarm ──────┼──→ [HiveMind :8400] ──→ Backends         │
│  CEO-Agent ──────┘        │                   ├─ TurboQuant  │
│                           │                   │   :11434     │
│                           ├─ PII Shield       ├─ Ollama      │
│                           │   :5100           │   :11435     │
│                           ├─ RAG/Qdrant       ├─ VPS         │
│                           │   :6333           │   100.64.0.2 │
│                           ├─ Headroom         └─ Z.AI Cloud  │
│                           │   :9103                          │
│                           └─ Metrics                         │
│                               :8402                          │
│                                                              │
│  Claude API (Anthropic) ← RLM/Super-Worker (direct, N/A)    │
│  OpenRouter ← CEO-Agent cloud fallback (direct, N/A)         │
└─────────────────────────────────────────────────────────────┘
```

**Note:** `claude --print` (Anthropic native API) cannot route through hivemind — it uses Anthropic's proprietary protocol, not OpenAI-compatible. This is by design. Hivemind handles all **local inference** routing. Cloud API calls to Anthropic/OpenRouter remain direct.

---

## Launch Command

```bash
# Execute milestones in DAG order (Layer 0 → 3)
# Layer 0: Fix backends + config upgrades (parallel)
# Layer 1: Wire RLM + CEO-Agent (parallel, after Layer 0)
# Layer 2: Commit + rebuild + restart
# Layer 3: End-to-end validation

# Or queue for night execution:
# /night-queue add --type maintenance --priority P1 \
#   --payload "Execute MILESTONE-CONTRACT-CONSOLIDATION.md milestones M-1 through M-6"
```

---

## Metrics (Before/After)

| Metric | Before (2026-06-07) | Target |
|--------|-------------------|--------|
| Backends healthy | 5/7 | 7/7 |
| Consumers routed through hivemind | 2/4 | 4/4 |
| Headroom compress requests | 5 | >50/day |
| PII-scanned inference requests | ~40% | 100% of local inference |
| Prometheus consumer labels | 2 (openclaude, compliancebot) | 4+ |
| Unified fallback coverage | Partial | All local consumers |
