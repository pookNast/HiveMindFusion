# W5-02: Collect confidence gates

## Repo
/home/pook/ralph/hivemind

## Goal
Gather confidence scores from 6 independent evaluator agents (dev, architect, siteops, security, product, QA). Each reviews evidence and reports a confidence score.

## Output Format
Write `logs/confidence-gates.json`:
```json
{
  "dev": 0.XX,
  "architect": 0.XX,
  "siteops": 0.XX,
  "security": 0.XX,
  "product": 0.XX,
  "qa": 0.XX
}
```

## Acceptance Criteria
- All 6 roles report confidence >= 0.95
- Each score backed by evidence review (not self-assessed)

## Verification Command
```bash
python3 -c "import json; g=json.load(open('logs/confidence-gates.json')); assert all(v>=0.95 for v in g.values()), f'Low: {g}'; print('PASS')"
```

## Blocker Format
BLOCKER: <description> | ITEM: W5-02 | SEVERITY: blocking
