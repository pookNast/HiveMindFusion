# SHIP/NO-SHIP Verification Plan — HiveMind Consolidation

## Decision Framework

**SHIP** requires ALL of the following:
1. All 10 MUST criteria produce PASS evidence in `logs/evidence-MUST-*.log`
2. All 6 confidence gates report >= 0.95 in `logs/confidence-gates.json`
3. No unresolved blocking PRD items (all `passes: true` in prd.json)
4. Build artifact (`hivemind-gw`) exists and matches committed source
5. `go test ./...` exits 0
6. All 14 audit reports exist in `logs/audit-specs/`

**NO-SHIP** triggers when:
- Any MUST criterion fails → add blocking PRD item, iterate
- Any confidence gate < 0.95 → identify gap, add blocking PRD item
- Any audit report contains VERDICT: FAIL → address finding

---

## Confidence Gate Evaluators

Each role uses an independent Sonnet agent. No agent evaluates its own work.

| Role | What They Review | Evidence Sources |
|------|-----------------|-----------------|
| **Dev** | Code changes compile, tests pass, no regressions | `go build`, `go test`, git diff |
| **Architect** | Architecture matches locked diagram, no new deps, scope contained | MILESTONE_CONTRACT.md, go.mod diff, code review |
| **SiteOps** | Services healthy, ports respond, logs flow, restart clean | curl health checks, systemd status, log files |
| **Security** | PII fail-closed, no credentials in code, rate limits active | grep for secrets, PII shield health, rate limit test |
| **Product** | All 4 consumers routed, metrics visible, design intent met | Prometheus metrics, consumer labels |
| **QA** | End-to-end validation W4-01 through W4-05 all PASS | logs/W4-*.log evidence files |

---

## Verification Sequence

### Phase 1: Evidence Collection (automated)

```bash
# Run all MUST verification commands and capture evidence
for i in $(seq 1 10); do
  echo "=== MUST-$i ===" >> logs/evidence-MUST-$i.log
  # Run verify_cmd from MILESTONE_CONTRACT.md
done
```

### Phase 2: Audit Execution (Wave 5)

```bash
# Launch W5-01 — runs all 14 audits
bash launchers/W5-01-launch.sh
```

### Phase 3: Confidence Collection (Wave 5)

```bash
# Launch W5-02 — 6 independent evaluator agents
bash launchers/W5-02-launch.sh
```

### Phase 4: Final Decision (Wave 5)

```bash
# Launch W5-03 — independent evaluator renders verdict
bash launchers/W5-03-launch.sh

# Check result
grep 'VERDICT' logs/final-ship-decision.md
```

---

## Residual Risk Register

After SHIP, document remaining risks in `logs/residual-risks.md`:

| Risk | Impact | Mitigation | Owner |
|------|--------|-----------|-------|
| Ollama-secondary may be intentionally cold | Backend count shows 6/7 | Config documents expected-down status | SiteOps |
| Headroom adds ~50ms p99 latency | Minor perf impact | 500ms timeout already configured | SiteOps |
| CEO-Agent cloud fallbacks bypass hivemind | Expected — cloud != local | Documented in architecture | Architect |
| RLM `claude --print` bypasses hivemind | Expected — Anthropic native API | Documented in architecture | Architect |

---

## Rollback Procedure

If post-ship issues discovered:

1. **Immediate**: `cd ~/ralph/hivemind && git revert HEAD && bash restart.sh`
2. **CEO-Agent**: `cd ~/project-incubator/ceo-agent && git revert HEAD && systemctl restart ceo-agent`
3. **RLM Engine**: `cd ~/ralph/rlm-engine && git revert HEAD` (launchers are stateless)
4. **Verify rollback**: Re-run MUST-1 through MUST-3 health checks

---

## Swarm Execution Commands

```bash
# Full pipeline execution:

# 1. Preflight validation
cd ~/ralph/hivemind
bash ralph/rlm-engine/launchers/rlm-preflight.sh

# 2. Wave 0 — parallel (backends + config)
tmux new-session -d -s w0-01 'bash launchers/W0-01-launch.sh'
tmux new-session -d -s w0-02 'bash launchers/W0-02-launch.sh'
tmux new-session -d -s w0-03 'bash launchers/W0-03-launch.sh'
tmux new-session -d -s w0-04 'bash launchers/W0-04-launch.sh'
tmux new-session -d -s w0-05 'bash launchers/W0-05-launch.sh'
tmux new-session -d -s w0-06 'bash launchers/W0-06-launch.sh'
# Wait for W0 completion, then W0-07 gate:
bash launchers/W0-07-launch.sh

# 3. Wave 1 + Wave 2 — parallel (RLM + CEO-Agent wiring)
tmux new-session -d -s w1-01 'bash launchers/W1-01-launch.sh'
tmux new-session -d -s w1-02 'bash launchers/W1-02-launch.sh'
tmux new-session -d -s w1-03 'bash launchers/W1-03-launch.sh'
tmux new-session -d -s w1-04 'bash launchers/W1-04-launch.sh'
tmux new-session -d -s w1-05 'bash launchers/W1-05-launch.sh'
tmux new-session -d -s w2-01 'bash launchers/W2-01-launch.sh'
tmux new-session -d -s w2-02 'bash launchers/W2-02-launch.sh'
tmux new-session -d -s w2-03 'bash launchers/W2-03-launch.sh'
tmux new-session -d -s w2-04 'bash launchers/W2-04-launch.sh'

# 4. Wave 3 — sequential (build + deploy)
bash launchers/W3-01-launch.sh
bash launchers/W3-02-launch.sh
bash launchers/W3-03-launch.sh

# 5. Wave 4 — parallel validation
tmux new-session -d -s w4-01 'bash launchers/W4-01-launch.sh'
tmux new-session -d -s w4-02 'bash launchers/W4-02-launch.sh'
tmux new-session -d -s w4-03 'bash launchers/W4-03-launch.sh'
tmux new-session -d -s w4-04 'bash launchers/W4-04-launch.sh'
tmux new-session -d -s w4-05 'bash launchers/W4-05-launch.sh'

# 6. Wave 5 — sequential (audit + ship decision)
bash launchers/W5-01-launch.sh
bash launchers/W5-02-launch.sh
bash launchers/W5-03-launch.sh

# 7. Check verdict
grep 'VERDICT' logs/final-ship-decision.md
```

---

## Night Queue Alternative

For off-peak execution (recommended for Max plan utilization):

```bash
/night-queue add --type maintenance --priority P1 \
  --payload "Execute MILESTONE-CONTRACT-CONSOLIDATION milestones M-1 through M-6" \
  --prd_file ralph/hivemind/prd.json \
  --profile production_compact_rlm
```
