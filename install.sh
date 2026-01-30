#!/bin/bash
#
# Stromboli Container Installer
# Usage: curl -sL https://raw.githubusercontent.com/tomblancdev/stromboli/main/install.sh | bash
#
# Supports both Docker and Podman
#

set -e

REPO="tomblancdev/stromboli"
BRANCH="main"
BASE_URL="https://raw.githubusercontent.com/${REPO}/${BRANCH}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Detected runtime
RUNTIME=""
COMPOSE_CMD=""

echo -e "${BLUE}"
echo "  🌋 Stromboli Installer"
echo "  ======================"
echo -e "${NC}"

# Check requirements
echo -e "${YELLOW}Checking requirements...${NC}"

# Check for Podman first (preferred for Stromboli)
if command -v podman &> /dev/null; then
    RUNTIME="podman"
    echo -e "${GREEN}✓${NC} Podman found"

    if command -v podman-compose &> /dev/null; then
        COMPOSE_CMD="podman-compose"
    elif podman compose version &> /dev/null 2>&1; then
        COMPOSE_CMD="podman compose"
    fi
    [ -n "$COMPOSE_CMD" ] && echo -e "${GREEN}✓${NC} Compose found"

# Fall back to Docker
elif command -v docker &> /dev/null; then
    RUNTIME="docker"
    echo -e "${GREEN}✓${NC} Docker found"

    if command -v docker compose &> /dev/null; then
        COMPOSE_CMD="docker compose"
    elif command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    fi
    [ -n "$COMPOSE_CMD" ] && echo -e "${GREEN}✓${NC} Compose found"
fi

# Validate
if [ -z "$RUNTIME" ]; then
    echo -e "${RED}❌ Neither Docker nor Podman found.${NC}"
    echo ""
    echo "  Install Podman: https://podman.io/getting-started/installation"
    echo "  Install Docker: https://docs.docker.com/get-docker/"
    exit 1
fi

if [ -z "$COMPOSE_CMD" ]; then
    echo -e "${RED}❌ No Compose tool found.${NC}"
    exit 1
fi

# Create installation directory
INSTALL_DIR="${1:-stromboli}"
echo ""
echo -e "${YELLOW}Installing to: ${INSTALL_DIR}/${NC}"

mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR"

# Download files
echo ""
echo -e "${YELLOW}Downloading files...${NC}"

curl -sL "${BASE_URL}/install/docker-compose.yml" -o docker-compose.yml
echo -e "${GREEN}✓${NC} docker-compose.yml"

curl -sL "${BASE_URL}/install/stromboli.example.yaml" -o stromboli.example.yaml
echo -e "${GREEN}✓${NC} stromboli.example.yaml"

# Summary
echo ""
echo -e "${GREEN}✅ Installation complete!${NC}"
echo ""
echo -e "${BLUE}Runtime: ${RUNTIME} | Compose: ${COMPOSE_CMD}${NC}"
echo ""
echo -e "${BLUE}Next steps:${NC}"
echo ""

if [ "$RUNTIME" = "podman" ]; then
    echo "  1. Enable Podman socket (if not already):"
    echo -e "     ${YELLOW}systemctl --user enable --now podman.socket${NC}"
    echo ""
    echo "  2. Start Stromboli:"
    echo -e "     ${YELLOW}${COMPOSE_CMD} up -d${NC}"
else
    echo "  1. Start Stromboli:"
    echo -e "     ${YELLOW}${COMPOSE_CMD} up -d${NC}"
fi

echo ""
echo "  Check health:"
echo -e "     ${YELLOW}curl http://localhost:8080/health${NC}"
echo ""
echo -e "${BLUE}Documentation:${NC} https://tomblancdev.github.io/stromboli"
echo ""
