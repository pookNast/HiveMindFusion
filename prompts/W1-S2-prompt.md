# W1-S2: Backend health checker

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: W0-S1, W0-S2

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Create internal/gateway/health.go — periodic health checks against each backend's health_endpoint. Tracks state (healthy/unhealthy/unknown) with configurable interval and failure threshold. Unhealthy backends excluded from routing. Exposes GET /health on admin port with all backend states + VRAM usage.

## Files to modify
- internal/gateway/health.go

## Acceptance criteria
- [ ] Health check runs on configurable interval
- [ ] Unhealthy backend auto-excluded from routing
- [ ] GET /health returns JSON with all backend states
- [ ] VRAM usage included via NVML or nvidia-smi parsing

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
feat: [W1-S2] Backend health checker
```
