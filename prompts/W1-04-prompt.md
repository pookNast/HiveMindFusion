# W1-04: Add HIVEMIND_URL to mega-chief.sh

## Repo
/home/pook/ralph/hivemind

## Goal
Export HIVEMIND_URL in mega-chief.sh orchestrator environment so child agents inherit it.

## Files to Edit
- `ralph/rlm-engine/launchers/mega-chief.sh` — env export section around lines 59-63

## Acceptance Criteria
- HIVEMIND_URL exported with default http://127.0.0.1:8400/v1
- Placed alongside existing RLM_PROMPT_DIR, RLM_LAUNCHER_DIR exports

## Verification Command
```bash
grep -c 'HIVEMIND_URL' ralph/rlm-engine/launchers/mega-chief.sh | grep -q '[1-9]' && echo PASS || echo FAIL
```

## Output
Write results to `logs/W1-04.log`.

## Blocker Format
BLOCKER: <description> | ITEM: W1-04 | SEVERITY: blocking|non-blocking
