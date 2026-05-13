#!/bin/bash
set -euo pipefail

# CloudLab boot-time setup for the monolift build server.
# Runs as root. Logs to /var/log/cloudlab-setup.log.

LOGFILE=/var/log/cloudlab-setup.log
exec > >(tee -a "$LOGFILE") 2>&1

GO_VERSION="1.26.0"
REPO_DIR="/local/repository"

echo "=== monolift build-server setup ($(date)) ==="

apt-get update
apt-get install -y build-essential git curl wget jq htop

# Go
if ! command -v go &>/dev/null || ! go version | grep -q "go${GO_VERSION}"; then
    echo "Installing Go ${GO_VERSION}..."
    wget -q "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -O /tmp/go.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
fi

cat > /etc/profile.d/golang.sh <<'EOF'
export PATH=/usr/local/go/bin:$HOME/go/bin:$PATH
export GOPATH=$HOME/go
EOF
source /etc/profile.d/golang.sh

echo "Go: $(go version)"

# Docker
if ! command -v docker &>/dev/null; then
    echo "Installing Docker..."
    curl -fsSL https://get.docker.com | sh
    usermod -aG docker "$(logname 2>/dev/null || echo ubuntu)"
fi

echo "Docker: $(docker --version)"

# Pre-fetch Go module cache
if [ -d "${REPO_DIR}" ]; then
    echo "Downloading Go modules..."
    cd "${REPO_DIR}"
    /usr/local/go/bin/go mod download
fi

echo "=== setup complete ($(date)) ==="
