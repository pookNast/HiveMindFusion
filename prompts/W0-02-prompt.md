# W0-02: Diagnose vps-fallback (100.64.0.2:11434)

## Repo
/home/pook/ralph/hivemind

## Goal
Check VPS Ollama via Headscale SSH. If down, restart. If VPS unavailable due to maintenance, mark backend as intentional-offline in config.

## Files to Inspect
- `config/batkave.toml` — vps-fallback backend definition
- SSH: `ssh vps 'ss -tlnp | grep 11434'`

## Acceptance Criteria
- VPS Ollama responds on 100.64.0.2:11434
- OR documented as intentional-offline in batkave.toml

## Verification Command
```bash
ssh vps 'ss -tlnp | grep 11434' && echo PASS || echo FAIL
```

## Output
Write results to `logs/W0-02.log` with timestamp, action taken, and exit status.

## Blocker Format
BLOCKER: <description> | ITEM: W0-02 | SEVERITY: blocking|non-blocking
