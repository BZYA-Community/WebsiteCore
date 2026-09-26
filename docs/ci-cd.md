# CI/CD

All automation runs on GitHub Actions. Workflow files live in `.github/workflows/`.

## Pipelines at a glance

| Workflow | Trigger | Purpose |
| --- | --- | --- |
| `ci.yml` | push to `main`/`beta`/`dev`, all pull requests | Hard gate: backend build/test/lint + frontend lint/build |
| `claude-review.yml` | PR opened / synchronized / reopened | AI code review following the embedded reviewer charter |
| `codeql.yml` | push/PR to `main`, weekly schedule | Security analysis (4 languages) |
| `weekly-report.yml` | Friday 12:00 UTC (20:00 Beijing), manual | Community weekly report to DingTalk |

Changes to `README.md` and `docs/**` do not trigger `ci.yml` — documentation-only PRs still get AI review.

## ci.yml — the hard gate

Two jobs run in parallel on `ubuntu-latest`:

**backend**

| Step | Command / tool |
| --- | --- |
| Setup Go | version from `go.mod`, module cache on |
| Prepare embed dir | `mkdir -p web/dist` (so `embed` builds compile without frontend assets) |
| Syntax check | `go build ./...` |
| Unit tests | `go test ./...` |
| Code quality | `golangci-lint@v1.64.8 run ./...` (config: `.golangci.yml` — govet, gofmt, goimports, ineffassign; `auto/` excluded) |

**frontend** (working directory `web/`)

| Step | Command / tool |
| --- | --- |
| Setup Node | Node 22 |
| Install | `npm install` |
| Lint | `npm run lint` (ESLint) |
| Build | `npm run build` (Vite) |

This is the automated subset of the BVT defined in [../CONTRIBUTING.md](../CONTRIBUTING.md). The visual checks (page overlap at multiple viewports) are not automated in CI; they are run locally via the Playwright scripts in `scripts/` and their results are pasted into the PR.

A red CI blocks merge for everyone, including maintainers. `main` is protected: no direct pushes, no force pushes, linear history, squash merges only.

## claude-review.yml — AI code review

Every PR gets an automated review by Claude (`anthropics/claude-code-action@v1`) following the reviewer charter (《AI 审查员守则》). The charter is not stored as a repository file: it is embedded in `.github/workflows/claude-review.yml` and written into the workspace at review time, so a PR cannot tamper with the standards it is judged by. Changing the review rules means changing the workflow file itself:

- A summary comment at the top of the PR, ending with a difficulty grade **S / M / L** (display-only; grades never auto-unlock permissions — promotion is confirmed by humans, see [governance.md](governance.md)).
- Inline comments for individual findings, each marked 🔴 (must fix before merge), 🟡 (respond with justification), or 🟢 (optional nit).
- Extra scrutiny on new dependencies, secrets in code, and Chinese UI copy appropriate for a youth community.
- First-time contributors get explicit positive feedback.

Handling rules: all 🔴 findings must be resolved; 🟡 findings need a reply; 🟢 are voluntary. The AI review complements — never replaces — the human merge decision.

## codeql.yml — security scanning

- Runs on push/PR to `main` and every Monday 19:30 UTC.
- Matrix: `actions`, `go` (autobuild), `javascript-typescript`, `python`.
- Results appear under the repository **Security → Code scanning** tab.

## weekly-report.yml — community weekly report

- Schedule: Friday 12:00 UTC = 20:00 Beijing time; also dispatchable manually from the Actions tab.
- Runs `scripts/weekly-report.sh`: collects PRs merged in the past 7 days, classifies them by conventional-commit prefix (`feat:` / `fix:` / other), builds a contributor leaderboard, and posts a Markdown message to DingTalk.
- Required repository secrets: `DINGTALK_WEBHOOK`; optional `DINGTALK_SECRET` (HMAC-SHA256 signing for the DingTalk bot).
- The report is the transparency layer for the promotion system: merged-PR counts, difficulty distribution, and review participation are all public.

## Dependabot

Configured in `.github/dependabot.yml`: Go modules only, weekly, targeting the `dev` branch, commit prefix `mod:`. Dependabot PRs go through the same CI + AI review + human merge flow as any other PR. Frontend dependencies are currently updated manually.

## Release and deployment flow

Per [governance.md](governance.md) section 5:

```text
merge to main → test environment auto-updates (pulls every 10 min) → internal acceptance
  → maintainer acceptance + owner decision → cut a Release tag
  → production pulls and deploys the tag (GitHub Environments approval required)
  → rollback = redeploy the previous Release tag
```

Production never tracks `main`; it only runs tagged releases. The deployment itself is manual and documented in [deploy/production.md](deploy/production.md).
