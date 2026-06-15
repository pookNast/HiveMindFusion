# W4-04: Validate CEO-Agent routing

## Repo
~/project-incubator/ceo-agent

## Goal
Start CEO-Agent, trigger a decision, and verify ceo-agent consumer appears in hivemind Prometheus metrics.

## Acceptance Criteria
- ceo-agent consumer label appears in hivemind_requests_total metric

## Verification Command
```bash
curl -s http://127.0.0.1:8402/metrics | grep -q 'consumer="ceo-agent"' && echo PASS || echo FAIL
```

## Output
Write metrics output to `logs/W4-04.log`.

## Blocker Format
BLOCKER: <description> | ITEM: W4-04 | SEVERITY: blocking
