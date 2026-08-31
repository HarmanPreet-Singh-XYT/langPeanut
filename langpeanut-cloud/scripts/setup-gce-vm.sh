#!/usr/bin/env bash
# ==============================================================================
# langPeanut Cloud — Google Compute Engine (GCE) VM Provisioning Script
# ==============================================================================
# Run this once on a fresh Ubuntu 22.04/24.04 or Debian 12 GCE instance:
#   curl -fsSL https://raw.githubusercontent.com/langPeanut/langTranslate/main/langpeanut-cloud/scripts/setup-gce-vm.sh | bash
# ==============================================================================

set -euo pipefail

echo "========================================================"
echo " 🥜 Provisioning Google Compute Engine VM for langPeanut"
echo "========================================================"

# 1. Update system packages
echo "==> 1. Updating APT package lists..."
sudo apt-get update -y
sudo apt-get install -y ca-certificates curl gnupg lsb-release git ufw

# 2. Install Docker Engine + Docker Compose Plugin
echo "==> 2. Installing Docker Engine and Docker Compose v2..."
if ! command -v docker &> /dev/null; then
  curl -fsSL https://get.docker.com | sh
  sudo systemctl enable --now docker
else
  echo "Docker is already installed."
fi

# Add current user to docker group
sudo usermod -aG docker "$USER"

# 3. Create persistent deployment directory
DEPLOY_DIR="/opt/langpeanut"
echo "==> 3. Creating deployment directory at ${DEPLOY_DIR}..."
sudo mkdir -p "${DEPLOY_DIR}"
sudo chown "${USER}:${USER}" "${DEPLOY_DIR}"

# 4. Configure Firewall (Optional - allow HTTP/HTTPS/SSH)
echo "==> 4. Configuring basic UFW firewall..."
if command -v ufw &> /dev/null; then
  sudo ufw allow OpenSSH || true
  sudo ufw allow 80/tcp || true
  sudo ufw allow 443/tcp || true
  sudo ufw allow 443/udp || true  # HTTP/3 for Caddy
  sudo ufw --force enable || true
fi

echo ""
echo "========================================================"
echo " ✅ GCE VM Provisioning Complete!"
echo "========================================================"
echo " Next Steps:"
echo " 1. Log out and log back in (or run 'newgrp docker') so group changes take effect."
echo " 2. Configure GitHub Secrets in your repo settings:"
echo "    - GCP_PROJECT_ID"
echo "    - GCE_INSTANCE_NAME"
echo "    - GCE_ZONE"
echo "    - GCP_SA_KEY (Service Account JSON with Compute OS Admin / IAP permissions)"
echo "    - MASTER_KEY, GITHUB_APP_ID, GITHUB_WEBHOOK_SECRET, GITHUB_APP_PRIVATE_KEY_PEM"
echo " 3. Push to main or trigger the 'Deploy to GCE' GitHub Action workflow!"
echo "========================================================"
