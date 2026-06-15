# W2-S2: Fallback chain routing

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: W1-S1, W1-S2

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Create internal/gateway/fallback.go — when primary backend returns error or is unhealthy, try next backend in priority order. Chain: TurboQuant(:11434) → Ollama(:11435) → VPS(100.64.0.2:11434) → error. Log fallback events. Add X-HiveMind-Fallback header when fallback used. Configurable per-model fallback chains in config.toml.

## Files to modify
- internal/gateway/fallback.go

## Acceptance criteria
- [ ] Primary failure triggers fallback to next healthy backend
- [ ] Fallback chain respects priority order
- [ ] X-HiveMind-Fallback header present on fallback responses
- [ ] All backends down returns 503 with descriptive error

## Rules
- cd into /home/pook/ralph/hivemind before any file operations
- Read each file BEFORE editing — understand existing code first
- Do NOT add new dependencies without explicit need
- Do NOT refactor surrounding code — surgical changes only
- Run typecheck/build after changes if available

## Verify
```bash
cd /home/pook/ralph/hivemind && cd /home/pook/ralph/hivemind && go vet ./internal/gateway/
```

## Commit
```
feat: [W2-S2] Fallback chain routing
```
