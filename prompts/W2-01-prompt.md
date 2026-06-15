# W2-01: Modify registerAdapters() for hivemind routing

## Repo
~/project-incubator/ceo-agent

## Goal
In cmd/ceo-agent/main.go, modify registerAdapters() to route the Ollama adapter through hivemind :8400 instead of direct :11434. Add X-HiveMind-Consumer: ceo-agent header.

## Files to Edit
- `cmd/ceo-agent/main.go` — registerAdapters() function around line 313-345

## Acceptance Criteria
- Ollama adapter URL changed from :11434 to :8400
- X-HiveMind-Consumer header set to "ceo-agent"
- Anthropic and OpenRouter adapters UNCHANGED
- ChatGPT adapter UNCHANGED

## Verification Command
```bash
grep -c '8400\|HiveMind' ~/project-incubator/ceo-agent/cmd/ceo-agent/main.go | grep -q '[1-9]' && echo PASS || echo FAIL
```

## Output
Write results to `logs/W2-01.log`.

## Blocker Format
BLOCKER: <description> | ITEM: W2-01 | SEVERITY: blocking|non-blocking
