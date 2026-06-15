#!/usr/bin/env bash
# improve-dispatch.sh — route /improve stages through HiveMind fusion panels
# Usage: improve-dispatch.sh <stage> <prompt-file>
#   stage: recon | advisor | executor | reviewer
#   prompt-file: file containing the stage's input prompt (recon target, audit context, plan, diff)
set -euo pipefail

STAGE="${1:?usage: improve-dispatch.sh <stage> <prompt-file>}"
PROMPT_FILE="${2:?usage: improve-dispatch.sh <stage> <prompt-file>}"
GATEWAY="http://127.0.0.1:8400/v1/chat/completions"
PROMPTS_DIR="$(cd "$(dirname "$0")/.." && pwd)/prompts/improve"

case "$STAGE" in
  recon)    MODEL="hivemind/fusion-improve-recon-local" ;;
  advisor)  MODEL="hivemind/fusion-improve-advisor" ;;
  executor) MODEL="hivemind/fusion-improve-executor" ;;
  reviewer) MODEL="hivemind/fusion-improve-reviewer" ;;
  *) echo "ERROR: unknown stage '$STAGE' (use: recon|advisor|executor|reviewer)" >&2; exit 1 ;;
esac

# Build the user prompt: stage guidance + the actual input
STAGE_GUIDANCE="$(cat "$PROMPTS_DIR/$STAGE.md")"
USER_INPUT="$(cat "$PROMPT_FILE")"
COMBINED="$STAGE_GUIDANCE

---
# Input
$USER_INPUT"

# ponytail: single curl call, streaming output to stdout — upgrade: add retry + timeout per stage
# Escape for JSON via python
PAYLOAD=$(python3 -c "
import json, sys
print(json.dumps({
    'model': '$MODEL',
    'messages': [{'role': 'user', 'content': sys.stdin.read()}],
    'max_tokens': 4096
}))
" <<< "$COMBINED")

exec curl -sS --max-time 300 "$GATEWAY" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD"
