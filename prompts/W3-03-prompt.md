# W3-03: Restart gateway and verify ports

## Repo
/home/pook/ralph/hivemind

## Goal
Run restart.sh to deploy new binary and config. Verify all 3 ports respond.

## Commands
```bash
cd ~/ralph/hivemind && bash restart.sh
```

## Acceptance Criteria
- :8400 responds (proxy — models endpoint)
- :8401 responds (admin — health endpoint)
- :8402 responds (metrics — Prometheus)
- New consumers visible in config

## Verification Command
```bash
curl -sf http://127.0.0.1:8400/v1/models > /dev/null && curl -sf http://127.0.0.1:8401/health > /dev/null && curl -sf http://127.0.0.1:8402/metrics > /dev/null && echo PASS || echo FAIL
```

## Output
Write health response and port verification to `logs/W3-03.log`.

## Blocker Format
BLOCKER: <description> | ITEM: W3-03 | SEVERITY: blocking
