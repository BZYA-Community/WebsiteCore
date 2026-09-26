<div align="center">
  <h1>WebsiteCore</h1>
  <p>
    A self-hosted micro-community platform deeply customized from <a href="https://github.com/rocboss/paopao-ce">paopao-ce</a><br>
    Go + Vue 3 full stack · identity-group permissions · content moderation · Bilibili-style in-site messaging
  </p>
  <a href="https://github.com/BZYA-Community/WebsiteCore/actions/workflows/ci.yml">
    <img src="https://img.shields.io/github/actions/workflow/status/BZYA-Community/WebsiteCore/ci.yml?style=flat-square&label=CI" alt="CI">
  </a>
  <a href="https://goreportcard.com/report/github.com/BZYA-Community/WebsiteCore">
    <img src="https://img.shields.io/badge/Go%20Report%20Card-A%2B-brightgreen?style=flat-square" alt="Go Report Card">
  </a>
  <a href="https://github.com/BZYA-Community/WebsiteCore">
    <img src="https://img.shields.io/github/go-mod/go-version/BZYA-Community/WebsiteCore?style=flat-square&label=Go" alt="Go Version">
  </a>
  <a href="https://github.com/BZYA-Community/WebsiteCore/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/BZYA-Community/WebsiteCore?style=flat-square" alt="License">
  </a>
  <a href="https://github.com/BZYA-Community/WebsiteCore/issues">
    <img src="https://img.shields.io/github/issues/BZYA-Community/WebsiteCore?style=flat-square" alt="Issues">
  </a>
  <a href="https://github.com/BZYA-Community/WebsiteCore/stargazers">
    <img src="https://img.shields.io/github/stars/BZYA-Community/WebsiteCore?style=flat-square" alt="Stars">
  </a>
  <a href="https://github.com/BZYA-Community/WebsiteCore/network/members">
    <img src="https://img.shields.io/github/forks/BZYA-Community/WebsiteCore?style=flat-square" alt="Forks">
  </a>
  <a href="https://github.com/BZYA-Community/WebsiteCore/graphs/contributors">
    <img src="https://img.shields.io/github/contributors/BZYA-Community/WebsiteCore?style=flat-square" alt="Contributors">
  </a>
</div>

<br>

English | [简体中文](README_ZH.md)

## About

WebsiteCore is a self-hosted micro-community / forum system: a Go backend (Gin + GORM + Redis + Meilisearch) embedding a Vue 3 single-page app, shipped as a single binary. On top of the upstream paopao-ce it adds an identity-group system, a content moderation pipeline, conversational in-site messaging, and a course module — and it is built and governed by a student community, for a platform serving minors.

## Features

- **Identity groups and RBAC**: guest / member / mentor / auditor / admin / operator, with backend user management (role changes, mute, soft delete) and full audit trails
- **Content moderation**: posts from regular users enter a review queue; mentor and above are exempt; rejection returns content to private with resubmission; results are notified in-site
- **In-site messaging (Bilibili-style)**: session list with pinned system contacts, standalone chat window, read/unread state, paginated history, and messaging rules derived from identity groups
- **System notification session**: follow / comment / reply / moderation / management notices unified into one conversation, with jump links to posts and profiles
- **Courses and long-form content**: course groups, play counts, signed playback, Markdown long-form posts, topics, trending searches
- **Follow and visibility**: one-way following, post visibility levels (public / following / private)
- **Pluggable capabilities**: storage (LocalOSS / MinIO / S3 / AliOSS / COS / HuaweiOBS), search (Meilisearch / Zinc), database (PostgreSQL / MySQL) and more, assembled via `Features` flags

## Tech stack

| Layer | Technology |
| --- | --- |
| Backend | Go · Gin · GORM · Redis (rueidis) · go-mir (API codegen) · golang-migrate |
| Frontend | Vue 3 · Vite · Naive UI · Pinia · vue-advanced-chat · md-editor-v3 · Artplayer |
| Infrastructure | PostgreSQL / MySQL · Redis · Meilisearch (optional) |

## Quick start

### 1. Start the dependency stack

Local dependencies (PostgreSQL, Redis, Meilisearch) come up with one command:

