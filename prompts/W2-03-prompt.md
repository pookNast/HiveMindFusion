# W2-03: Remove redundant PII call from CEO-Agent

## Repo
~/project-incubator/ceo-agent

## Goal
Remove CEO-Agent's own PII scanning (PIIFilterAdapter wrapping Ollama). Hivemind already does PII scanning inline — double-scanning wastes compute.

## Files to Edit
- `cmd/ceo-agent/main.go` — remove PIIFilterAdapter wrapping in registerAdapters()
- `internal/llm/pii_filter.go` — can be left in place but unused (or removed if no other references)

## Acceptance Criteria
- Ollama adapter registered WITHOUT PIIFilterAdapter wrapper
- CEO-Agent no longer calls PII Shield directly (100.64.0.3:5000)
- CEO_AGENT_PII_URL env var no longer used in adapter registration
- Cloud adapters (Anthropic, OpenRouter) unchanged — they don't go through hivemind

## Verification Command
```bash
grep -c 'PIIFilter\|pii_filter\|anonymize' ~/project-incubator/ceo-agent/cmd/ceo-agent/main.go | grep -q '^0$' && echo PASS || echo FAIL
```

## Output
Write results to `logs/W2-03.log`.

## Blocker Format
BLOCKER: <description> | ITEM: W2-03 | SEVERITY: blocking|non-blocking
