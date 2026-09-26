# Public Launch Checklist

Everything that must be true before a WebsiteCore instance accepts traffic from the public internet. Work top to bottom; the sections marked **blocking** will cause real damage or real legal trouble if skipped.

## Network exposure (blocking)

- [ ] The firewall / cloud security group allows inbound traffic on **80 and 443 only** (plus SSH from known addresses).
- [ ] `WebServer.HttpIp` is `127.0.0.1` and Nginx is the only public entry point.
- [ ] PostgreSQL, Redis, Meilisearch have **no published ports** beyond `127.0.0.1` (or none at all in a compose network). Verify from outside the server:
      `nmap -Pn -p 5432,6379,7700,8008,6060,6080,8011 your-server-ip`
- [ ] Auxiliary servers stay disabled or internal: `Pprof` (6060), `Metrics` (6080), `Docs` (8011) are not reachable from the internet. Pprof in particular allows arbitrary profiling of the live process.
- [ ] HTTPS is enforced: valid certificate, HTTP redirects to HTTPS, `LocalOSS.Secure: true` and `LocalOSS.Domain` set to the public domain so attachment URLs are https.

## Secrets (blocking)

- [ ] `JWT.Secret` is a fresh random value (`openssl rand -hex 24`), not the sample placeholder. Leaking it allows forging any user's session.
- [ ] `AdminSettings.EncryptionKey` is a fresh long random string.
- [ ] Database, Redis (`requirepass`), and Meilisearch (`MEILI_MASTER_KEY`) passwords are strong and unique per service.
- [ ] `config.yaml` and compose `.env` are mode `600`, owned by the service user, and **never committed to git**.
- [ ] The Operator bootstrap password has been changed through the UI and removed from `config.yaml`.
- [ ] No key, token, password, or real phone number appears anywhere in the repository — this is an automatic reject in code review (see the reviewer charter embedded in `.github/workflows/claude-review.yml`).

## Application hardening

- [ ] `App.RunMode` and `Logger.Level` set to `release` / `error`.
- [ ] Content moderation is on: `Audit.Enabled: true`. For a youth community this is not optional — regular users' posts, comments, replies, and nickname changes must pass moderation before becoming public.
- [ ] Decide registration policy: open (`WebProfile.AllowUserRegister`) vs. closed (`Web:DisallowUserRegister` feature). If open, moderation and reporting paths must be staffed.
- [ ] Phone binding: either configure SMS properly ([sms.md](sms.md)) or disable `WebProfile.AllowPhoneBind`. Never leave it enabled without the `Sms` feature — any code would be accepted.
- [ ] Daily private-message cap (`App.MaxWhisperDaily`) and captcha attempt limits are at sane values.
- [ ] `client_max_body_size` in Nginx matches the largest upload you intend to allow.

## Compliance (blocking for servers in mainland China)

- [ ] **ICP filing (ICP 备案)** completed for the domain, and the filing number is displayed in the site footer (`WebProfile.Copyright*` settings are the natural place).
- [ ] **Public security filing (公安备案)** within 30 days of launch, where required.
- [ ] Privacy policy published, written for a minor-user audience and their guardians: what is collected (account, posts, phone number if bound), why, how long it is kept, how to request deletion.
- [ ] Data minimization respected per [../governance.md](../governance.md) section 6: exporting or sharing user data requires maintainer approval; phone numbers are collected only for binding.
- [ ] An abuse/report contact is reachable (email or in-site), and someone checks it.
- [ ] Real-name/phone-binding requirements, if enabled, follow current regulations for community platforms.

## Data durability

- [ ] Daily automated database backups (cron + `pg_dump`, see [database.md](database.md)), stored off-box, mode 600.
- [ ] `custom/` directory (LocalOSS uploads, logs) or the object storage bucket is in the backup scope.
- [ ] **A restore has been rehearsed** into a scratch database, and the application was verified to start against it.
- [ ] Rollback procedure written down: previous binary + pre-migration dump (see [production.md](production.md), Upgrades).

## Operational readiness

- [ ] systemd `Restart=always` verified (kill the process, watch it come back).
- [ ] Logs are being written and rotated (`LoggerFile` under `custom/`, plus journald for the service itself). Consider `LoggerOtlp`/Sentry for centralized error tracking — do not expose their endpoints publicly.
- [ ] Monitoring in place (uptime check on the homepage; Prometheus scrape of the internal `Metrics` port if enabled).
- [ ] Upgrade path agreed: production deploys only tagged releases, behind the GitHub Environments approval flow described in [../governance.md](../governance.md) section 5.
- [ ] Subscribe to dependency updates: Dependabot runs weekly against the `dev` branch; security fixes flow through the normal PR + review process ([../ci-cd.md](../ci-cd.md)).
- [ ] Vulnerability reporting channel published: GitHub private security report, per [../../SECURITY.md](../../SECURITY.md).

## Launch-day smoke test

1. Register a fresh account from an external network.
2. Post content as a regular user; confirm it lands in the audit queue (`/#/admin/audit`) and becomes visible only after approval.
3. Upload an image and a video; confirm URLs are https on your domain.
4. Search a keyword; confirm Meilisearch returns results.
5. Send a private message between two test accounts; confirm the permission rules behave (see the whisper matrix in `scripts/test_whisper_matrix.py`).
6. Log in as Operator; confirm admin pages (`/#/admin/users`, `/#/admin/settings`) load and role changes are logged.
7. Force a backup, restore it to a scratch instance, and confirm the site comes up.
