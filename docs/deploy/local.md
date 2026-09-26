# Local Development Environment

This guide brings up a full development stack on your machine: dependency services in Docker, the Go backend and Vue frontend running natively.

## Prerequisites

| Tool | Version | Notes |
| --- | --- | --- |
| Go | see `go.mod` (1.24+) | backend |
| Node.js | 22.x | matches CI; 20.19+ also works |
| Yarn or npm | Yarn 1.x / npm 10+ | `make build-web` uses Yarn; CI uses npm |
| Docker + Compose | any recent | dependency stack |
| make | GNU make | on Windows use Git Bash or WSL |

## 1. Start the dependency stack

```sh
make deps-up
```

This runs `docker compose -f docker-compose.dev.yml up -d --wait` and blocks until every service passes its health check. Images are pinned and all ports bind to `127.0.0.1` only:

| Service | Image | Port | Credentials | Volume |
| --- | --- | --- | --- | --- |
| PostgreSQL | `postgres:18.6` | 5432 | `paopao` / `paopao`, database `websitecore` | `websitecore-pgdata` |
| Redis | `redis:7.4.11` | 6379 | none (AOF persistence enabled) | `websitecore-redisdata` |
| Meilisearch | `getmeili/meilisearch:v1.54.0` | 7700 | master key `paopao-meilisearch` | `websitecore-meilidata` |

Other stack commands:

```sh
make deps-status   # compose ps
make deps-logs     # follow logs
make deps-down     # stop, keep volumes
make deps-reset    # stop and DELETE all data volumes
```

## 2. Configure

```sh
cp config.yaml.sample config.yaml
```

Then edit `config.yaml` and set one required value:

```yaml
JWT:
  Secret: ""   # fill with: openssl rand -hex 24
```

Everything else already points at the dev stack (PostgreSQL on 127.0.0.1:5432, Redis on 6379, Meili on 7700). The default feature set is:

```yaml
Features:
  Default: ["Web", "Frontend:EmbedWeb", "Meili", "LocalOSS", "Postgres", "BigCacheIndex", "LoggerFile"]
```

A minimal working sample is also kept at [examples/config.yaml](examples/config.yaml).

## 3. Create the database schema

```sh
make migrate
```

This runs the `migrate` subcommand with the `migration` build tag, applying every SQL file under `scripts/migration/postgres/` (or `mysql/`) in order. See [database.md](database.md) for details.

## 4. Build the frontend and run

```sh
make build-web          # yarn build in web/, output to web/dist/
make run TAGS='embed'   # go run with the SPA embedded in the binary
```

Open <http://127.0.0.1:8008>.

The first account is created from the `Operator` section of `config.yaml` (default username `kiana`). Set `Operator.Password` (6-16 chars) to have it created automatically with admin privileges; on later starts, changing the value resets the password.

## Frontend-only development

For hot-reload frontend work, run the Vite dev server against a running backend:

```sh
make run                       # backend on :8008 (API only)
cd web && npm install && npm run dev
```

See [../development.md](../development.md) for the full development workflow, code generation, and testing.

## Troubleshooting

| Symptom | Cause / fix |
| --- | --- |
| Process exits immediately at startup | `JWT.Secret` is empty. Generate one with `openssl rand -hex 24`. |
| `make migrate` says the build lacks the migration tag | You ran the plain binary instead of `make migrate`. The Makefile adds `-tags migration` automatically. |
| Port 5432/6379/7700 already in use | A local service or another compose project holds the port. Stop it, or edit `docker-compose.dev.yml` host ports. |
| Search returns nothing | Meilisearch not healthy yet (`make deps-status`), or `Meili` missing from `Features.Default`. |
| Data vanished after `make deps-reset` | Expected: `deps-reset` deletes the volumes. Use `make deps-down` to keep data. |
