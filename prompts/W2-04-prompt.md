# W2-04: Keep cloud fallbacks unchanged

## Repo
~/project-incubator/ceo-agent

## Goal
Verify that Anthropic and OpenRouter adapters are untouched after W2-01/W2-02/W2-03 changes. Cloud calls must remain direct — they don't route through hivemind.

## Files to Inspect
- `cmd/ceo-agent/main.go` — registerAdapters() and configureFallbackChains()

## Acceptance Criteria
- NewAnthropicAdapter() call unchanged
- NewOpenRouterAdapter() call unchanged
- Fallback chain still includes anthropic and openrouter as cloud fallbacks
- No :8400 references in cloud adapter code

## Verification Command
```bash
grep -c 'NewAnthropicAdapter\|NewOpenRouterAdapter' ~/project-incubator/ceo-agent/cmd/ceo-agent/main.go | grep -q '^2$' && echo PASS || echo FAIL
```

## Output
Write results to `logs/W2-04.log`.

## Blocker Format
BLOCKER: <description> | ITEM: W2-04 | SEVERITY: blocking|non-blocking
