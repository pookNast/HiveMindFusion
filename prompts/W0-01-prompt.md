# W0-01: Diagnose ollama-secondary (:11435)

## Repo
/home/pook/ralph/hivemind

## Goal
Check if Ollama process is running on port 11435. If down, restart it. If intentionally stopped (VRAM conservation), document as expected-down in config.

## Files to Inspect
- `config/batkave.toml` — backend definition for ollama-secondary
- System process list for ollama on :11435

## Acceptance Criteria
- ollama-secondary responds to health check on :11435
- OR documented as intentional cold-standby in batkave.toml comments

## Verification Command
```bash
curl -sf http://127.0.0.1:11435/api/tags > /dev/null && echo PASS || echo FAIL
```

## Output
Write results to `logs/W0-01.log` with timestamp, action taken, and exit status.

## Blocker Format
BLOCKER: <description> | ITEM: W0-01 | SEVERITY: blocking|non-blocking
