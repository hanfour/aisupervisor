#!/bin/bash
set -e

# Start tmux server (required for workers)
tmux start-server

# Set up git defaults if not configured
if [ -z "$(git config --global user.name 2>/dev/null)" ]; then
  git config --global user.name "aisupervisor-docker"
  git config --global user.email "aisupervisor@docker.local"
fi

# Trust all mounted repos
git config --global --add safe.directory '*'

# Display environment info
echo "=== AI Supervisor Docker Environment ==="
echo "Claude Code: $(claude --version 2>/dev/null || echo 'not found')"
echo "tmux: $(tmux -V)"
echo "git: $(git --version)"
echo "node: $(node --version)"
echo "OpenAI backend: gpt-4o-mini"
if [ -n "$OPENAI_API_KEY" ]; then
  echo "OPENAI_API_KEY: set (${#OPENAI_API_KEY} chars)"
else
  echo "OPENAI_API_KEY: NOT SET"
fi
echo "========================================="

# Execute passed command, or default to aisupervisor company
exec "${@:-aisupervisor company}"
