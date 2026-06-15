# W6-S2: Smoke test script

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: W4-S1, W4-S2

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Create scripts/smoke-test.sh — end-to-end validation against live BatKave: start gateway with batkave.toml, test /v1/chat/completions with Qwen model, test /health, test /metrics, test /admin/usage, test PII scan (send fake SSN), test rate limit. Exit 0 if all pass, 1 with details if any fail.

## Files to modify
- scripts/smoke-test.sh

## Acceptance criteria
- [ ] Script runs end-to-end against live system
- [ ] Tests all critical paths
- [ ] Clear pass/fail output
- [ ] Non-destructive (read-only where possible)

## Rules
- cd into /home/pook/ralph/hivemind before any file operations
- Read each file BEFORE editing — understand existing code first
- Do NOT add new dependencies without explicit need
- Do NOT refactor surrounding code — surgical changes only
- Run typecheck/build after changes if available

## Verify
```bash
cd /home/pook/ralph/hivemind && test -x /home/pook/ralph/hivemind/scripts/smoke-test.sh || chmod +x /home/pook/ralph/hivemind/scripts/smoke-test.sh
```

## Commit
```
feat: [W6-S2] Smoke test script
```
