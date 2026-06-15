# W4-03: Validate RLM engine routing

## Repo
/home/pook/ralph/hivemind

## Goal
Run a test RLM agent with RLM_MODEL_OVERRIDE=hivemind and verify rlm-swarm consumer appears in Prometheus metrics.

## Acceptance Criteria
- rlm-swarm consumer label appears in hivemind_requests_total metric

## Verification Command
```bash
curl -s http://127.0.0.1:8402/metrics | grep -q 'consumer="rlm-swarm"' && echo PASS || echo FAIL
```

## Output
Write metrics output to `logs/W4-03.log`.

## Blocker Format
BLOCKER: <description> | ITEM: W4-03 | SEVERITY: blocking
