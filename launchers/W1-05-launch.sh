#!/usr/bin/env bash
# Launcher for W1-05
# Generated: 2026-06-07T11:59:57-04:00
set -euo pipefail

# Source environment
[[ -f ~/.bashrc ]] && source ~/.bashrc
[[ -f ~/.cargo/env ]] && source ~/.cargo/env

# Change to repo root
cd "/home/pook/ralph/hivemind"

# Execute via file-piped prompt
cat prompts/CONTEXT.md prompts/W1-05-prompt.md | claude --print --model sonnet 2>&1 | tee logs/W1-05.log

echo "--- W1-05 complete: $(date -Iseconds) ---" >> logs/W1-05.log
