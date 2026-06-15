# W0-05: Add RAG config for ceo-agent consumer

## Repo
/home/pook/ralph/hivemind

## Goal
Add RAG context injection config for ceo-agent consumer. Executive decisions benefit from full knowledge graph context.

## Settings
- collection = "knowledge_graph"
- top_k = 5
- min_score = 0.7

## Files to Edit
- `config/batkave.toml` — add under RAG consumer overrides section

## Acceptance Criteria
- ceo-agent RAG section exists with collection=knowledge_graph, top_k=5, min_score=0.7
- TOML parses cleanly

## Verification Command
```bash
grep -A4 'ceo-agent' config/batkave.toml | grep -q 'knowledge_graph' && echo PASS || echo FAIL
```

## Output
Write results to `logs/W0-05.log`.

## Blocker Format
BLOCKER: <description> | ITEM: W0-05 | SEVERITY: blocking|non-blocking
