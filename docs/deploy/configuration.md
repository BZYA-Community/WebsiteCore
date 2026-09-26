# Configuration Reference

WebsiteCore is configured through three layers, applied in this order (later layers win):

```text
1. Embedded defaults    internal/conf/config.yaml, compiled into the binary
2. External YAML file   ./custom/config.yaml, or ./config.yaml if that does not exist
3. Admin site settings  stored in the database, edited at /#/admin/settings
```

The external YAML file therefore only needs to contain the keys you want to change. Ports, databases, and feature selection stay file-based; most product and operational settings are managed in the admin UI and persisted to the database (they are re-applied to the in-memory configuration at every startup).

## Required secrets

Two values must be set before first launch:

| Key | Generate with | Consequence if missing |
| --- | --- | --- |
| `JWT.Secret` | `openssl rand -hex 24` | The process exits at startup. |
| `AdminSettings.EncryptionKey` | a long random string | Secrets saved through the admin UI (search API keys, storage secrets, SMS key) cannot be encrypted or read back. |

`EncryptionKey` is hashed with SHA-256 into a 32-byte AES key; every field marked as secret in the admin UI is stored in the database as AES-GCM ciphertext with an `enc:v1:` prefix. Rotating it makes previously stored secrets unreadable — re-enter them in the admin UI after a rotation.

## The `Features` section

`Features` selects capability bundles ("feature suites"). `Default` is applied on every start; additional suites can be activated per-run with CLI flags.

```yaml
Features:
  Default: ["Web", "Frontend:EmbedWeb", "Meili", "LocalOSS", "Postgres", "BigCacheIndex", "LoggerFile"]
```

Runtime overrides:

```sh
./paopao serve                                  # default suite
./paopao serve --features sms                   # default + Sms
./paopao serve --no-default-features --features develop   # only the named suite
```

Feature names are case-insensitive. Every feature recognized by the codebase, grouped by domain:

| Domain | Feature names | Notes |
| --- | --- | --- |
| Sub-services | `Web`, `Docs`, `Pprof`, `Metrics` | `Web` (port 8008) is the main API and serves the SPA |
| Frontend | `Frontend:Web`, `Frontend:EmbedWeb` | `EmbedWeb` serves the frontend compiled into the binary (`embed` build tag) |
| Database | `Postgres` | the only supported database |
| Search | `Meili` | Meilisearch is the recommended engine; without it, search falls back to PostgreSQL `ILIKE` |
| Cache index | `BigCacheIndex`, `SimpleCacheIndex`, `RedisCacheIndex` | timeline caching strategy |
| Object storage | `LocalOSS`, `AliOSS` | enable exactly one (defaults to `LocalOSS`); sub-features `OSS:TempDir`, `OSS:Retention` control upload staging |
| SMS | `Sms` | enables phone verification via Juhe SMS; see [sms.md](sms.md) |
| Logging | `LoggerFile`, `LoggerOtlp` | |
| Observability | `Sentry`, `Pyroscope` | |
| Migration | `Migration` | auto-migrate schema at startup; requires a binary built with the `migration` tag |
| Registration | `Web:DisallowUserRegister` | closes public registration |

The full maturity/support matrix is in [../features-status.md](../features-status.md).

## Configuration sections

All sections below are defined in `internal/conf/setting.go`; defaults live in `internal/conf/config.yaml`. Only the sections relevant to your enabled features need to appear in your external `config.yaml`.

### Servers

Each sub-service has its own HTTP server block (`RunMode`, `HttpIp`, `HttpPort`, `ReadTimeout`, `WriteTimeout`):

| Section | Default port | Purpose |
| --- | --- | --- |
| `WebServer` | 8008 | main API + SPA (the only port users need) |
| `FrontendWebServer` | 8006 | standalone frontend static server (`Frontend:Web`) |
| `DocsServer` | 8011 | OpenAPI docs (`docs` build tag) |
| `PprofServer` | 6060 | pprof (`Pprof` feature) |
| `MetricsServer` | 6080 | Prometheus metrics (`Metrics` feature) |

Keep every port except `WebServer` bound to `127.0.0.1` or an internal interface in production.

### App behavior (`App`)

`RunMode` (`debug`/`release`), `MaxCommentCount`, `MaxWhisperDaily` (daily private-message cap), `MaxCaptchaTimes`, `DefaultContextTimeout`, `DefaultPageSize`, `MaxPageSize`.

### Database (`Database`, `Postgres`)

`Database` holds shared settings: `LogLevel` (`silent|error|warn|info`) and `TablePrefix` (default `p_`).

```yaml
Postgres:
  User: paopao
  Password: ""        # required
  DBName: websitecore
  Schema: public
  Host: 127.0.0.1
  Port: 5432
  SSLMode: disable    # use require/verify-full in production when available
```

See [database.md](database.md) for deployment, migration, and backup.

