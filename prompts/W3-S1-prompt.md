# W3-S1: hivemind-ctl models list/info

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: W0-S1, W0-S3

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Implement `hivemind-ctl models list` — scan HIVEMIND_MODEL_DIR for *.gguf files, parse filename for model name/quant/size, show file size, estimate VRAM with calculator. `hivemind-ctl models info <name>` shows detailed info including GGUF metadata (read header). Table output with columns: Name, Quant, Size, VRAM Est, Fits 24GB.

## Files to modify
- cmd/hivemind-ctl/main.go
- internal/models/scanner.go
- internal/models/gguf.go

## Acceptance criteria
- [ ] Lists all GGUFs in model dir
- [ ] Parses quant type from filename
- [ ] VRAM estimate shown per model
- [ ] Fit/no-fit indicator for 24GB

## Rules
- cd into /home/pook/ralph/hivemind before any file operations
- Read each file BEFORE editing — understand existing code first
- Do NOT add new dependencies without explicit need
- Do NOT refactor surrounding code — surgical changes only
- Run typecheck/build after changes if available

## Verify
```bash
cd /home/pook/ralph/hivemind && go build ./cmd/hivemind-ctl && ./hivemind-ctl models list --dir /mnt/olympus/LLMs/gguf/
```

## Commit
```
feat: [W3-S1] hivemind-ctl models list/info
```
