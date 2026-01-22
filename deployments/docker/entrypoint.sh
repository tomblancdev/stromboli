#!/bin/bash
set -e

# Read token from mounted secrets file if it exists
if [ -f /run/secrets/claude-token ]; then
    export CLAUDE_CODE_OAUTH_TOKEN=$(cat /run/secrets/claude-token | tr -d '\n')
fi

# Execute claude with all arguments
exec claude "$@"
