# W1-03: Add HIVEMIND_URL to gen-launchers.sh template

## Repo
/home/pook/ralph/hivemind

## Goal
Inject HIVEMIND_URL env var into the launcher template in gen-launchers.sh so all generated launchers inherit hivemind routing.

## Files to Edit
- `ralph/rlm-engine/launchers/gen-launchers.sh` — template expansion section

## Acceptance Criteria
- Generated launchers export HIVEMIND_URL=${HIVEMIND_URL:-http://127.0.0.1:8400/v1}
- Template variable injected alongside existing env vars

## Verification Command
```bash
grep -c 'HIVEMIND_URL' ralph/rlm-engine/launchers/gen-launchers.sh | grep -q '[1-9]' && echo PASS || echo FAIL
```

## Output
Write results to `logs/W1-03.log`.

## Blocker Format
BLOCKER: <description> | ITEM: W1-03 | SEVERITY: blocking|non-blocking
