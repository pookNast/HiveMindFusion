# W4-S3: Migrate oc-start to use gateway

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: W1-S5

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Update /home/pook/bin/oc-start — change OPENAI_BASE_URL from http://127.0.0.1:11434/v1 to http://127.0.0.1:8400/v1. Add fallback: if :8400 not healthy, fall back to direct :11434. Add X-HiveMind-Consumer: openclaude header. Keep CLAUDE_CODE_PROVIDER_PROFILE_ENV_APPLIED=1.

## Files to modify
- /home/pook/bin/oc-start

## Acceptance criteria
- [ ] oc-start connects via :8400 gateway
- [ ] Fallback to :11434 if gateway down
- [ ] X-HiveMind-Consumer header set
- [ ] Existing functionality preserved

## Rules
- cd into /home/pook/ralph/hivemind before any file operations
- Read each file BEFORE editing — understand existing code first
- Do NOT add new dependencies without explicit need
- Do NOT refactor surrounding code — surgical changes only
- Run typecheck/build after changes if available

## Verify
```bash
cd /home/pook/ralph/hivemind && grep '8400' /home/pook/bin/oc-start
```

## Commit
```
feat: [W4-S3] Migrate oc-start to use gateway
```
