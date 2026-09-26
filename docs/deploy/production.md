# Production Deployment (Recommended)

Recommended layout: **the WebsiteCore binary runs natively under systemd; PostgreSQL, Redis, and Meilisearch run in Docker containers** on the same host. Nginx terminates TLS and reverse-proxies to the application.

This guide assumes a Linux server (Debian/Ubuntu or similar) with Docker installed, and a domain name pointing at it. Work through [public-launch.md](public-launch.md) before opening the site to real users.

## 1. Build the release binary

On your build machine (or the server, if you build in place):

```sh
make build-web                          # build the Vue frontend into web/dist/
make linux-amd64 CGO_ENABLED=0 TAGS='embed migration'
```

The artifact is `release/linux-amd64/paopao-ce/paopao`: a single static binary with the frontend assets and database migrations embedded. Copy it to the server, e.g. `/opt/websitecore/paopao`.

> `make release TAGS='embed migration'` builds all four platforms (linux-amd64, darwin-amd64/arm64, windows-amd64) and zips each with LICENSE, README, config sample, scripts, and docs.

## 2. Prepare the application user and directories

```sh
sudo useradd --system --create-home --home-dir /opt/websitecore websitecore
sudo mkdir -p /opt/websitecore/custom
sudo chown -R websitecore:websitecore /opt/websitecore
```

`custom/` holds runtime data (LocalOSS uploads, logs) and must be writable by the service user and included in backups.

## 3. Start the dependency stack with Docker

Create `/opt/websitecore/docker-compose.yml` (adjust every password):

```yaml
name: websitecore-prod

services:
  postgres:
    image: postgres:18.6
    restart: unless-stopped
    environment:
      POSTGRES_USER: websitecore
      POSTGRES_PASSWORD: CHANGE-ME-strong-password
      POSTGRES_DB: websitecore
    ports:
      - "127.0.0.1:5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U websitecore -d websitecore"]
      interval: 10s
      timeout: 5s
      retries: 10

  redis:
    image: redis:7.4.11
    restart: unless-stopped
    command: ["redis-server", "--appendonly", "yes", "--requirepass", "CHANGE-ME-redis-password"]
    ports:
      - "127.0.0.1:6379:6379"
    volumes:
      - redisdata:/data
    healthcheck:
      test: ["CMD-SHELL", "redis-cli -a CHANGE-ME-redis-password ping | grep PONG"]
      interval: 10s
      timeout: 5s
      retries: 10

  meilisearch:
    image: getmeili/meilisearch:v1.54.0
    restart: unless-stopped
    environment:
      MEILI_MASTER_KEY: CHANGE-ME-meili-key
      MEILI_ENV: production
      MEILI_NO_ANALYTICS: "true"
    ports:
      - "127.0.0.1:7700:7700"
    volumes:
      - meilidata:/meili_data
    healthcheck:
      test: ["CMD-SHELL", "curl -fsS http://127.0.0.1:7700/health"]
      interval: 10s
      timeout: 5s
      retries: 10

volumes:
  pgdata:
  redisdata:
  meilidata:
```

```sh
cd /opt/websitecore && sudo docker compose up -d --wait
```

All ports are bound to `127.0.0.1`, so the services are reachable only from the server itself. See [database.md](database.md) for backup procedures.

## 4. Write the application config

Create `/opt/websitecore/config.yaml` (owner `websitecore`, mode `600`):

