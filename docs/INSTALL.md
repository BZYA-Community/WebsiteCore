# Installation Guide

English | [简体中文](INSTALL_ZH.md)

This guide covers the recommended ways to run PaoPao in development, evaluation, and self-hosted deployments. For a project overview, see [README.md](../README.md).

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
- `scripts/migration/{postgres,mysql}/` - versioned migration SQL
- `scripts/paopao-mysql.sql` - MySQL bootstrap schema
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
   openssl rand -base64 32   # put the output into JWT.Secret
   ```

3. Create the database schema from the embedded migrations:

   ```sh
   make migrate
   ```

4. Run or build the backend:

   ```sh
   make run               # web assets are embedded by default; run `make build-web` first so web/dist is not empty
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

The frontend is embedded **by default** — `web/embed.go` carries the build constraint `//go:build !(slim && embed)`, so a build with no tags (or with `embed` alone) packs `web/dist` into the binary. The `embed` tag by itself is a no-op; only the combination `slim embed` excludes the assets. Build the assets first (a fresh clone only has `web/dist/.gitkeep`), then run:

```sh
make build-web
make run                 # serves the embedded frontend (requires the `Frontend:EmbedWeb` feature)
make run TAGS='embed'    # identical result: `embed` alone changes nothing
```

With `make build TAGS='slim embed'` the assets are left out and no static routes are registered; in that case serve `web/dist` yourself (e.g. from Nginx).

<a id="deploy-release-binary"></a>

## Deploy the Release Binary to a Server

The recommended build uses the `migration` tag so database migrations are embedded in the binary; web assets are already embedded by default (see above), so the `embed` tag is optional and does not change what the binary serves. The server only needs the binary itself and a `config.yaml`.

```sh
# 1. Build the web assets
make build-web

# 2. Build the release binary (native platform)
make build TAGS='migration'        # TAGS='embed migration' is equivalent

# Or cross-compile for Linux amd64 (pure Go, no CGO needed)
make linux-amd64 CGO_ENABLED=0 TAGS='migration'
```

The artifact is written to `release/`. Deployment steps:

1. Upload `release/paopao` (`paopao.exe` on Windows) together with `config.yaml` to the same directory on your server.
2. Prepare the dependencies: database (MySQL/PostgreSQL), Redis, and Meilisearch, and point to them in `config.yaml`.
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
| `embed` | No-op on its own: web assets are embedded unless **both** `slim` and `embed` are set. Together with `slim` (`TAGS='slim embed'`) it strips the embedded frontend |
| `slim` | Only meaningful together with `embed` — see above |
| `migration` | Include migration support in the backend binary |
| `docs` | Enable the developer docs / OpenAPI service |

Examples:

```sh
make build TAGS='migration'
make run TAGS='docs'
make run                       # frontend is embedded by default
make build TAGS='slim embed'   # build without the embedded frontend
```

## Configuration Basics

At startup, PaoPao reads either:

1. `./config.yaml`
2. `./custom/config.yaml`

The first file found is used (search order defined by `newViper` in `internal/conf/setting.go`).

Important: the external file is no longer expected to carry every runtime knob. PaoPao loads embedded defaults first, then overlays your local config file.

Recommended split:

- **Bootstrap YAML**: ports, feature selection, database, Redis, JWT, `AdminSettings.EncryptionKey`
- **Admin UI (`/#/admin/settings`)**: most site, search, storage, SMS, payment, and app-behavior settings

If a setting is marked **restart required** in the admin page, it is persisted immediately but only becomes active after a process restart.

The `Features` section controls which capability bundles are enabled:

```yaml
Features:
  Default: ["Web", "Frontend:EmbedWeb", "Meili", "LocalOSS", "Postgres", "BigCacheIndex", "LoggerFile"]
  Develop: ["Base", "MySQL", "BigCacheIndex", "Meili", "Sms", "AliOSS", "LoggerMeili", "OSS:Retention"]
  Demo: ["Base", "MySQL", "Option", "Zinc", "Sms", "MinIO", "LoggerZinc", "Migration"]
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

The default modern stack is centered on **Meilisearch**, **Redis**, and either **LocalOSS**, **MinIO**, or a cloud object store. Optional integrations can be started separately when needed.

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

### MinIO

```sh
mkdir -p data/minio/data
docker run -d --name minio \
  -v ${PWD}/data/minio/data:/data \
  -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=minio-root-user \
  -e MINIO_ROOT_PASSWORD=minio-root-password \
  -e MINIO_DEFAULT_BUCKETS=paopao:public \
  bitnami/minio:latest
