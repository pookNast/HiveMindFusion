# W1-01: Add hivemind routing to agent-launch.sh

## Repo
/home/pook/ralph/hivemind

## Goal
Add hivemind routing option to agent-launch.sh. When RLM_MODEL_OVERRIDE=local or RLM_MODEL_OVERRIDE=hivemind, set OPENAI_BASE_URL=http://127.0.0.1:8400/v1 and add X-HiveMind-Consumer: rlm-swarm header.

## Files to Edit
- `ralph/rlm-engine/launchers/agent-launch.sh` — model selection logic around lines 195-211

## Acceptance Criteria
- New case in model selection: hivemind|local routes through :8400
- X-HiveMind-Consumer header set to rlm-swarm
- Existing fallback chains (chatgpt, glm, direct) unchanged
- Health check for :8400 before routing (5s timeout, like existing fallbacks)

## Verification Command
```bash
grep -c 'HIVEMIND\|8400' ralph/rlm-engine/launchers/agent-launch.sh | grep -q '[1-9]' && echo PASS || echo FAIL
```

## Output
Write results to `logs/W1-01.log`.

## Blocker Format
BLOCKER: <description> | ITEM: W1-01 | SEVERITY: blocking|non-blocking
