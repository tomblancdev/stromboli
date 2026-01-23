#!/bin/bash
set -e

# Read token from mounted secrets file if it exists
if [ -f /run/secrets/claude-token ]; then
    export CLAUDE_CODE_OAUTH_TOKEN=$(cat /run/secrets/claude-token | tr -d '\n')
fi

# Ensure home directory structure exists and is writable by current user
# This handles --userns=keep-id scenarios where we may run as different UID
CURRENT_UID=$(id -u)
HOME_DIR="${HOME:-/home/claude}"

# Create required Claude directories if they don't exist
mkdir -p "$HOME_DIR/.claude" 2>/dev/null || true

# Execute claude with all arguments
exec claude "$@"
