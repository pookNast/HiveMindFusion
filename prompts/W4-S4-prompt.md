# W4-S4: Grafana dashboard JSON

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: W1-S3

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Create config/grafana-hivemind.json — Grafana dashboard with panels: Request rate by consumer, Latency p50/p99, Tokens/sec, VRAM usage gauge, Backend health status, PII scan rate, Rate limit rejections. Auto-provisionable via Grafana dashboard API.

## Files to modify
- config/grafana-hivemind.json

## Acceptance criteria
- [ ] Valid Grafana dashboard JSON
- [ ] All Prometheus metrics have panels
- [ ] Dashboard imports without error

## Rules
- cd into /home/pook/ralph/hivemind before any file operations
- Read each file BEFORE editing — understand existing code first
- Do NOT add new dependencies without explicit need
- Do NOT refactor surrounding code — surgical changes only
- Run typecheck/build after changes if available

## Verify
```bash
cd /home/pook/ralph/hivemind && python3 -c "import json; json.load(open('/home/pook/ralph/hivemind/config/grafana-hivemind.json'))" && echo valid
```

## Commit
```
feat: [W4-S4] Grafana dashboard JSON
```
