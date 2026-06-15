# W1-S1: OpenAI-compatible proxy handler

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: W0-S1, W0-S2

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Create internal/gateway/proxy.go — reverse proxy handler for /v1/chat/completions, /v1/models, /v1/completions. Routes to configured backend based on 'model' field in request body. Supports streaming (SSE pass-through). Adds X-HiveMind-Backend response header. Connection pooling with configurable timeouts.

## Files to modify
- internal/gateway/proxy.go
- internal/gateway/router.go

## Acceptance criteria
- [ ] POST /v1/chat/completions proxies to backend
- [ ] model field routes to correct backend
- [ ] SSE streaming works
- [ ] Unknown model returns 404 with available models list

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
feat: [W1-S1] OpenAI-compatible proxy handler
```
