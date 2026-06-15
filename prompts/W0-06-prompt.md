# W0-06: Verify rlm-swarm RAG config alignment

## Repo
/home/pook/ralph/hivemind

## Goal
Check if existing `ralph-swarm` RAG config should be renamed to `rlm-swarm` or if a separate rlm-swarm RAG entry is needed. Ensure naming consistency.

## Files to Inspect/Edit
- `config/batkave.toml` — look for `ralph-swarm` in RAG consumer overrides

## Acceptance Criteria
- rlm-swarm has RAG config (either renamed from ralph-swarm or newly added)
- No orphaned ralph-swarm references that should be rlm-swarm
- TOML parses cleanly

## Verification Command
```bash
python3 -c "import tomllib; c=tomllib.load(open('config/batkave.toml','rb')); print('PASS')"
```

## Output
Write results to `logs/W0-06.log` including what action was taken.

## Blocker Format
BLOCKER: <description> | ITEM: W0-06 | SEVERITY: blocking|non-blocking
