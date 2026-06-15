# W1-02: Wire Toolmem research through hivemind

## Repo
/home/pook/ralph/hivemind

## Goal
Change recall/research queries in agent-launch.sh (lines 27-82) to route through hivemind :8400 when available, with fallback to direct Toolmem URL.

## Files to Edit
- `ralph/rlm-engine/launchers/agent-launch.sh` — Toolmem recall logic around lines 27-82

## Acceptance Criteria
- HIVEMIND_URL check: if hivemind healthy, use :8400/v1 for recall inference
- Fallback: if hivemind down, use direct TOOLMEM_URL (existing behavior)
- No breaking change to existing Toolmem integration

## Verification Command
```bash
grep -c 'HIVEMIND_URL\|8400' ralph/rlm-engine/launchers/agent-launch.sh | grep -q '[2-9]' && echo PASS || echo FAIL
```

## Output
Write results to `logs/W1-02.log`.

## Blocker Format
BLOCKER: <description> | ITEM: W1-02 | SEVERITY: blocking|non-blocking
