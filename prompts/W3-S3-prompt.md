# W3-S3: hivemind-ctl vram command

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: W0-S3

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Implement `hivemind-ctl vram status` — show current VRAM usage via NVML (call nvidia-smi or use go-nvml bindings). `hivemind-ctl vram estimate --model X --context Y [--kv turbo3] [--spec-n-max N]` — use calculator to predict VRAM. `hivemind-ctl vram budget` — show what fits given current free VRAM.

## Files to modify
- internal/vram/nvml.go
- cmd/hivemind-ctl/vram.go

## Acceptance criteria
- [ ] vram status shows current GPU memory
- [ ] vram estimate matches known values (Qwen3.6 Q4_K_XL + 160K + turbo3 ~20GB)
- [ ] vram budget lists models that fit in free VRAM

## Rules
- cd into /home/pook/ralph/hivemind before any file operations
- Read each file BEFORE editing — understand existing code first
- Do NOT add new dependencies without explicit need
- Do NOT refactor surrounding code — surgical changes only
- Run typecheck/build after changes if available

## Verify
```bash
cd /home/pook/ralph/hivemind && cd /home/pook/ralph/hivemind && go vet ./internal/vram/
```

## Commit
```
feat: [W3-S3] hivemind-ctl vram command
```
