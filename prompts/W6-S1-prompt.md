# W6-S1: Integration test suite

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: W2-S1, W2-S2, W2-S3

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Create tests/integration_test.go — starts gateway with test config, tests: proxy to mock backend, PII scan blocks sensitive request, PII scan cleans response, rate limit triggers 429, fallback when primary down, health endpoint, metrics endpoint, consumer tracking. Requires PII Shield running (skip if not available).

## Files to modify
- tests/integration_test.go
- tests/testdata/config.toml

## Acceptance criteria
- [ ] All integration tests pass
- [ ] Tests are idempotent
- [ ] PII tests skip gracefully if :5100 down
- [ ] Coverage >70% on gateway package

## Rules
- cd into /home/pook/ralph/hivemind before any file operations
- Read each file BEFORE editing — understand existing code first
- Do NOT add new dependencies without explicit need
- Do NOT refactor surrounding code — surgical changes only
- Run typecheck/build after changes if available

## Verify
```bash
cd /home/pook/ralph/hivemind && cd /home/pook/ralph/hivemind && go test ./tests/ -v -count=1 2>&1 | tail -5
```

## Commit
```
feat: [W6-S1] Integration test suite
```
