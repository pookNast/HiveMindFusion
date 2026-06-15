# W3-01: Commit all hivemind changes

## Repo
/home/pook/ralph/hivemind

## Goal
Stage and commit all modified tracked files: config/batkave.toml, internal/config/config.go, internal/gateway/proxy.go, plus any new changes from W0-W2.

## Files to Stage
- `config/batkave.toml`
- `internal/config/config.go`
- `internal/gateway/proxy.go`
- Any other modified tracked files in the hivemind repo

## Acceptance Criteria
- All hivemind changes committed with descriptive message
- git diff --name-only returns empty for tracked files
- Push to Forgejo remote

## Verification Command
```bash
cd ~/ralph/hivemind && git diff --name-only | wc -l | grep -q '^0$' && echo PASS || echo FAIL
```

## Output
Write commit hash and summary to `logs/W3-01.log`.

## Blocker Format
BLOCKER: <description> | ITEM: W3-01 | SEVERITY: blocking|non-blocking
