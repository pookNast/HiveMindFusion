# W5-03: Final SHIP/NO-SHIP decision

## Repo
/home/pook/ralph/hivemind

## Goal
Independent evaluator reviews ALL evidence and renders final verdict. Must check:
1. All 10 MUST criteria in MILESTONE_CONTRACT.md have PASS evidence
2. All 6 confidence gates >= 0.95
3. No unresolved blocking PRD items
4. Build + test artifacts exist

## Output
Write `logs/final-ship-decision.md` containing:
- VERDICT: SHIP or VERDICT: NO-SHIP
- Evidence summary per MUST criterion
- Confidence gate summary
- Residual risks
- Timestamp

## Acceptance Criteria
- final-ship-decision.md contains VERDICT: SHIP (or NO-SHIP with blockers)
- All evidence referenced, not self-assessed

## Verification Command
```bash
grep -q 'VERDICT: SHIP' logs/final-ship-decision.md && echo PASS || echo FAIL
```

## Blocker Format
BLOCKER: <description> | ITEM: W5-03 | SEVERITY: blocking
