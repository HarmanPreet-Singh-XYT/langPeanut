#!/usr/bin/env bash
set -euo pipefail

DEPLOY_PATH="/opt/langpeanut"

echo "==> 1. Preparing deployment directory and Docker permissions..."
sudo mkdir -p "${DEPLOY_PATH}/langpeanut-cloud"
sudo chown -R "$(whoami):$(whoami)" "${DEPLOY_PATH}"
sudo chmod -R 775 "${DEPLOY_PATH}"
sudo usermod -aG docker "$(whoami)" || true
sudo chmod 666 /var/run/docker.sock || true

echo "==> 2. Unpacking pre-compiled deployment bundle..."
tar -xzf /tmp/deploy-package.tar.gz -C "${DEPLOY_PATH}/langpeanut-cloud"
cp "${DEPLOY_PATH}/langpeanut-cloud/Dockerfile.prebuilt" "${DEPLOY_PATH}/langpeanut-cloud/Dockerfile"
cp "${DEPLOY_PATH}/langpeanut-cloud/Dockerfile.runner.prebuilt" "${DEPLOY_PATH}/langpeanut-cloud/Dockerfile.runner"

cd "${DEPLOY_PATH}/langpeanut-cloud"

echo "==> 3. Creating persistent data directories..."
mkdir -p data/mirrors data/jobs

echo "==> 4. Deploying environment files..."
cp /tmp/local.env .env
chmod 600 .env

if [ -s /tmp/local_github_app.pem ]; then
  echo "==> 5. Deploying GitHub App private key..."
  cp /tmp/local_github_app.pem data/github-app.pem
  chmod 644 data/github-app.pem
  chmod -R 777 data/
fi

echo "==> 6. Launching pre-built Docker containers..."
docker build -f Dockerfile.runner -t langpeanut-runner:latest .
if [ "${ENABLE_CADDY}" = "yes" ]; then
  docker compose --profile caddy up -d --build
else
  docker compose up -d --build
fi

# Clean up dangling build layers
docker image prune -f || true

echo "==> 7. Verifying application health status..."
sleep 5
for i in {1..20}; do
  if curl -sf http://127.0.0.1:8080/health | grep -q 'ok'; then
    echo "==> SUCCESS: langPeanut Cloud is healthy and running on GCE!"
    rm -f /tmp/local.env /tmp/local_github_app.pem /tmp/deploy-package.tar.gz /tmp/deploy-gce.sh
    exit 0
  fi
  echo "Waiting for app to become ready (attempt $i/20)..."
  sleep 3
done

echo "==> ERROR: Health check failed after deployment!"
docker compose logs --tail=50 app
exit 1
