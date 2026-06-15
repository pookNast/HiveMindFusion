# W2-S3: Consumer auth + usage tracking

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: W1-S1, W1-S4

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Create internal/gateway/consumer.go — identify consumers via X-HiveMind-Consumer header or configurable API keys in config.toml. Track per-consumer usage: request count, prompt/completion tokens, last request time. Expose via admin API GET /admin/usage. Store in-memory with periodic JSON dump to /var/lib/hivemind/usage.json.

## Files to modify
- internal/gateway/consumer.go

## Acceptance criteria
- [ ] Consumer identified from header or API key
- [ ] Usage tracked per consumer
- [ ] GET /admin/usage returns JSON usage report
- [ ] Periodic persistence to disk

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
feat: [W2-S3] Consumer auth + usage tracking
```
