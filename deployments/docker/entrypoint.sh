#!/bin/bash
set -e

# Credentials file is mounted directly at ~/.claude/.credentials.json
# Claude will read it automatically - no token export needed

# Ensure home directory structure exists and is writable by current user
# This handles --userns=keep-id scenarios where we may run as different UID
HOME_DIR="${HOME:-/home/claude}"

# Create required Claude directories if they don't exist
mkdir -p "$HOME_DIR/.claude" 2>/dev/null || true

# Execute claude with all arguments
exec claude "$@"
