#!/usr/bin/env bash
# One-time toolchain setup for Ubuntu 24.04 on WSL2: Docker Engine, kubectl,
# kind, helm and Go. Safe to run again; already-installed tools are skipped.
#
#   bash scripts/wsl-setup.sh
set -euo pipefail

KIND_VERSION="v0.30.0"
GO_VERSION="1.24.7"

if [ "$(ps -p 1 -o comm=)" != "systemd" ]; then
  echo "systemd is not PID 1. Add this to /etc/wsl.conf, then run 'wsl --shutdown' from Windows:" >&2
  echo "  [boot]" >&2
  echo "  systemd=true" >&2
  exit 1
fi

sudo apt-get update
sudo apt-get install -y ca-certificates curl gnupg git make

# --- Docker Engine from the official apt repo ---
if ! command -v docker >/dev/null; then
  sudo install -m 0755 -d /etc/apt/keyrings
  sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
  sudo chmod a+r /etc/apt/keyrings/docker.asc
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" |
    sudo tee /etc/apt/sources.list.d/docker.list >/dev/null
  sudo apt-get update
  sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
fi
sudo systemctl enable --now docker
if ! id -nG "$USER" | grep -qw docker; then
  sudo usermod -aG docker "$USER"
  echo "Added $USER to the docker group: run 'wsl --shutdown' from Windows and reopen Ubuntu."
fi

# --- kubectl (latest stable) ---
if ! command -v kubectl >/dev/null; then
  KVER="$(curl -fsSL https://dl.k8s.io/release/stable.txt)"
  curl -fsSLo /tmp/kubectl "https://dl.k8s.io/release/${KVER}/bin/linux/amd64/kubectl"
  sudo install -m 0755 /tmp/kubectl /usr/local/bin/kubectl
  rm -f /tmp/kubectl
fi

# --- kind ---
if ! command -v kind >/dev/null; then
  curl -fsSLo /tmp/kind "https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-linux-amd64"
  sudo install -m 0755 /tmp/kind /usr/local/bin/kind
  rm -f /tmp/kind
fi

# --- helm 3 ---
if ! command -v helm >/dev/null; then
  curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
fi

# --- Go ---
if ! /usr/local/go/bin/go version 2>/dev/null | grep -q "go${GO_VERSION}"; then
  curl -fsSLo /tmp/go.tgz "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"
  sudo rm -rf /usr/local/go
  sudo tar -C /usr/local -xzf /tmp/go.tgz
  rm -f /tmp/go.tgz
fi
if ! grep -q '/usr/local/go/bin' "$HOME/.profile"; then
  echo 'export PATH=$PATH:/usr/local/go/bin' >> "$HOME/.profile"
fi

echo
echo "Done. Verify in a new shell:"
echo "  docker run --rm hello-world && kind version && kubectl version --client && helm version --short && go version"
