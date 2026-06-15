# W0-S1: Go module + project structure

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: none

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Initialize Go module, create cmd/hivemind-gw, cmd/hivemind-ctl, internal/config, internal/gateway, internal/pii, internal/vram, internal/models directories. Add go.mod with BurntSushi/toml, prometheus/client_golang deps.

## Files to modify
- go.mod
- go.sum
- cmd/hivemind-gw/main.go
- cmd/hivemind-ctl/main.go

## Acceptance criteria
- [ ] go mod tidy succeeds
- [ ] go build ./cmd/hivemind-gw compiles
- [ ] go build ./cmd/hivemind-ctl compiles

## Rules
- cd into /home/pook/ralph/hivemind before any file operations
- Read each file BEFORE editing — understand existing code first
- Do NOT add new dependencies without explicit need
- Do NOT refactor surrounding code — surgical changes only
- Run typecheck/build after changes if available

## Verify
```bash
cd /home/pook/ralph/hivemind && cd /home/pook/ralph/hivemind && go build ./cmd/hivemind-gw && go build ./cmd/hivemind-ctl
```

## Commit
```
feat: [W0-S1] Go module + project structure
```
