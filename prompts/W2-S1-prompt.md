# W2-S1: PII middleware — request/response scanning

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: W1-S1

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Create internal/pii/middleware.go — HTTP middleware that intercepts request body, sends to PII Shield POST /scan (agent_id from X-HiveMind-Consumer, direction=input), blocks if decision=block, passes sanitized_text if decision=redact, passes original if decision=allow. Same for response body (direction=output). Circuit breaker: if PII Shield unreachable for >30s, follow bypass_on_failure config (default: block).

## Files to modify
- internal/pii/middleware.go
- internal/pii/client.go
- internal/pii/middleware_test.go

## Acceptance criteria
- [ ] Request with PII gets sanitized before reaching backend
- [ ] Response with PII gets sanitized before reaching client
- [ ] Circuit breaker activates after configured timeout
- [ ] bypass_on_failure=true allows requests when PII Shield down
- [ ] bypass_on_failure=false blocks all requests when PII Shield down

## Rules
- cd into /home/pook/ralph/hivemind before any file operations
- Read each file BEFORE editing — understand existing code first
- Do NOT add new dependencies without explicit need
- Do NOT refactor surrounding code — surgical changes only
- Run typecheck/build after changes if available

## Verify
```bash
cd /home/pook/ralph/hivemind && cd /home/pook/ralph/hivemind && go test ./internal/pii/
```

## Commit
```
feat: [W2-S1] PII middleware — request/response scanning
```
