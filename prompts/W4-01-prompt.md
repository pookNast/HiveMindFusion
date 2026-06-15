# W4-01: Validate OpenClaude routing

## Repo
/home/pook/ralph/hivemind

## Goal
Verify OpenClaude still routes through hivemind after config changes. Send a test prompt and check Prometheus metrics.

## Acceptance Criteria
- openclaude consumer label appears in hivemind_requests_total metric

## Verification Command
```bash
curl -s http://127.0.0.1:8402/metrics | grep -q 'consumer="openclaude"' && echo PASS || echo FAIL
```

## Output
Write metrics output to `logs/W4-01.log`.

## Blocker Format
BLOCKER: <description> | ITEM: W4-01 | SEVERITY: blocking