### Redis (`Redis`)

```yaml
Redis:
  InitAddress:
    - 127.0.0.1:6379
  Username:
  Password:
  SelectDB:
  ConnWriteTimeout: 60
```

### Auth (`JWT`)

`Secret` (required), `Issuer`, `Expire` (seconds, default 86400).

### Search (`Meili`, `TweetSearch`)

```yaml
Meili:
  Host: 127.0.0.1:7700
  Index: paopao-data
  ApiKey: ""          # Meilisearch master/admin key
  Secure: false       # true = https
```

`TweetSearch` tunes the async indexing bridge: `MaxUpdateQPS` (10-10000, default 100) and `MinWorker` (5-1000, default 10). The `Meili` values are also editable in the admin UI (search group). When the `Meili` feature is not enabled, search falls back to direct PostgreSQL `ILIKE` fuzzy matching — no external search service required.

### Object storage (`ObjectStorage`, `LocalOSS`, `AliOSS`)

`ObjectStorage` is shared: `RetainInDays` (temp object expiry) and `TempDir`. Then one backend:

| Section | Keys |
| --- | --- |
| `LocalOSS` | `SavePath`, `Secure`, `Bucket`, `Domain` |
| `AliOSS` | `Endpoint`, `AccessKeyID`, `AccessKeySecret`, `Bucket`, `Domain` |

All of these are also manageable from the admin UI (storage group). `LocalOSS.SavePath` defaults to `custom/data/paopao-ce/oss` relative to the working directory — back this directory up. When no storage feature is enabled, `LocalOSS` is used automatically.

### SMS (`SmsJuhe`)

See [sms.md](sms.md). Keys: `Gateway`, `Key`, `TplID`, `TplVal`.

### Caching (`Cache`, `CacheIndex`, `SimpleCacheIndex`, `BigCacheIndex`, `RedisCacheIndex`)

`Cache` sets per-domain Redis TTLs (user info, timelines, comments, unread messages, online users...). The `*CacheIndex` sections configure the public timeline cache selected by the corresponding feature; e.g. `BigCacheIndex` has `MaxIndexPage`, `HardMaxCacheSize` (MB), `ExpireInSecond`.

### Background workers (`EventManager`, `MetricManager`, `JobManager`)

Worker pool sizes and buffer limits; `JobManager` holds cron expressions (`MaxOnlineInterval`, `UpdateMetricsInterval`, both default `@every 5m`).

### Logging and observability (`Logger`, `LoggerFile`, `LoggerOtlp`, `Sentry`, `Pyroscope`)

`Logger.Level` accepts `panic|fatal|error|warn|info|debug|trace`. Each sink section carries its own connection settings; use `release` + `error` level in production.

### Content moderation (`Audit`)

```yaml
Audit:
  Enabled: true   # defaults to true when the section is absent
```

When enabled, posts/comments/replies/nickname changes from regular users enter a moderation queue and become public only after approval; users with the Mentor role or above are exempt. Every decision is written to an audit log. Editable live from the admin UI (audit group). For a youth-oriented community this should stay enabled.

### Bootstrap operator account (`Operator`)

```yaml
Operator:
  Username: kiana   # empty username disables the mechanism
  Password: ""      # 6-16 chars
```

At every startup the server ensures this account exists with the Operator role and admin flag: it is created if missing (when a password is set), and its password is reset if it no longer matches the config value. Set the password in production, then consider clearing it from the file once the account exists so the password is no longer reset on restart.

### Frontend profile (`WebProfile`)

Site-level UI knobs: trends bar, attachments/videos allowed, registration allowed, phone binding allowed, tweet length and ellipsis sizes, default tweet visibility, message polling interval, copyright strings. These are managed in the admin UI (web/profile group); the YAML values only seed the first boot. Note: `AllowUserRegister` and `AllowPhoneBind` are bootstrap-only (read from YAML at startup, not editable at runtime).

## Admin-managed settings (`/#/admin/settings`)

Settings registered in `internal/sitesetting/registry.go` are grouped in the admin UI as: web/profile, app (general, limits), search (bridge, meili), storage (common + each backend), notifications/sms_juhe, and audit.

Each setting has an apply mode:

| Mode | Meaning |
| --- | --- |
| `live` | takes effect immediately |
| `restart_required` | saved to the database, active after the next process restart |
| `bootstrap_only` | read from YAML at startup only; shown read-only |

Secret fields (API keys, passwords, `sms_juhe.key`) are stored encrypted with `AdminSettings.EncryptionKey`.

## Keeping sample and embedded config in sync

`config.yaml.sample` (repository root) is the canonical template for new deployments; `internal/conf/config.yaml` is the embedded default compiled into the binary. When you add or rename a configuration key, update **both** files in the same PR — CI reviewers check for this, and it is an item on the PR template checklist.
