# HiveMind Consolidation — Shared Context

## Project
Unify all local AI inference through the HiveMind gateway (:8400).
Currently 2/4 consumers bypass hivemind, missing PII filtering, RAG, metrics, compression.

## Repo
`/home/pook/ralph/hivemind` — Go 1.23, TOML config, Prometheus metrics.

## Architecture
```
Consumers → [HiveMind :8400] → PII(:5100) → RAG(:6333) → Headroom(:9103) → Backends(7)
```
Ports: 8400 (proxy), 8401 (admin/health), 8402 (metrics).

## Key Files
- `config/batkave.toml` — gateway config (backends, consumers, rate limits, RAG, PII)
- `internal/config/config.go` — Go config struct
- `internal/gateway/proxy.go` — request routing, backend selection, fallback
- `cmd/hivemind-gw/main.go` — entry point
- `restart.sh` — deploy script (kill → cp binary → restart)

## Related Projects
- RLM Engine: `~/ralph/rlm-engine/launchers/` — agent-launch.sh, gen-launchers.sh, mega-chief.sh, rlm-preflight.sh
- CEO-Agent: `~/project-incubator/ceo-agent/` — Go, systemd, adapter registry pattern

## Contracts
- Read `MILESTONE_CONTRACT.md` for success criteria and verification commands
- Read `prd.json` for your specific item's acceptance criteria

## Rules
1. Edit files once — read full file, plan all changes, single edit
2. Run verification command after changes
3. Write evidence to `logs/<ITEM_ID>.log`
4. Report blockers as: `BLOCKER: <description> | ITEM: <id> | SEVERITY: blocking|non-blocking`
5. Never mark your own work complete — evaluator verifies
6. No new dependencies without architecture review
7. TOML must parse after every config edit
8. PII Shield is fail-closed — never disable or bypass
9. Cloud API calls (Anthropic, OpenRouter) stay direct — hivemind is local inference only
10. Keep changes minimal — no refactoring beyond the task scope

## Build & Test
```bash
cd ~/ralph/hivemind
go build -o hivemind-gw ./cmd/hivemind-gw/
go test ./...
python3 -c "import tomllib; tomllib.load(open('config/batkave.toml','rb'))"
```

## Verification
After your changes, run your item's verify_cmd from prd.json and capture output to logs/<ITEM_ID>.log.
