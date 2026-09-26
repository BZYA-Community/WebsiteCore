# Database Guide

## Supported databases

| Engine | Status | Default |
| --- | --- | --- |
| PostgreSQL | Fully supported, default choice | `postgres:18.6` in the dev stack |
| MySQL | Fully supported (utf8mb4) | — |

Enable exactly one via the `Features` list (`Postgres` or `MySQL`) and fill in the matching config section. SQLite is not supported in this fork.

For new deployments choose PostgreSQL: it is what the development stack and CI-adjacent tooling standardize on. MySQL works identically at the application level; pick it only if your infrastructure already standardizes on MySQL.

## Running the database with Docker

For local development, `make deps-up` starts everything (see [local.md](local.md)). For a server, run the dependency stack with a production compose file — a complete example is in [production.md](production.md). Key differences from the dev stack:

- Strong, unique passwords (not `paopao`/`paopao`).
- Ports bound to `127.0.0.1` (single-host layout) or not published at all (compose-internal network).
- Named volumes for data, included in your backup plan.
- Pinned image tags.

## Schema management

### Migrations

The schema is defined by versioned SQL migrations under `scripts/migration/`:

```text
scripts/migration/
├── postgres/   0001_initialize_schema ... 0023_remove_friendship   (23 pairs)
└── mysql/      0001_initialize_schema ... 0024_remove_friendship   (24 pairs)
```

Each migration is a pair of `NNNN_name.up.sql` / `NNNN_name.down.sql` files, embedded into the binary at build time (only when the `migration` build tag is present) and executed by [golang-migrate](https://github.com/golang-migrate/migrate). The applied version is tracked in the `p_schema_migrations` table (`Database.TablePrefix` + `schema_migrations`).

> The MySQL directory contains one extra migration (`0004_optimize_idx`) with no PostgreSQL counterpart; numbering therefore differs between the two dialects. This is expected.

### Running migrations

```sh
make migrate
```

This compiles the binary with `-tags migration` and runs the `migrate` subcommand against the database configured in `config.yaml`. Run it:

- once after first deploying (creates the full schema);
- after every upgrade that ships new migration files.

A production binary built with `TAGS='embed migration'` can also migrate automatically at startup: add `"Migration"` to `Features.Default` and the schema is upgraded before the server starts. Without the build tag, requesting the `Migration` feature fails fast with an explicit error.

### Adding a migration (contributors)

Schema changes must go through migrations — never edit the production database by hand (see [../governance.md](../governance.md), section 6).

1. Create the next-numbered pair in **both** dialects: `scripts/migration/postgres/NNNN_name.{up,down}.sql` and `scripts/migration/mysql/NNNN_name.{up,down}.sql`. If a change is genuinely single-dialect, the linked issue must say so explicitly.
2. Write a working `down` migration.
3. Verify locally with a build that carries the tag: `make migrate`, or start the server with `TAGS='migration'` and the `Migration` feature.
4. Back up before applying anywhere that holds real data.

### Legacy bootstrap SQL

`scripts/paopao-{mysql,postgres}.sql` are full-schema snapshots inherited from upstream. They are kept for reference; prefer `make migrate` for any new installation.

## Connection settings

PostgreSQL:

```yaml
Features:
  Default: [..., "Postgres"]

Postgres:
  User: websitecore
  Password: "<strong password>"
  DBName: websitecore
  Schema: public
  Host: 127.0.0.1
  Port: 5432
  SSLMode: disable      # prefer require / verify-full when your setup allows
```

MySQL:

```yaml
Features:
  Default: [..., "MySQL"]

MySQL:
  Username: websitecore
  Password: "<strong password>"
  Host: 127.0.0.1:3306
  DBName: websitecore
  Charset: utf8mb4
  ParseTime: True
  MaxIdleConns: 10
  MaxOpenConns: 30
```

`Database.LogLevel` controls GORM query logging (`silent|error|warn|info`); use `error` or `silent` in production. `Database.TablePrefix` defaults to `p_` — if you change it, change it before the first migration, not after.

## Backup and restore

Back up before every migration and on a daily schedule. Backup files must be stored outside the repository with restrictive permissions (mode 600 per governance policy).

PostgreSQL:

```sh
# Backup (custom format, compressed, restorable selectively)
docker exec <pg-container> pg_dump -U websitecore -Fc websitecore > websitecore-$(date +%F).dump
chmod 600 websitecore-*.dump

# Restore
docker exec -i <pg-container> pg_restore -U websitecore -d websitecore --clean --if-exists < websitecore-2026-09-26.dump
```

MySQL:

```sh
docker exec <mysql-container> mysqldump -u websitecore -p --single-transaction websitecore > websitecore-$(date +%F).sql
chmod 600 websitecore-*.sql

docker exec -i <mysql-container> mysql -u websitecore -p websitecore < websitecore-2026-09-26.sql
```

Also back up:

- `config.yaml` (contains connection settings and secrets — store it encrypted);
- the `LocalOSS` upload directory (`custom/data/paopao-ce/oss` by default) or your object storage bucket;
- Redis is a cache and can be rebuilt, but phone-verification codes and counters live there — losing it is tolerable, restoring it is not required.

### Restore drill

A backup you have never restored is not a backup. Before launch, practice a full restore into a scratch database and verify the server starts and serves the timeline against it (see [public-launch.md](public-launch.md)).
