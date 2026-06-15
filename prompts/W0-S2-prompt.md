# W0-S2: TOML config loader

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: none

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Create internal/config/config.go — parse /etc/hivemind/config.toml with Gateway (port, admin_port, metrics_port), Backends (list of name/url/model/priority/health_endpoint), PII (endpoint, enabled, bypass_on_failure, timeout_ms), Models (dir, default), RateLimit (per_consumer map), Qdrant (endpoint) sections. Include config.example.toml.

## Files to modify
- internal/config/config.go
- config.example.toml

## Acceptance criteria
- [ ] Config struct covers all sections
- [ ] Loads from HIVEMIND_CONFIG env or default path
- [ ] Validates required fields
- [ ] Returns typed errors on bad config

## Rules
- cd into /home/pook/ralph/hivemind before any file operations
- Read each file BEFORE editing — understand existing code first
- Do NOT add new dependencies without explicit need
- Do NOT refactor surrounding code — surgical changes only
- Run typecheck/build after changes if available

## Verify
```bash
cd /home/pook/ralph/hivemind && cd /home/pook/ralph/hivemind && go vet ./internal/config/
```

## Commit
```
feat: [W0-S2] TOML config loader
```