```

Matching config example:

```yaml
MinIO:
  AccessKey: Q3AM3UQ867SPQQA43P2F
  SecretKey: zuf+tfteSlswRu7BJ86wekitnifILbZam1KYY3TG
  Secure: False
  Endpoint: 127.0.0.1:9000
  Bucket: paopao
  Domain: 127.0.0.1:9000
```

### OpenObserve

```sh
mkdir -p data/openobserve
docker run -v ${PWD}/data/openobserve:/data \
  -e ZO_DATA_DIR=/data \
  -p 5080:5080 \
  -e ZO_ROOT_USER_EMAIL=root@paopao.info \
  -e ZO_ROOT_USER_PASSWORD=paopao-ce \
  public.ecr.aws/zinclabs/openobserve:latest
```

### Pyroscope

```sh
docker run -it -p 4040:4040 pyroscope/pyroscope:latest server
```

### Zinc (legacy / optional)

Zinc still appears in the repository and feature definitions, but the current default stack is Meilisearch-based. Use it only if you intentionally want the legacy search path.

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

## Security Recommendations

Hardening checklist for a public deployment (vulnerability reporting: see [SECURITY.md](../SECURITY.md)):

- **`JWT.Secret` is mandatory.** The process prints an error and exits at startup when it is empty. Generate a random value (`openssl rand -base64 32`) and keep it in `config.yaml` / `custom/config.yaml`, never in a public repository. Rotating it invalidates all existing sessions.
- **Replace the shipped defaults.** `config.yaml.sample` uses `App.RunMode: debug` and `AdminSettings.EncryptionKey: CHANGE-ME-TO-A-LONG-RANDOM-SECRET` — set `RunMode: release` and generate your own key (it seeds the encryption of settings persisted by the admin UI).
- **Do not expose the API port directly.** Keep `WebServer.HttpIp`/`HttpPort` reachable only from your reverse proxy, and keep PostgreSQL, Redis and Meilisearch on a private network: the sample binds them to `127.0.0.1` with development credentials (`paopao`/`paopao`) that must be changed in production.
- **Uploads are validated server-side**: only the `public/image`, `public/video`, `public/avatar` and `attachment` upload types, a `Content-Type` allow-list (`webp/png/jpg/gif/mp4/mov/zip`) and a 100 MB per-file limit. Keep your reverse proxy's body-size limit aligned with it, and don't expose the `LocalOSS` storage directory through a permissive file server.
- **Terminate TLS at a reverse proxy.** The Go service speaks plain HTTP only; put Nginx/Caddy in front with HTTPS (and rate limiting), and only publish that endpoint.
- **Protect the data directory.** `config.yaml` and the `custom/` directory (attachments and local data) hold secrets and user content — restrict filesystem permissions and back them up.

## Additional Deployment Docs

For platform-specific or production-oriented deployment references, see:

- [docs/deploy/README.md](deploy/README.md)
- [docs/deploy/core/](deploy/core/)
- [docs/deploy/local/](deploy/local/)
- [docs/deploy/k8s/](deploy/k8s/)
- [docs/deploy/aliyun/](deploy/aliyun/)
- [docs/deploy/huawei/](deploy/huawei/)
- [docs/deploy/tencent/](deploy/tencent/)

## Operational Notes

- For long-running deployments, it is reasonable to run the backend under a process manager and place Nginx in front of the application.
- The SMS implementation currently references Juhe in the sample configuration. If that provider is not suitable for your deployment, replace it with another compatible service.
- The repository includes multiple runtime combinations; keep your selected `Features` set aligned with the infrastructure you actually provision.
