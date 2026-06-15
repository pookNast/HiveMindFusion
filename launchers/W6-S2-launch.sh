#!/usr/bin/env bash
# W6-S2-launch.sh — standalone agent launcher for Smoke test script
set -uo pipefail
source ~/.bashrc 2>/dev/null || true
source ~/.cargo/env 2>/dev/null || true
export PATH="$HOME/.npm-global/bin:$HOME/.bun/bin:$HOME/.local/bin:$PATH"

cd "/home/pook/ralph/hivemind" || exit 1

TASK_ID="W6-S2"
PROMPT="/home/pook/ralph/hivemind/prompts/W6-S2-prompt.md"
LOG="/home/pook/ralph/hivemind/logs/W6-S2.log"

echo "[$( date -Iseconds )] Launching agent $TASK_ID in /home/pook/ralph/hivemind"
cat "$PROMPT" | claude --print \
  --allowedTools 'Edit,Write,Read,Bash(*)' \
  --model sonnet \
  --max-turns 60 \
  --mcp-config '{"mcpServers":{}}' \
  --strict-mcp-config \
  2>&1 | tee "$LOG"
EXIT_CODE=$?
echo "[$( date -Iseconds )] Agent $TASK_ID finished (exit $EXIT_CODE)"
exit $EXIT_CODE
