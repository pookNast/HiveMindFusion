# W1-05: Add hivemind health check to rlm-preflight.sh

## Repo
/home/pook/ralph/hivemind

## Goal
Add hivemind gateway health check to rlm-preflight.sh. Warn (non-blocking) if :8400 is unreachable — RLM can still work without hivemind via direct API.

## Files to Edit
- `ralph/rlm-engine/launchers/rlm-preflight.sh` — health check section

## Acceptance Criteria
- Hivemind health check added: curl -sf http://127.0.0.1:8400/v1/models
- Warning printed if unreachable (not a blocker — RLM falls back to direct)
- Fits existing health check pattern in the script

## Verification Command
```bash
grep -c '8400\|hivemind' ralph/rlm-engine/launchers/rlm-preflight.sh | grep -q '[1-9]' && echo PASS || echo FAIL
```

## Output
Write results to `logs/W1-05.log`.

## Blocker Format
BLOCKER: <description> | ITEM: W1-05 | SEVERITY: blocking|non-blocking
