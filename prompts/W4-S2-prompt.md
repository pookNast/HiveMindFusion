# W4-S2: Production config.toml

## Context
- Repo: /home/pook/ralph/hivemind
- Working directory: /home/pook/ralph/hivemind
- Dependencies: W0-S2

## Stack (MANDATORY — do not deviate)
- Runtime: Go

## Do
Create /home/pook/ralph/hivemind/config/batkave.toml — production config for BatKave: TurboQuant on :11434 as primary, Ollama on :11435 as secondary, VPS fallback via Headscale. PII Shield on :5100. Consumer entries for: openclaude, ralph-swarm, siteops, contractpilot, compliancebot, magicdocs. Rate limits: openclaude=unlimited, swarm=60rpm, products=120rpm.

## Files to modify
- config/batkave.toml

## Acceptance criteria
- [ ] All backends configured with health endpoints
- [ ] All consumers defined with rate limits
- [ ] PII Shield configured with circuit breaker
- [ ] Qdrant endpoint configured

## Rules
- cd into /home/pook/ralph/hivemind before any file operations
- Read each file BEFORE editing — understand existing code first
- Do NOT add new dependencies without explicit need
- Do NOT refactor surrounding code — surgical changes only
- Run typecheck/build after changes if available

## Verify
```bash
cd /home/pook/ralph/hivemind && test -f /home/pook/ralph/hivemind/config/batkave.toml
```

## Commit
```
feat: [W4-S2] Production config.toml
```