```yaml
App:
  RunMode: release

Features:
  Default: ["Web", "Frontend:EmbedWeb", "Meili", "LocalOSS", "Postgres", "BigCacheIndex", "LoggerFile", "Migration"]

WebServer:
  HttpIp: 127.0.0.1        # Nginx proxies to it; do not expose directly
  HttpPort: 8008

Database:
  LogLevel: error
  TablePrefix: p_

Postgres:
  User: websitecore
  Password: CHANGE-ME-strong-password
  DBName: websitecore
  Host: 127.0.0.1
  Port: 5432
  SSLMode: disable

Redis:
  InitAddress:
    - 127.0.0.1:6379
  Password: CHANGE-ME-redis-password

Meili:
  Host: 127.0.0.1:7700
  Index: paopao-data
  ApiKey: CHANGE-ME-meili-key
  Secure: false

LocalOSS:
  SavePath: custom/data/oss
  Domain: your-domain.example.com   # public domain, no scheme
  Secure: true                      # served over https

JWT:
  Secret: ""            # REQUIRED: openssl rand -hex 24
  Issuer: websitecore
  Expire: 86400

AdminSettings:
  EncryptionKey: ""     # REQUIRED: long random string

Logger:
  Level: error

Audit:
  Enabled: true         # keep content moderation on

Operator:
  Username: <your-ops-account>
  Password: <initial-password>   # 6-16 chars; change in the UI, then remove from this file
```

Notes:

- The binary was built with the `migration` tag, so including `"Migration"` in `Features.Default` migrates the schema automatically at every startup. Drop it if you prefer migrating manually (`./paopao migrate` with a migration-tagged binary).
- `LocalOSS.Domain` must be the public domain; uploaded file URLs are generated from it.
- Full reference: [configuration.md](configuration.md).

## 5. Install the systemd service

Create `/etc/systemd/system/websitecore.service` (adapted from `scripts/systemd/paopao.service`):

```ini
[Unit]
Description=WebsiteCore community server
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
User=websitecore
Group=websitecore
WorkingDirectory=/opt/websitecore
ExecStart=/opt/websitecore/paopao serve
Restart=always
RestartSec=3
Environment=USER=websitecore HOME=/opt/websitecore
# Hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/websitecore/custom
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now websitecore
sudo journalctl -u websitecore -f      # watch the first startup
```

On first boot the server creates the schema (if `Migration` is enabled) and provisions the Operator account from config. Log in at `http://127.0.0.1:8008` with the Operator credentials, change the password in the UI, then remove `Operator.Password` from `config.yaml` so restarts no longer reset it.

## 6. Nginx reverse proxy with HTTPS

Install Nginx and obtain a certificate (Let's Encrypt shown):

```sh
sudo apt install nginx certbot python3-certbot-nginx
```

Create `/etc/nginx/sites-available/websitecore`:

```nginx
server {
    listen 80;
    server_name your-domain.example.com;

    client_max_body_size 100m;          # video/attachment uploads

    location / {
        proxy_pass http://127.0.0.1:8008;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 120s;
    }
}
```

```sh
sudo ln -s /etc/nginx/sites-available/websitecore /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
sudo certbot --nginx -d your-domain.example.com   # installs and auto-renews TLS
```

## 7. Smoke test

```sh
curl -fsS http://127.0.0.1:8008/            # SPA index served
curl -fsS https://your-domain.example.com/  # public entry
```

Then in a browser: register or log in, post something (it should enter the audit queue if `Audit.Enabled`), upload an image (URL should use your domain over https), and search for a keyword (Meilisearch indexing).

## Upgrades

```sh
cd /opt/websitecore
sudo -u websitecore pg_dump ... > pre-upgrade-$(date +%F).dump   # backup first, see database.md
sudo systemctl stop websitecore
sudo install -o websitecore -g websitecore paopao.new /opt/websitecore/paopao
sudo systemctl start websitecore          # Migration feature applies new schema automatically
sudo journalctl -u websitecore -n 50
```

Rollback = restore the previous binary (and the database dump if a migration ran). Per governance policy, production only ever runs tagged releases, not `main` HEAD — see [../governance.md](../governance.md), section 5.

## Running dependencies without Docker

If Docker is not an option, install PostgreSQL, Redis, and Meilisearch from your distribution's packages and point `config.yaml` at them. The application makes no assumptions beyond the connection settings. A fully containerized deployment (application included) is described in [docker-compose.md](docker-compose.md).
