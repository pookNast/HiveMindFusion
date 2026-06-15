#!/usr/bin/env bash
# Launcher for W4-04
# Generated: 2026-06-07T11:59:57-04:00
set -euo pipefail

# Source environment
[[ -f ~/.bashrc ]] && source ~/.bashrc
[[ -f ~/.cargo/env ]] && source ~/.cargo/env

# Change to repo root
cd "/home/pook/ralph/hivemind"

# Execute via file-piped prompt
cat prompts/CONTEXT.md prompts/W4-04-prompt.md | claude --print --model sonnet 2>&1 | tee logs/W4-04.log

echo "--- W4-04 complete: $(date -Iseconds) ---" >> logs/W4-04.log
