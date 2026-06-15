# W1-S3: Prometheus metrics exporter

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: W0-S1

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Create internal/gateway/metrics.go — Prometheus counters/histograms: hivemind_requests_total (labels: consumer, model, backend, status), hivemind_request_duration_seconds, hivemind_tokens_total (prompt/completion), hivemind_vram_usage_bytes, hivemind_backend_health (gauge). Serves /metrics on metrics_port.

## Files to modify
- internal/gateway/metrics.go

## Acceptance criteria
- [ ] All metrics registered without collision
- [ ] /metrics serves valid Prometheus exposition format
- [ ] Request middleware increments counters

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
feat: [W1-S3] Prometheus metrics exporter
```
