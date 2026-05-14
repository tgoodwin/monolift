#!/bin/bash
set -euo pipefail

# CloudLab boot-time setup for the monolift build server.
# Runs as root. Logs to /var/log/cloudlab-setup.log.

LOGFILE=/var/log/cloudlab-setup.log
exec > >(tee -a "$LOGFILE") 2>&1

GO_VERSION="1.26.0"
KIND_VERSION="v0.31.0"
KUBECTL_VERSION="v1.34.0"
K9S_VERSION="v0.50.18"
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

# kind — required by `make e2e` for the Kind-backed harness.
if ! command -v kind &>/dev/null || ! kind --version | grep -q "${KIND_VERSION#v}"; then
    echo "Installing kind ${KIND_VERSION}..."
    curl -fsSL -o /usr/local/bin/kind \
        "https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-linux-amd64"
    chmod +x /usr/local/bin/kind
fi
echo "kind: $(kind --version)"

# kubectl — required by `make e2e`.
if ! command -v kubectl &>/dev/null || ! kubectl version --client=true --output=yaml 2>/dev/null | grep -q "${KUBECTL_VERSION}"; then
    echo "Installing kubectl ${KUBECTL_VERSION}..."
    curl -fsSL -o /usr/local/bin/kubectl \
        "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl"
    chmod +x /usr/local/bin/kubectl
fi
echo "kubectl: $(kubectl version --client=true --output=yaml 2>/dev/null | grep gitVersion | head -1)"

# k9s — interactive TUI for inspecting the Kind cluster.
if ! command -v k9s &>/dev/null || ! k9s version --short 2>/dev/null | grep -q "${K9S_VERSION#v}"; then
    echo "Installing k9s ${K9S_VERSION}..."
    curl -fsSL "https://github.com/derailed/k9s/releases/download/${K9S_VERSION}/k9s_Linux_amd64.tar.gz" -o /tmp/k9s.tar.gz
    tar -xzf /tmp/k9s.tar.gz -C /tmp k9s
    mv /tmp/k9s /usr/local/bin/k9s
    rm -f /tmp/k9s.tar.gz
fi
echo "k9s: $(k9s version --short 2>/dev/null | head -2 | tr '\n' ' ')"

# Evaluation targets — must be cloned before `go mod download` because
# go.mod has local-replace directives pointing at ./evaluation/<target>.
if [ -d "${REPO_DIR}" ]; then
    echo "Cloning evaluation targets..."
    REPO_DIR="${REPO_DIR}" "${REPO_DIR}/cloudlab/clone-targets.sh"

    # Hand the clones back to the swapper user so they can build without sudo.
    OWNER=$(stat -c '%U' "${REPO_DIR}")
    chown -R "${OWNER}:${OWNER}" "${REPO_DIR}/evaluation"
fi

# Pre-fetch Go module cache — run as the swapper user so the cache lands
# in their $HOME with the right ownership (not root's, which is unreachable
# from the user's later `go build`).
if [ -d "${REPO_DIR}" ]; then
    echo "Downloading Go modules..."
    OWNER=$(stat -c '%U' "${REPO_DIR}")
    sudo -u "${OWNER}" --preserve-env=PATH bash -c "cd '${REPO_DIR}' && /usr/local/go/bin/go mod download"
fi

echo "=== setup complete ($(date)) ==="
