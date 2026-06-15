# W5-S2: Embedding ingestion endpoint

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: W5-S1

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Add POST /admin/ingest to admin API — accepts text/file, generates embedding via local model (Qwen-0.5B through Ollama /api/embeddings), stores in Qdrant collection. Supports batch ingestion. Metadata: source, timestamp, consumer.

## Files to modify
- internal/rag/ingest.go

## Acceptance criteria
- [ ] POST /admin/ingest stores embedding in Qdrant
- [ ] Batch mode processes multiple documents
- [ ] Metadata correctly attached
- [ ] Uses local embedding model (no cloud)

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
feat: [W5-S2] Embedding ingestion endpoint
```
