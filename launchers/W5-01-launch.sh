#!/usr/bin/env bash
# Launcher for W5-01
# Generated: 2026-06-07T11:59:57-04:00
set -euo pipefail

# Source environment
[[ -f ~/.bashrc ]] && source ~/.bashrc
[[ -f ~/.cargo/env ]] && source ~/.cargo/env

# Change to repo root
cd "/home/pook/ralph/hivemind"

# Execute via file-piped prompt
cat prompts/CONTEXT.md prompts/W5-01-prompt.md | claude --print --model sonnet 2>&1 | tee logs/W5-01.log

echo "--- W5-01 complete: $(date -Iseconds) ---" >> logs/W5-01.log
