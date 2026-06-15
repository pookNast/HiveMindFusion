# W5-S1: RAG context injection middleware

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: W1-S1, W0-S2

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Create internal/rag/middleware.go — optional middleware that queries Qdrant for relevant context based on user message, injects as system message prefix. Configurable per-consumer: enabled/disabled, collection name, top_k, min_score. Uses existing Qdrant collections (knowledge_graph, obsidian_notes).

## Files to modify
- internal/rag/middleware.go
- internal/rag/qdrant.go

## Acceptance criteria
- [ ] Queries Qdrant with user message embedding
- [ ] Injects top_k results as system context
- [ ] Respects per-consumer config
- [ ] Disabled by default, opt-in

## Rules
- cd into /home/pook/ralph/hivemind before any file operations
- Read each file BEFORE editing — understand existing code first
- Do NOT add new dependencies without explicit need
- Do NOT refactor surrounding code — surgical changes only
- Run typecheck/build after changes if available

## Verify
```bash
cd /home/pook/ralph/hivemind && cd /home/pook/ralph/hivemind && go vet ./internal/rag/
```

## Commit
```
feat: [W5-S1] RAG context injection middleware
```
