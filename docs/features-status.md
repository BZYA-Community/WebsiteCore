# Feature Status

WebsiteCore assembles its capabilities from feature flags declared in `config.yaml` under `Features` (see [deploy/configuration.md](deploy/configuration.md)). This page is the authoritative matrix of what each flag does and how well it is supported in this fork.

Status legend:

- **stable** — used in production paths, covered by CI builds and/or verification scripts
- **works** — functional, less exercised; report issues normally
- **legacy** — inherited from upstream paopao-ce, kept compiling but not recommended for new deployments

The default suite shipped in `config.yaml.sample`:

```yaml
Features:
  Default: ["Web", "Frontend:EmbedWeb", "Meili", "LocalOSS", "Postgres", "BigCacheIndex", "LoggerFile"]
```

## Sub-services

| Feature | Port | Status | Notes |
| --- | --- | --- | --- |
| `Web` | 8008 | stable | Main REST API; the core of every deployment |
| `Frontend:EmbedWeb` | (via Web) | stable | Serves the Vue SPA compiled into the binary (`embed` build tag) |
| `Frontend:Web` | 8006 | works | Standalone static frontend server |
| `Docs` | 8011 | works | OpenAPI docs (`docs` build tag); spec still partially reflects upstream |
| `Pprof` | 6060 | works | Profiling; internal networks only |
| `Metrics` | 6080 | works | Prometheus endpoint; internal networks only |

## Database

| Feature | Status | Notes |
| --- | --- | --- |
| `Postgres` | stable | Default and only supported database; dev stack pins PostgreSQL 18.6 |

MySQL and SQLite support were removed in this fork (2026-09-25).

## Search

| Feature | Status | Notes |
| --- | --- | --- |
| `Meili` | stable | Meilisearch; recommended, in the default suite and dev stack (v1.54.0) |
| SQL fallback | stable | When `Meili` is not enabled, search falls back to PostgreSQL `ILIKE` fuzzy matching — no external service required |

Indexing goes through an async bridge tuned by `TweetSearch` (`MaxUpdateQPS`, `MinWorker`).

## Cache index (public timeline)

| Feature | Status | Notes |
| --- | --- | --- |
| `BigCacheIndex` | stable | In-process BigCache; default choice |
| `SimpleCacheIndex` | works | Minimal in-process index |
| `RedisCacheIndex` | works | Shared cache across instances; use when running multiple replicas |

## Object storage

| Feature | Status | Notes |
| --- | --- | --- |
| `LocalOSS` | stable | Local disk under `custom/`; default fallback |
| `AliOSS` | works | Alibaba Cloud OSS |
| `OSS:TempDir` | works | Stage uploads in a temp directory first |
| `OSS:Retention` | works | Set retain-until metadata on objects |

Enable exactly one backend. When no storage feature is enabled, `LocalOSS` is used automatically. All backends are configurable from the admin UI (storage group).

## Messaging

| Feature | Status | Notes |
| --- | --- | --- |
| `Sms` | works | Phone-number binding via Juhe SMS (聚合数据); the only built-in provider. See [deploy/sms.md](deploy/sms.md). Without this flag, phone verification is a no-op — disable `AllowPhoneBind` on public sites. |

## Logging

| Feature | Status | Notes |
| --- | --- | --- |
| `LoggerFile` | stable | File logs under `custom/`; default choice |
| `LoggerOtlp` | works | OpenTelemetry Protocol export (logs/traces/metrics) |

## Observability

| Feature | Status | Notes |
| --- | --- | --- |
| `Sentry` | works | Error tracking with stack traces |
| `Pyroscope` | works | Continuous profiling |

## Lifecycle

| Feature | Status | Notes |
| --- | --- | --- |
| `Migration` | stable | Auto-apply database migrations at startup; requires the `migration` build tag (the `migrate` subcommand always enables both) |
| `Web:DisallowUserRegister` | works | Close public registration |

## Removed in this fork

| Former feature | Reason |
| --- | --- |
| `Sqlite3` | Removed 2026-09-25 to slim the binary |
| `MySQL` | Removed 2026-09-27; PostgreSQL is the sole database |
| `Zinc` | Removed 2026-09-27; Meilisearch or SQL fallback cover all search needs |
| `Admin` (standalone 8014) | Removed 2026-09-27; Web 内的 `/v1/admin` 管理面板仍保留 |
| `SpaceX` (8012) | Removed 2026-09-27; upstream experimental service |
| `Bot` (8016) | Removed 2026-09-27; upstream bot service |
| `NativeOBS` (8018) | Removed 2026-09-27; direct upload service |
| `Mobile` (8020 gRPC) | Removed 2026-09-27; no mobile client in this fork |
| `MinIO`, `S3`, `COS`, `HuaweiOBS` | Removed 2026-09-27; LocalOSS 与 AliOSS 覆盖存储需求 |
| `LoggerZinc`, `LoggerMeili`, `LoggerOpenObserve` | Removed 2026-09-27; LoggerFile 与 LoggerOtlp 覆盖日志需求 |
| `Lightship` (open mode) | Deprecated upstream |
| `Alipay` / wallet | Platform is non-commercial (2026-09-25) |
| `Friendship` | Replaced by identity-group messaging rules (2026-09-26) |
| `Deprecated:OldWeb` | Old frontend removed upstream |
