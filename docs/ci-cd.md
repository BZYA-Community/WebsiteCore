# CI/CD

All automation runs on GitHub Actions. Workflow files live in `.github/workflows/`.

## Pipelines at a glance

| Workflow | Trigger | Purpose |
| --- | --- | --- |
| `ci.yml` | push to `main`/`beta`/`dev`, all pull requests | Backend build/test/lint, frontend lint/locales/tests/build, then AI review for same-repository PRs |
| `codeql.yml` | push/PR to `main`, weekly schedule | Security analysis (4 languages) |
| `weekly-report.yml` | Friday 12:00 UTC (20:00 Beijing), manual | Community weekly report to DingTalk |

Pushes changing only `README.md` and `docs/**` do not trigger `ci.yml`. Pull requests, including documentation-only PRs, run the checks. Fork PRs skip the review job because it requires repository secrets and write permissions.

## ci.yml — the hard gate

Two jobs run in parallel on `ubuntu-latest`:

**backend**

| Step | Command / tool |
| --- | --- |
| Setup Go | version from `go.mod`, module cache on |
| Prepare embed dir | `mkdir -p web/dist` (so `embed` builds compile without frontend assets) |
| Syntax check | `go build ./...` |
| Unit tests | `go test ./...` |
| Code quality | `golangci-lint@v2.14.0 run ./...` (config: `.golangci.yml` — govet and ineffassign; generated `auto/` excluded from lint) |

**frontend** (working directory `web/`)

| Step | Command / tool |
| --- | --- |
| Setup Node | Node 22 |
| Install | `corepack enable`, then `yarn install --frozen-lockfile` |
| Lint | `npm run lint` (ESLint) |
| Locale packs | `npm run i18n:check` |
| Regression tests | `yarn test:permissions`, `yarn test:media`, `yarn test:courses`, `yarn test:sidebar`, `yarn test:admin` |
| Build | `npm run build` (Vite) |

This is the automated subset of the [BVT](development.md#build-verification-before-a-pr). Visual checks at multiple viewports run locally via the Playwright scripts in `scripts/` and `web/scripts/`; attach their results to the PR.

A red CI blocks merge for everyone, including maintainers. `main` is protected: no direct pushes, no force pushes, linear history, squash merges only.

## ci.yml review job — AI code review

Same-repository PRs get an automated review by Claude (`anthropics/claude-code-action@v1`) after both deterministic CI jobs finish, including when checks fail. The reviewer charter is embedded in `.github/workflows/ci.yml` and written into the workspace at review time. Changing the review rules means changing the workflow file itself:

- A summary comment at the top of the PR, ending with a difficulty grade **S / M / L** (display-only; grades do not change permissions).
- Inline comments for individual findings, each marked 🔴 (must fix before merge), 🟡 (respond with justification), or 🟢 (optional nit).
- Extra scrutiny on new dependencies, secrets in code, and Chinese UI copy appropriate for a youth community.

Handling rules: all 🔴 findings must be resolved; 🟡 findings need a reply; 🟢 are voluntary. The AI review complements — never replaces — the human merge decision.

## codeql.yml — security scanning

- Runs on push/PR to `main` and every Monday 19:30 UTC.
- Matrix: `actions`, `go` (autobuild), `javascript-typescript`, `python`.
- Results appear under the repository **Security → Code scanning** tab.

## weekly-report.yml — community weekly report

- Schedule: Friday 12:00 UTC = 20:00 Beijing time; also dispatchable manually from the Actions tab.
- Runs `scripts/weekly-report.sh`: collects PRs merged in the past 7 days, classifies them by conventional-commit prefix (`feat:` / `fix:` / other), builds a contributor leaderboard, and posts a Markdown message to DingTalk.
- Required repository secrets: `DINGTALK_WEBHOOK`; optional `DINGTALK_SECRET` (HMAC-SHA256 signing for the DingTalk bot).

## Dependabot

Configured in `.github/dependabot.yml`: Go modules only, weekly, targeting the `dev` branch, commit prefix `mod:`. Dependabot PRs go through the same CI + AI review + human merge flow as any other PR. Frontend dependencies are currently updated manually.

## Release and deployment flow

Release and deployment are manual; the repository workflows do not deploy the application. Use tested release tags and keep the previous binary and database backup for rollback, as documented in [deploy/production.md](deploy/production.md).
