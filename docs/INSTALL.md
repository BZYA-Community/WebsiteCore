# Installation Guide

This guide covers the recommended ways to run WebsiteCore in development, evaluation, and self-hosted deployments. For a project overview, see [README.md](../README.md); for the full operations library, see [deploy/README.md](deploy/README.md).

## Choose an Installation Path

| Scenario | Recommended path |
| --- | --- |
| Backend or frontend development | [Run from source](#run-from-source) |
| Deploy to a server | [Deploy the release binary](#deploy-release-binary) |

## Requirements

### For source-based development

- Go `1.24+`
- Node.js `20.19+` or `22.12+`
- Yarn `1.x`
- Docker with Compose (for the local PostgreSQL / Redis / Meilisearch stack)

### Helpful repository files

- `config.yaml.sample` - canonical configuration template (overrides over embedded defaults)
- `docker-compose.dev.yml` - local PostgreSQL / Redis / Meilisearch dev stack
- `scripts/migration/postgres/` - versioned migration SQL
- `scripts/paopao-postgres.sql` - PostgreSQL bootstrap schema

<a id="run-from-source"></a>

## Run from Source

### Backend

1. Start the local dependency stack (PostgreSQL, Redis, Meilisearch):

   ```sh
   make deps-up
   ```

2. Copy the configuration template and set a JWT secret — the app exits at startup if `JWT.Secret` is empty:

   ```sh
   cp config.yaml.sample config.yaml
   openssl rand -hex 24   # put the output into JWT.Secret
   ```

3. Create the database schema from the embedded migrations:

   ```sh
   make migrate
   ```

4. Run or build the backend:

   ```sh
   make run               # or: make build-web && make run TAGS='embed' to serve the frontend
   ```

The dev stack pins `postgres:18.6`, `redis:7.4.11` and `getmeili/meilisearch:v1.54.0`, binds every port to `127.0.0.1`, and keeps data in named volumes (`make deps-reset` wipes them). `config.yaml.sample` already targets this stack: PostgreSQL, database `websitecore`, user/password `paopao`.

Build a release binary:

```sh
make build
```

The binary is written to `release/paopao`.

### Web frontend

```sh
cd web
cp .env .env.local
yarn
yarn dev
```

Build the web bundle:

```sh
yarn build
```

### Embedded web UI

If you want the Go service to serve the web frontend directly, build the frontend assets first and then run with the `embed` tag:

```sh
make build-web
make run TAGS='embed'
```

<a id="deploy-release-binary"></a>

## Deploy the Release Binary to a Server

The recommended build uses the `embed` + `migration` tags so the web assets and database migrations are embedded in the binary. The server only needs the binary itself and a `config.yaml`.

```sh
# 1. Build the web assets
make build-web

# 2. Build the release binary (native platform)
make build TAGS='embed migration'

# Or cross-compile for Linux amd64 (pure Go, no CGO needed)
make linux-amd64 CGO_ENABLED=0 TAGS='embed migration'
```

The artifact is written to `release/`. Deployment steps:

1. Upload `release/paopao` (`paopao.exe` on Windows) together with `config.yaml` to the same directory on your server.
2. Prepare the dependencies: PostgreSQL, Redis, and (optionally) Meilisearch, and point to them in `config.yaml`.
3. Start the service:

```sh
./paopao serve
```

Notes:

- Automatic migration requires both the `migration` build tag and the `"Migration"` feature declared in the `Features` section of `config.yaml` (e.g. `Default: ["Web", "Frontend:EmbedWeb", "Meili", "LocalOSS", "Postgres", "Migration"]`). When both are present, the schema is migrated automatically at startup.
- Without the `migration` tag the schema is not created automatically; rebuild with the tag, or apply the versioned migrations under `scripts/migration/` yourself.
- Attachments and other persistent data are stored under `custom/` next to the binary by default; back that directory up.

## Common Build Tags

| Tag | Purpose |
| --- | --- |
| `embed` | Serve the built web frontend from the Go binary |
| `migration` | Include migration support in the backend binary |
| `docs` | Enable the developer docs / OpenAPI service |

Examples:

```sh
make build TAGS='migration'
make run TAGS='docs'
make run TAGS='embed'
```

## Configuration Basics

At startup, WebsiteCore reads either:

1. `./custom/config.yaml`
2. `./config.yaml`

The first file found is used.

Important: the external file is no longer expected to carry every runtime knob. WebsiteCore loads embedded defaults first, then overlays your local config file. A complete reference for every section and feature flag is in [deploy/configuration.md](deploy/configuration.md).

Recommended split:

- **Bootstrap YAML**: ports, feature selection, database, Redis, JWT, `AdminSettings.EncryptionKey`
- **Admin UI (`/#/admin/settings`)**: most site, search, storage, SMS, and app-behavior settings

If a setting is marked **restart required** in the admin page, it is persisted immediately but only becomes active after a process restart.

The `Features` section controls which capability bundles are enabled:

```yaml
Features:
  Default: ["Web", "Frontend:EmbedWeb", "Meili", "LocalOSS", "Postgres", "BigCacheIndex", "LoggerFile"]
  Develop: ["Base", "Postgres", "BigCacheIndex", "Meili", "Sms", "AliOSS", "LoggerOtlp", "OSS:Retention"]
  Demo: ["Base", "Postgres", "Option", "Meili", "Sms", "LocalOSS", "LoggerFile", "Migration"]
  Slim: ["Base", "Postgres", "LocalOSS", "LoggerFile", "OSS:TempDir"]
```

Useful commands:

```sh
# Use the default suite
release/paopao serve

# Use only the develop suite
release/paopao serve --no-default-features --features develop

# Add one extra feature on top of default
release/paopao serve --features sms

# Specify features explicitly
release/paopao serve --no-default-features --features postgres,localoss,loggerfile,redis
```

For feature maturity and support status, see [features-status.md](features-status.md).

## Optional Infrastructure Services

The default modern stack is centered on **Meilisearch**, **Redis**, and **LocalOSS** (or AliOSS for cloud deployments). Optional integrations can be started separately when needed.

### Meilisearch (recommended search engine)

`make deps-up` already starts Meilisearch for local development. To run a standalone instance instead:

```sh
mkdir -p data/meili/data
docker run -d --name meili \
  -v ${PWD}/data/meili/data:/meili_data \
  -p 127.0.0.1:7700:7700 \
  -e MEILI_MASTER_KEY=paopao-meilisearch \
  getmeili/meilisearch:v1.54.0
```

Matching config example:

```yaml
Meili:
  Host: 127.0.0.1:7700
  Index: paopao-data
  ApiKey: paopao-meilisearch
  Secure: False
```

### Pyroscope

```sh
docker run -it -p 4040:4040 pyroscope/pyroscope:latest server
```

> **Note:** When the `Meili` feature is not enabled, search automatically falls back to PostgreSQL `ILIKE` fuzzy matching — no external search service is required.

## Enable API Documentation Locally

Add the Docs feature suite and run with the `docs` build tag:

```yaml
Features:
  Default: ["Base", "Postgres", "Option", "LocalOSS", "LoggerFile", "Docs"]
  Docs: ["Docs:OpenAPI"]
```

```sh
make run TAGS='docs'
```

Then visit:

- `http://127.0.0.1:8011/docs/openapi`

## Additional Deployment Docs

For production-oriented deployment references, see:

- [deploy/README.md](deploy/README.md) - deployment overview and path selection
- [deploy/production.md](deploy/production.md) - recommended production layout (binary + systemd + Nginx, dependencies in Docker)
- [deploy/docker-compose.md](deploy/docker-compose.md) - fully containerized alternative
- [deploy/configuration.md](deploy/configuration.md) - full configuration reference
- [deploy/database.md](deploy/database.md) - database deployment, migrations, backups
- [deploy/sms.md](deploy/sms.md) - SMS platform configuration
- [deploy/public-launch.md](deploy/public-launch.md) - pre-launch security and compliance checklist

## Operational Notes

- For long-running deployments, it is reasonable to run the backend under a process manager and place Nginx in front of the application.
- The SMS implementation currently references Juhe in the sample configuration. If that provider is not suitable for your deployment, replace it with another compatible service.
- The repository includes multiple runtime combinations; keep your selected `Features` set aligned with the infrastructure you actually provision.
