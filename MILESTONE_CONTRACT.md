# MILESTONE CONTRACT: HiveMind Consolidation — Unified AI Inference Plane

**Project:** hivemind-consolidation
**Created:** 2026-06-07
**Repo:** `/home/pook/ralph/hivemind`
**Confidence Target:** ≥0.95 all roles
**Decision:** SHIP when all MUST criteria pass + all confidence gates ≥0.95

---

## 1. Locked Architecture

```
ALL LOCAL AI INFERENCE
  ├─ OpenClaude (oc-start)
  ├─ Departmental Agents (legal, sales, ops)
  ├─ RLM Engine / Super-Worker
  └─ CEO-Agent (C-Suite AWE)
        │
        ▼
  [HiveMind Gateway :8400]
        ├─ PII Shield (:5100, fail-closed)
        ├─ RAG/Qdrant (:6333, knowledge_graph)
        ├─ Headroom Compression (:9103)
        └─ Prometheus Metrics (:8402)
              │
              ▼
        Backends (7 total)
          ├─ TurboQuant :11434
          ├─ Ollama :11435
          ├─ VPS 100.64.0.2:11434
          └─ 4× Z.AI Cloud
```

**Scope boundary:** Hivemind handles OpenAI-compatible local inference only. `claude --print` (Anthropic native API) and OpenRouter cloud calls remain direct — by design.

---

## 2. Stack / Environment / Deploy Constraints

| Component | Version / Config |
|-----------|-----------------|
| Go | 1.23.0 (toolchain 1.24.4) |
| Module | github.com/pook/hivemind |
| Config format | TOML (BurntSushi/toml v1.4.0) |
| CEO-Agent Go | 1.25.0 |
| Gateway ports | 8400 (proxy), 8401 (admin), 8402 (metrics) |
| PII Shield | :5100 (fail-closed, 500ms timeout) |
| RAG | Qdrant :6333, collection=knowledge_graph |
| Headroom | :9103, min_body=100 bytes, 500ms timeout |
| Build | `go build -o hivemind-gw ./cmd/hivemind-gw/` |
| Deploy | `restart.sh` → cp binary → restart process |
| CEO-Agent deploy | systemd: ceo-agent.service |
| Model path | /mnt/olympus/LLMs/gguf |
| Default model | qwopus3.6-27b-v2 |

---

## 3. Dependency Policy

- No new Go dependencies without architecture review
- TOML config changes must parse without errors before commit
- Z.AI API keys stored in batkave.toml (existing pattern — not ideal, accepted)
- CEO-Agent secrets via Vaultwarden (deploy.sh pulls at deploy time)
- RLM secrets via Hermes Vault (HERMES_VAULT_PASSPHRASE from Bitwarden)

---

## 4. Security Boundaries

| Boundary | Enforcement |
|----------|-------------|
| PII scanning | Fail-closed at hivemind layer; CEO-Agent redundant call removed |
| API keys | Never in env vars for Anthropic (per openclaw auth memory); Hermes Vault for RLM |
| Rate limiting | Per-consumer: rlm-swarm 120/min, ceo-agent 30/min, openclaude uncapped |
| Circuit breaker | Per-backend health checks, automatic failover |
| Network | Local inference stays on LAN/Headscale; cloud calls via Z.AI only |

---

## 5. Success Criteria

### MUST (blocking — each has verification command)

