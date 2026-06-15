# W1-S4: Rate limiter middleware

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: W0-S1, W0-S2

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Create internal/gateway/ratelimit.go — per-consumer token bucket rate limiter. Consumer identified by X-HiveMind-Consumer header or API key. Configurable in config.toml per consumer (requests_per_minute, burst). Default limit for unidentified consumers. Returns 429 with Retry-After header.

## Files to modify
- internal/gateway/ratelimit.go

## Acceptance criteria
- [ ] Rate limit enforced per consumer
- [ ] 429 returned with correct Retry-After
- [ ] Default limit applies to unknown consumers
- [ ] Config reload updates limits without restart

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
feat: [W1-S4] Rate limiter middleware
```
