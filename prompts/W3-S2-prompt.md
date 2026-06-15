# W3-S2: hivemind-ctl models load/unload/swap

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: W0-S3, W3-S1

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Implement `hivemind-ctl models load <name> [--context N] [--spec-n-max N]` — generates and executes llama-server launch command with correct params (derived from model metadata + config). Manages PID file. `unload` gracefully stops. `swap <name>` does unload→load atomically. Validates VRAM before load (reject if won't fit).

## Files to modify
- internal/models/lifecycle.go
- internal/models/launcher.go

## Acceptance criteria
- [ ] Load starts llama-server with correct params
- [ ] Unload gracefully kills server
- [ ] Swap is atomic (unload then load)
- [ ] VRAM check prevents OOM loads
- [ ] PID file managed correctly

## Rules
- cd into /home/pook/ralph/hivemind before any file operations
- Read each file BEFORE editing — understand existing code first
- Do NOT add new dependencies without explicit need
- Do NOT refactor surrounding code — surgical changes only
- Run typecheck/build after changes if available

## Verify
```bash
cd /home/pook/ralph/hivemind && cd /home/pook/ralph/hivemind && go vet ./internal/models/
```

## Commit
```
feat: [W3-S2] hivemind-ctl models load/unload/swap
```
