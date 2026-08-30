# langPeanut Cloud — VPS Deployment Guide

> **Self-Sufficient Single-VPS Deployment Guide**  
> Run your own hosted GitHub localization bot with automatic sandboxed job containers, Next.js web dashboard, SQLite persistence, and an HTTPS reverse proxy (bundled Caddy, or your own nginx).

---

## 1. Overview & Architecture

`langPeanut Cloud` runs on a single VPS as a lightweight, fully self-sufficient stack:

- **Reverse Proxy**: bundled Caddy (automatic Let's Encrypt TLS on ports 80/443) — or, if the VPS
  already runs nginx with HTTPS configured, skip Caddy and proxy to the app container instead
  (§7 Option B). Either way the app itself always listens on `127.0.0.1:8080`.
- **Core Server**: `langpeanut-cloud` Go binary (serves HTTP API + Next.js frontend + in-process worker queue).
- **Datastore**: Embedded SQLite in WAL mode (`/data/langpeanut.db`) — zero DB server maintenance.
- **Sandboxed Runner**: `langpeanut-runner` Docker image — auto-spawns an ephemeral container per localization job and auto-destroys on exit.

```
┌─────────────────────────────────────────────────────────────┐
│                          YOUR VPS                           │
│                                                             │
│   ┌────────────────┐     ┌────────────────────────────────┐ │
│   │ Caddy :80/:443 │────►│ app:8080 (Go API + Next.js UI) │ │
│   │  -- or --      │     └───────────────┬────────────────┘ │
│   │ existing nginx │                     │                  │
│   └────────────────┘              /var/run/docker.sock      │
│                                          │                   │
│                                          ▼                   │
│                         ┌────────────────────────────────┐  │
│                         │ Ephemeral Runner Container     │  │
│                         │ (Auto-spawned per job, capped, │  │
│                         │  auto-destroyed on completion) │  │
│                         └────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. Server Requirements

- **OS**: Ubuntu 22.04 / 24.04 LTS or Debian 12.
- **Hardware**: 1 vCPU, 2 GB RAM, 20 GB SSD (minimum recommended: Hetzner CX22, DigitalOcean $12/mo droplet, or equivalent).
- **Software**: Docker Engine + Docker Compose v2.

---

## 3. Step 1: Create Your GitHub App

1. Go to [GitHub Developer Settings → GitHub Apps → New GitHub App](https://github.com/settings/apps/new).
2. Fill in the basic details:
   - **GitHub App name**: `langPeanut Localization Bot` (or your preferred name).
   - **Homepage URL**: `https://yourdomain.com` (or `http://your-vps-ip`).
   - **Callback URL**: `https://yourdomain.com/api/auth/callback` (for OAuth login).
   - **Webhook URL**: `https://yourdomain.com/api/webhook`.
   - **Webhook secret**: Generate a random string (e.g. `openssl rand -hex 16`).
3. Set **Permissions**:
   - **Repository permissions**:
     - *Contents*: **Read & Write** (needed to clone and push localized branches).
     - *Pull Requests*: **Read & Write** (needed to open PRs and post review comments).
     - *Metadata*: **Read-only** (default).
4. Set **Subscribe to events**:
   - Check **Push** and **Installation**.
5. Click **Create GitHub App**.
6. Under **General Settings**:
   - Copy your numeric **App ID**.
   - Scroll down to **Private keys** and click **Generate a private key**. Save the downloaded `.pem` file.

---

## 4. Step 2: Prepare the VPS

SSH into your VPS and install Docker & Docker Compose:

```bash
# Update and install Docker
curl -fsSL https://get.docker.com | sh

# Enable Docker service
sudo systemctl enable --now docker

# Add your user to the docker group (optional)
sudo usermod -aG docker $USER
```

---

## 5. Step 3: Clone the Repository

```bash
sudo mkdir -p /opt/langpeanut
sudo chown $USER:$USER /opt/langpeanut

cd /opt/langpeanut
git clone https://github.com/langPeanut/langTranslate.git .
cd langpeanut-cloud
```

---

## 6. Step 4: Configure Environment & Secrets

1. Copy the example environment file:
   ```bash
   cp .env.example .env
   ```

2. Generate a 32-byte master encryption key:
   ```bash
   openssl rand -hex 32
   ```

3. Edit `.env` with your values:
   ```env
   # 32-byte hex master key for AES-256-GCM credential encryption at rest
   MASTER_KEY=paste_your_64_character_hex_key_here

   # GitHub App configuration
   GITHUB_APP_ID=123456
   GITHUB_APP_PRIVATE_KEY_PATH=/data/github-app.pem
   GITHUB_WEBHOOK_SECRET=your_webhook_secret_here

   # Optional runtime settings
   LISTEN_ADDR=:8080
   DATABASE_PATH=/data/langpeanut.db
   MIRRORS_DIR=/data/mirrors
   JOBS_DIR=/data/jobs
   RUNNER_IMAGE=langpeanut-runner:latest
   ```

4. Create persistent data directories and place the GitHub App private key:
   ```bash
   mkdir -p data/mirrors data/jobs
   cp /path/to/downloaded-private-key.pem data/github-app.pem
   chmod 600 data/github-app.pem .env
   ```

---

## 7. Step 5: Configure Domain & SSL

The `app` container always publishes its API on `127.0.0.1:8080` on the host —
loopback only. Something has to sit in front of it for TLS. Pick whichever of
these matches your VPS:

### Option A — Use the bundled Caddy (no existing proxy on the box)

Open `Caddyfile`:
```bash
nano Caddyfile
```

Replace `:80` with your domain to enable automatic Let's Encrypt SSL:
```caddy
langpeanut.yourdomain.com {
    reverse_proxy app:8080

    header {
        Strict-Transport-Security "max-age=31536000; includeSubDomains"
        X-Content-Type-Options nosniff
        X-Frame-Options DENY
        Referrer-Policy strict-origin-when-cross-origin
    }

    encode gzip
}
```
*(Note: Point your domain's DNS A record to your VPS IP address before starting Caddy).*

Then start the stack **with** the `caddy` profile enabled (see Step 6 below)
so Caddy binds 80/443 and gets its own certificate.

### Option B — VPS already has nginx configured with HTTPS

Don't run the bundled Caddy at all — two processes fighting over ports 80/443
will just fail to bind. Instead:

1. Start the stack **without** the caddy profile (Step 6, plain `docker compose up -d --build`).
   The `app` container is reachable at `127.0.0.1:8080` on the host and nothing
   else touches ports 80/443.
2. Copy `nginx.example.conf` into `/etc/nginx/sites-available/langpeanut`, edit
   `server_name` and the `ssl_certificate`/`ssl_certificate_key` paths to match
   your existing cert (e.g. from certbot), then:
   ```bash
   sudo ln -s /etc/nginx/sites-available/langpeanut /etc/nginx/sites-enabled/
   sudo nginx -t && sudo systemctl reload nginx
   ```
3. Your GitHub App's webhook/callback/homepage URLs (Step 3 above) should point
   at whatever domain nginx is already terminating TLS for.

---

## 8. Step 6: Build Runner Image & Launch Stack

1. **Build the sandboxed runner image** (spawns per job):
   ```bash
   docker build -f Dockerfile.runner -t langpeanut-runner:latest \
     --build-context langpeanut_local=../langpeanut_local .
   ```

2. **Launch the stack with Docker Compose**:

   - If nginx (or another proxy) already handles HTTPS on this VPS (Option B above):
     ```bash
     docker compose up -d --build
     ```
   - If you want the bundled Caddy to terminate TLS instead (Option A above):
     ```bash
     docker compose --profile caddy up -d --build
     ```

---

## 9. Step 7: Verify Deployment

Check running containers:
```bash
docker compose ps
```
*Expected output (Option A, with `--profile caddy`):*
```
NAME                    IMAGE                     STATUS         PORTS
langpeanut-cloud-app-1    langpeanut-cloud-app      Up (healthy)   127.0.0.1:8080->8080/tcp
langpeanut-cloud-caddy-1  caddy:2                   Up             0.0.0.0:80->80/tcp, 0.0.0.0:443->443/tcp
```
*Expected output (Option B, nginx already handling 80/443):*
```
NAME                    IMAGE                     STATUS         PORTS
langpeanut-cloud-app-1    langpeanut-cloud-app      Up (healthy)   127.0.0.1:8080->8080/tcp
```

Test health check:
```bash
curl -i http://localhost:8080/health
# Response: HTTP/1.1 200 OK {"status":"ok"}
```

Open your browser at `https://langpeanut.yourdomain.com` (or `http://your-vps-ip`).

---

## 10. Operations & Maintenance

### Viewing Live Logs
```bash
# Server & API logs
docker compose logs -f app

# Caddy reverse proxy logs (only if running with --profile caddy)
docker compose logs -f caddy
```

### Backing Up Data
All state lives in `./data`:
```bash
# Back up SQLite database and git mirrors
tar -czvf langpeanut-backup-$(date +%Y%m%d).tar.gz data/langpeanut.db data/mirrors/
```

### 1-Command Update
To pull the latest code improvements and restart without downtime (add
`--profile caddy` to the last command if you're using bundled Caddy):
```bash
git pull && \
docker build -f Dockerfile.runner -t langpeanut-runner:latest . && \
docker compose up -d --build
```