| ID | Criterion | Verification Command |
|----|-----------|---------------------|
| MUST-1 | 7/7 backends healthy | `curl -s http://127.0.0.1:8401/health \| jq '.backends \| map(select(.healthy == false)) \| length'` → `0` |
| MUST-2 | rlm-swarm consumer configured | `grep -c 'rlm-swarm' config/batkave.toml` → `≥1` |
| MUST-3 | ceo-agent consumer configured | `grep -c 'ceo-agent' config/batkave.toml` → `≥1` |
| MUST-4 | RLM routes through hivemind when RLM_MODEL_OVERRIDE=hivemind | `RLM_MODEL_OVERRIDE=hivemind bash agent-launch.sh 2>&1 \| grep -c 'X-HiveMind-Backend'` → `≥1` |
| MUST-5 | CEO-Agent routes local inference through :8400 | CEO-Agent logs show `X-HiveMind-Backend` header |
| MUST-6 | hivemind-gw builds cleanly | `cd ~/ralph/hivemind && go build -o hivemind-gw ./cmd/hivemind-gw/` → exit 0 |
| MUST-7 | All tests pass | `cd ~/ralph/hivemind && go test ./...` → exit 0 |
| MUST-8 | 4 consumer labels in Prometheus metrics | `curl -s http://127.0.0.1:8402/metrics \| grep hivemind_requests_total` shows openclaude, compliancebot, rlm-swarm, ceo-agent |
| MUST-9 | TOML config parses without errors | `python3 -c "import tomllib; tomllib.load(open('config/batkave.toml','rb'))"` → exit 0 |
| MUST-10 | All changes committed and pushed to Forgejo | `git status --porcelain \| wc -l` → `0` (for tracked hivemind files) |

### SHOULD (important, non-blocking)

| ID | Criterion |
|----|-----------|
| SHOULD-1 | Headroom compress_count > 5 (baseline) after validation |
| SHOULD-2 | PII-scanned inference requests = 100% of local inference |
| SHOULD-3 | p99 latency < 2s for local inference through hivemind |
| SHOULD-4 | CEO-Agent double-PII-scan eliminated |

### NICE (stretch)

| ID | Criterion |
|----|-----------|
| NICE-1 | RAG context injection verified for ceo-agent consumer |
| NICE-2 | Rate limit tested: rlm-swarm burst rejection at >40 concurrent |
| NICE-3 | Dashboard/Grafana panel for hivemind consumer breakdown |

---

## 6. Hyperframe / Evidence Requirements

Each MUST criterion must produce:
- Command output captured in `logs/evidence-<MUST-ID>.log`
- Timestamp of execution
- Exit code

No screenshots/video required (CLI-only project).

---

## 7. Observability / Logging Requirements

| System | Requirement |
|--------|-------------|
| Hivemind gateway | Structured JSON logs to stdout, captured by restart.sh |
| Prometheus metrics | :8402 exposes `hivemind_requests_total{consumer=...}`, `hivemind_backend_health`, `hivemind_latency_seconds` |
| PII Shield | Health at :5100/health, fail-closed mode logged |
| Headroom | Health at :9103/health, compress_count metric |
| RLM Engine | Per-agent logs to `logs/<ITEM_ID>.log` |
| CEO-Agent | systemd journal: `journalctl -u ceo-agent` |

---

## 8. Rollback / Backup Requirements

| Action | Rollback |
|--------|----------|
| Config change (batkave.toml) | `git checkout HEAD~1 -- config/batkave.toml && restart.sh` |
| Binary rebuild | Previous binary at `/usr/local/bin/hivemind-gw` (backup before replace) |
| CEO-Agent adapter change | `git revert` + `systemctl restart ceo-agent` |
| RLM launcher changes | `git revert` — launchers are stateless scripts |
| Full rollback | `git revert <merge-commit>` + restart all services |

---

## 9. Confidence Gate Policy

| Role | Required | Gate |
|------|----------|------|
| Dev | ≥0.95 | All MUST criteria pass, tests green |
| Architect | ≥0.95 | Architecture matches locked diagram, no new deps |
| SiteOps | ≥0.95 | Services restart cleanly, ports respond, logs flowing |
| Security | ≥0.95 | PII fail-closed, no credential exposure, rate limits active |
| Product/Design | ≥0.95 | All consumers routed, metrics visible |
| QA/Verification | ≥0.95 | End-to-end validation passes all 5 checks |

---

## 10. SHIP / NO-SHIP Decision Rule

**SHIP** when:
1. ALL 10 MUST criteria produce PASS evidence
2. ALL 6 confidence gates report ≥0.95
3. No unresolved blocking PRD items
4. Build + test artifacts exist in logs/

**NO-SHIP** when:
- Any MUST criterion fails → add blocking PRD item, iterate
- Any confidence gate < 0.95 → identify gap, add blocking PRD item
- Failed gates cannot be waived silently
- Record all residual risks in `logs/residual-risks.md`

**Decision authority:** Independent evaluator agent (never self-evaluated).
