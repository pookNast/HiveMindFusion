# W0-04: Add ceo-agent consumer to batkave.toml

## Repo
/home/pook/ralph/hivemind

## Goal
Add `ceo-agent` consumer to rate_limits section. Settings: 30 req/min, burst 10. Low-volume executive decision consumer.

## Files to Edit
- `config/batkave.toml` — add under `[rate_limits.consumers]` section

## Acceptance Criteria
- `ceo-agent` consumer entry exists with requests_per_minute = 30, burst = 10
- TOML still parses cleanly

## Verification Command
```bash
grep -A2 'ceo-agent' config/batkave.toml | grep -q '30' && echo PASS || echo FAIL
```

## Output
Write results to `logs/W0-04.log`.

## Blocker Format
BLOCKER: <description> | ITEM: W0-04 | SEVERITY: blocking|non-blocking
