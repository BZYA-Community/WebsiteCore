# Deployment Guides

This directory covers everything needed to run WebsiteCore, from a laptop development stack to a public production server.

## Pick a path

| I want to... | Read |
| --- | --- |
| Run the project locally for development | [local.md](local.md) |
| Understand `config.yaml`, feature flags, and admin-managed settings | [configuration.md](configuration.md) |
| Set up PostgreSQL, run migrations, take backups | [database.md](database.md) |
| Enable SMS verification (phone binding) | [sms.md](sms.md) |
| Deploy to a server (recommended: native binary + Dockerized dependencies) | [production.md](production.md) |
| Deploy everything with Docker Compose | [docker-compose.md](docker-compose.md) |
| Open the site to the public internet safely | [public-launch.md](public-launch.md) |

## Architecture at a glance

A WebsiteCore installation is a single Go binary plus three infrastructure services:

```text
                     +----------------------------+
   browser --------> | WebsiteCore binary         |
                     | (Gin API + embedded Vue SPA)|
                     +----+--------+--------+-----+
                          |        |        |
                    +-----+---+ +--+---+ +--+------------+
                    | PostgreSQL| | Redis| | Meilisearch |
                    |           | |      | | (optional)  |
                    +-----------+ +------+ +-------------+
```

- **Database** (required): PostgreSQL. See [database.md](database.md).
- **Redis** (required): caching, counters, phone verification codes.
- **Meilisearch** (optional but recommended): full-text search. When not enabled, search falls back to PostgreSQL `ILIKE` fuzzy matching.
- **Object storage**: local disk (`LocalOSS`) by default; AliOSS is available via feature flags.

All three infrastructure services are typically run with Docker; the application itself runs as a native binary under systemd. This is the recommended production layout and is documented in [production.md](production.md). A fully containerized alternative is documented in [docker-compose.md](docker-compose.md).

## Prerequisites for any deployment

1. A built binary (or a release package). Build with `make build TAGS='embed migration'` so the frontend assets and database migrations are compiled in. See [../INSTALL.md](../INSTALL.md).
2. A `config.yaml` next to the binary. Start from [../../config.yaml.sample](../../config.yaml.sample); a full reference is in [configuration.md](configuration.md).
3. Two secrets you must generate yourself:
   - `JWT.Secret` — `openssl rand -hex 24`. The process exits at startup if empty.
   - `AdminSettings.EncryptionKey` — a long random string used to encrypt secrets stored via the admin UI.

Before exposing any deployment to the public internet, work through [public-launch.md](public-launch.md).
