# W0-03: Add rlm-swarm consumer to batkave.toml

## Repo
/home/pook/ralph/hivemind

## Goal
Add `rlm-swarm` consumer to rate_limits section in batkave.toml. Settings: 120 req/min, burst 40. This is the highest-throughput consumer (swarm agents).

## Files to Edit
- `config/batkave.toml` — add under `[rate_limits.consumers]` section

## Acceptance Criteria
- `rlm-swarm` consumer entry exists with requests_per_minute = 120, burst = 40
- TOML still parses cleanly

## Verification Command
```bash
grep -A2 'rlm-swarm' config/batkave.toml | grep -q '120' && echo PASS || echo FAIL
```

## Output
Write results to `logs/W0-03.log`.

## Blocker Format
BLOCKER: <description> | ITEM: W0-03 | SEVERITY: blocking|non-blocking
