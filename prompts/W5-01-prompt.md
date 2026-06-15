# W5-01: Run all audits

## Repo
/home/pook/ralph/hivemind

## Goal
Execute all required audits and write reports to logs/audit-specs/. Required audits:
1. preflight — rlm-preflight.sh passes
2. security — PII fail-closed, no credential exposure, rate limits active
3. performance — p99 latency, backend response times
4. data-integrity — Qdrant RAG data consistent
5. dependency — no new unreviewed deps
6. rollback — rollback procedure documented and tested
7. siteops — services healthy, ports responding, logs flowing
8. rlm-ready — PRD structure valid, prompts/launchers compliant
9. gap-analysis — delta between current and target state
10. premortem — what could go wrong after ship

## Acceptance Criteria
- All audit reports generated as .md files in logs/audit-specs/
- At least 10 audit files present

## Verification Command
```bash
ls logs/audit-specs/*.md 2>/dev/null | wc -l | grep -q '[6-9]\|1[0-9]' && echo PASS || echo FAIL
```

## Output
Summary of audit findings to `logs/W5-01.log`.

## Blocker Format
BLOCKER: <description> | ITEM: W5-01 | SEVERITY: blocking
