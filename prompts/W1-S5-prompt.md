# W1-S5: Gateway main + server wiring

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: W1-S1, W1-S2, W1-S3, W1-S4

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Wire cmd/hivemind-gw/main.go — load config, start health checker, register proxy/metrics/ratelimit handlers, listen on gateway port + admin port + metrics port. Graceful shutdown on SIGTERM/SIGINT. Log startup banner with config summary.

## Files to modify
- cmd/hivemind-gw/main.go

## Acceptance criteria
- [ ] Gateway starts and listens on configured ports
- [ ] Graceful shutdown works
- [ ] Startup banner shows loaded config
- [ ] SIGHUP reloads config

## Rules
- cd into /home/pook/ralph/hivemind before any file operations
- Read each file BEFORE editing — understand existing code first
- Do NOT add new dependencies without explicit need
- Do NOT refactor surrounding code — surgical changes only
- Run typecheck/build after changes if available

## Verify
```bash
cd /home/pook/ralph/hivemind && cd /home/pook/ralph/hivemind && go build ./cmd/hivemind-gw && ./hivemind-gw --config config.example.toml --dry-run 2>/dev/null; echo $?
```

## Commit
```
feat: [W1-S5] Gateway main + server wiring
```
