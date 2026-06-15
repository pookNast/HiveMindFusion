#!/usr/bin/env bash
# Launcher for W3-03
# Generated: 2026-06-07T11:59:57-04:00
set -euo pipefail

# Source environment
[[ -f ~/.bashrc ]] && source ~/.bashrc
[[ -f ~/.cargo/env ]] && source ~/.cargo/env

# Change to repo root
cd "/home/pook/ralph/hivemind"

# Execute via file-piped prompt
cat prompts/CONTEXT.md prompts/W3-03-prompt.md | claude --print --model sonnet 2>&1 | tee logs/W3-03.log

echo "--- W3-03 complete: $(date -Iseconds) ---" >> logs/W3-03.log
