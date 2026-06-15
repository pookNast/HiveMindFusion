# W2-02: Update OLLAMA_BASE_URL default in CEO-Agent

## Repo
~/project-incubator/ceo-agent

## Goal
Change the default OLLAMA_BASE_URL from http://localhost:11434 to http://localhost:8400/v1 in the CEO-Agent config and env sample.

## Files to Edit
- `cmd/ceo-agent/main.go` — default URL constant or env fallback
- `.env.sample` — update OLLAMA_BASE_URL example

## Acceptance Criteria
- Default URL points to hivemind :8400
- .env.sample updated with new default
- Existing env var override still works (users can set OLLAMA_BASE_URL to bypass hivemind)

## Verification Command
```bash
grep -c '8400' ~/project-incubator/ceo-agent/cmd/ceo-agent/main.go | grep -q '[1-9]' && echo PASS || echo FAIL
```

## Output
Write results to `logs/W2-02.log`.

## Blocker Format
BLOCKER: <description> | ITEM: W2-02 | SEVERITY: blocking|non-blocking