```bash
make deps-up        # starts and waits for health checks
```

Images are pinned (`postgres:18.6` / `redis:7.4.11` / `getmeili/meilisearch:v1.54.0`) and ports bind to `127.0.0.1` only. Details: [docs/deploy/local.md](docs/deploy/local.md).

PostgreSQL is the default database; MySQL is also supported. Full-text search uses Meilisearch (optional).

### 2. Configure

```bash
cp config.yaml.sample config.yaml
```

Key values:

- `JWT.Secret`: **required** — the process exits if empty. Generate: `openssl rand -hex 24`
- `Features.Default`: capability flags; defaults to `Postgres`. Add `Migration` (with a `migration`-tagged build) for auto schema migration
- `WebServer.HttpPort`: listen port (default 8008)
- Database / Redis / Meili connection info already matches `docker-compose.dev.yml`

### 3. Create the schema

```bash
make migrate        # applies the embedded migrations
```

### 4. Build the frontend and run

```bash
make build-web          # builds web/dist/ for embedding
make run TAGS='embed'   # starts the backend with the embedded SPA
```

Open `http://127.0.0.1:8008`. Full installation and deployment guides: [docs/INSTALL.md](docs/INSTALL.md) and [docs/deploy/](docs/deploy/).

## Development

```bash
make run               # backend in dev mode (go run)
make gen-mir           # regenerate API code: mirc/ -> auto/ (never hand-edit generated files)
make gen-enum          # regenerate enums
make test              # go test ./...
cd web && npm run dev  # frontend dev server
```

Standard flow for a new API: declare it in `mirc/web/v1/` → `make gen-mir` generates the routing skeleton → implement it in `internal/servants/web/`. Schema changes go into `scripts/migration/{mysql,postgres}/` as numbered `NNNN_name.{up,down}.sql` pairs (one per dialect). Full guide: [docs/development.md](docs/development.md).

## Contributing

Every PR must pass CI, the AI review, and the **BVT** (Build Verification Test): backend syntax/build/lint/test checks, and — for frontend changes — layout checks ensuring no overlapping content at the standard viewports. See [CONTRIBUTING.md](CONTRIBUTING.md) for the full process, role/promotion system, and review rules.

This repository is maintained by a student community; contributions are recorded in the weekly report published every Friday.

## Repository layout

```
├── auto/          # API/routing code generated by go-mir (do not edit)
├── cmd/           # CLI entrypoints (serve / migrate / version)
├── internal/      # backend: conf, core, dao, servants, service, sitesetting, infra
├── mirc/          # API definitions (single source of truth for routes)
├── pkg/           # shared utilities
├── scripts/       # migrations, E2E/verification scripts, service units
├── web/           # Vue 3 frontend
└── docs/          # all documentation
```

## Documentation

| Document | Description |
| --- | --- |
| [docs/INSTALL.md](docs/INSTALL.md) | Installation and local setup |
| [docs/deploy/](docs/deploy/) | Deployment guides: configuration, database, SMS, production, Docker Compose, public-launch checklist |
| [docs/development.md](docs/development.md) | Development guide: environment, architecture, codegen, testing |
| [docs/ci-cd.md](docs/ci-cd.md) | CI pipelines, AI review, weekly report |
| [docs/features-status.md](docs/features-status.md) | Feature flag maturity matrix |
| [docs/CHANGELOG.md](docs/CHANGELOG.md) | Change history of this fork |
| [docs/governance.md](docs/governance.md) | Community governance charter (Chinese) |
| [docs/requirements-starisle-v2.0.md](docs/requirements-starisle-v2.0.md) | Product requirements baseline (Chinese) |
| [docs/openapi/](docs/openapi/) | OpenAPI assets (served at `/docs/openapi` with the `docs` build tag) |

Full index: [docs/README.md](docs/README.md).

## Acknowledgements

- Upstream project [rocboss/paopao-ce](https://github.com/rocboss/paopao-ce) (MIT License)

## License

[MIT](LICENSE)

## A Blessing to All Captains, Travelers, Trailblazers, and Proxies

Fight for all that is beautiful in the world!

We Will Be Reunited.

May this journey lead us starward!

Welcome to New Eridu.
