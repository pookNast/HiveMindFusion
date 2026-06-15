# W4-02: Validate departmental agent routing

## Repo
/home/pook/ralph/hivemind

## Goal
Run a test departmental agent (legal NDA triage or compliance) and verify it appears in hivemind metrics.

## Acceptance Criteria
- hivemind_requests_total metric shows departmental consumer activity

## Verification Command
```bash
curl -s http://127.0.0.1:8402/metrics | grep -q 'hivemind_requests_total' && echo PASS || echo FAIL
```

## Output
Write metrics output to `logs/W4-02.log`.

## Blocker Format
BLOCKER: <description> | ITEM: W4-02 | SEVERITY: blocking|non-blocking
