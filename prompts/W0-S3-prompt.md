# W0-S3: VRAM calculator library

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: none

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Create internal/vram/calculator.go — estimate VRAM for a model given: file size on disk, quantization type (parsed from filename convention Q4_K_XL etc), context length, KV cache type (turbo3=4.6x, q8=2x, f16=1x), spec_draft_n_max. Formula: model_size + (context * heads * dim * kv_multiplier * 2) / compression + (draft_heads * overhead). Return estimate in MiB with fit/no-fit verdict for 24GB.

## Files to modify
- internal/vram/calculator.go
- internal/vram/calculator_test.go

## Acceptance criteria
- [ ] Qwen3.6-27B Q4_K_XL + 160K + turbo3 + 3 draft = ~20GB estimate
- [ ] Reports no-fit for n-max=6 at 160K
- [ ] Unit tests pass

## Rules
- cd into /home/pook/ralph/hivemind before any file operations
- Read each file BEFORE editing — understand existing code first
- Do NOT add new dependencies without explicit need
- Do NOT refactor surrounding code — surgical changes only
- Run typecheck/build after changes if available

## Verify
```bash
cd /home/pook/ralph/hivemind && cd /home/pook/ralph/hivemind && go test ./internal/vram/
```

## Commit
```
feat: [W0-S3] VRAM calculator library
```
