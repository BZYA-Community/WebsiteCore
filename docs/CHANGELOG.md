# Changelog

All notable changes to WebsiteCore are documented in this file, following [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). This fork has not cut a release yet; changes accumulate under `Unreleased`.

History of the upstream paopao-ce project (0.2.0 through 0.6.0+dev) is archived at [archive/upstream/CHANGELOG.md](archive/upstream/CHANGELOG.md) and is not continued here.

## Unreleased

The fork started from paopao-ce on 2026-09-23 (`chore: strip Docker/Tauri and rebrand to BZYA-Community/WebsiteCore`). Everything below happened between 2026-09-23 and 2026-09-26.

### Added

- Identity-group system with RBAC: guest / member / mentor / auditor / admin / operator roles, backend user management with mute and soft delete, and full role-change audit logs.
- Content moderation pipeline: posts, comments, replies, and nickname changes from regular users enter an audit queue; mentor and above are exempt; rejection returns content to private with resubmission; results are notified in-site. Admin pages: audit queue, user management.
- Conversational private messaging (WeChat/Bilibili style): session list with pinned system contacts, standalone chat window, read/unread state, paginated history, and identity-group messaging rules (member-to-member DMs disabled, first-message limits toward higher roles, lifted after a reply).
- System notification session: follow / comment / reply / audit / management notices unified into one conversation with jump links to posts and profiles.
- Course module: groups, courses, comment moderation, play counts, signed playback URLs, direct-to-OSS uploads; standalone Markdown long-form publishing page.
- Local development stack: `docker-compose.dev.yml` with pinned PostgreSQL 18.6 / Redis 7.4.11 / Meilisearch v1.54.0, `make deps-*` targets, and migration-based schema setup (`make migrate`).
- Admin-managed site settings center (`/#/admin/settings`) with encrypted secret storage (`AdminSettings.EncryptionKey`, AES-GCM).
- Bootstrap operator account provisioning from configuration (`Operator` section).
- CI/CD: rebuilt GitHub Actions pipeline (Go build/test/golangci-lint + frontend ESLint/Vite build), AI code review workflow based on `CLAUDE.md`, CodeQL scanning, weekly DingTalk community report, issue and PR templates.
- End-to-end and visual verification scripts under `scripts/` (audit flow, course flow, whisper permission matrix, sidebar overlap at 830px, 7-viewport width measurement).
- ESLint (flat config) for the frontend alongside Biome formatting; Artplayer as the site-wide video player.

### Changed

- Desktop middle column adapts to viewport width; right sidebar breakpoint moved to 1140px.
- Configuration loading: embedded defaults plus external overrides; conf initialization failures return an error instead of killing the process (#14).
- Post visibility checks consolidated into `CanViewTweet` (#12); user status validated across the JWT chain so banned users lose access immediately (#26).
- Direct-message risk control fails closed when Redis is unavailable (#15).
- Repository rebranded to BZYA-Community/WebsiteCore; documentation restructured (upstream docs archived under `docs/archive/upstream/`).
- AI reviewer charter moved out of the repository root (`CLAUDE.md` removed) into `.github/workflows/claude-review.yml`; the workflow writes it into the workspace at review time, so PRs can no longer alter the standards they are reviewed against.

### Removed

- SQLite database support (binary size reduced by ~5.5 MB).
- Wallet/payment features (Alipay) — the platform is non-commercial.
- Friendship system — private messaging rules now derive from identity groups; migration `remove_friendship` drops the legacy schema.
- Docker/Tauri packaging inherited from upstream (re-added in documentation form for Compose deployments).

### Fixed

- LocalOSS object keys are validated against path traversal before persistence (#25).
- Swallowed/misused error returns corrected across servants (#13).
- Search results missing `audit_status` no longer mark posts as pending incorrectly.
- Page flicker on navigation (scrollbar-induced layout shift), keep-alive stacking, blank tabs, stuck spinner.
- Publish/registration validation returns HTTP 400 properly; admin sidebar overlap at narrow widths.
- All 5 high-severity CodeQL alerts resolved, including clear-text logging of sensitive information.

### Security

- JWT algorithm verification added; hardcoded secrets removed from the embedded configuration.
- `JWT.Secret` is mandatory — the server refuses to start without it.
- Banned-user enforcement moved into the authentication chain (#26).
