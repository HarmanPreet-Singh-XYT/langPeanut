#!/usr/bin/env bash
set -euo pipefail

DEPLOY_PATH="/opt/langpeanut"

echo "==> 1. Preparing deployment directory and Docker permissions..."
sudo mkdir -p "${DEPLOY_PATH}"
sudo chown -R "$(whoami):$(whoami)" "${DEPLOY_PATH}"
sudo chmod -R 775 "${DEPLOY_PATH}"
sudo git config --system --add safe.directory '*' || true
git config --global --add safe.directory '*' || true
sudo usermod -aG docker "$(whoami)" || true
sudo chmod 666 /var/run/docker.sock || true

cd "${DEPLOY_PATH}"

echo "==> 2. Syncing source repository..."
if [ ! -d .git ]; then
  git clone "${REPO_URL}" .
else
  git remote set-url origin "${REPO_URL}"
  git fetch origin "${TARGET_BRANCH}"
  git checkout -f "${TARGET_BRANCH}"
  git reset --hard "origin/${TARGET_BRANCH}"
  git clean -fd
fi

cd "${DEPLOY_PATH}/langpeanut-cloud"

echo "==> 3. Creating persistent data directories..."
mkdir -p data/mirrors data/jobs

echo "==> 4. Deploying environment files..."
cp /tmp/local.env .env
chmod 600 .env

if [ -s /tmp/local_github_app.pem ]; then
  echo "==> 5. Deploying GitHub App private key..."
  cp /tmp/local_github_app.pem data/github-app.pem
  chmod 600 data/github-app.pem
fi

echo "==> 6. Building sandboxed runner image (langpeanut-runner:latest)..."
docker build -f Dockerfile.runner -t langpeanut-runner:latest \
  --build-context langpeanut_local=../langpeanut_local .

echo "==> 7. Launching Docker Compose stack..."
if [ "${ENABLE_CADDY}" = "yes" ]; then
  docker compose --profile caddy up -d --build
else
  docker compose up -d --build
fi

# Clean up dangling build layers
docker image prune -f || true

echo "==> 8. Verifying application health status..."
sleep 5
for i in {1..20}; do
  if curl -sf http://127.0.0.1:8080/health | grep -q 'ok'; then
    echo "==> SUCCESS: langPeanut Cloud is healthy and running on GCE!"
    rm -f /tmp/local.env /tmp/local_github_app.pem /tmp/deploy-gce.sh
    exit 0
  fi
  echo "Waiting for app to become ready (attempt $i/20)..."
  sleep 3
done

echo "==> ERROR: Health check failed after deployment!"
docker compose logs --tail=50 app
exit 1
