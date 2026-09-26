# Contributing Guide

This repository is run by students. Anyone can contribute, and every contribution is recorded in the automatically generated weekly report (Fridays, 20:00 Beijing time).

New here? Read this page end to end, then pick a `good first issue`. A first PR that fixes one line of documentation still counts — merging a contribution on the day it is opened is a community tradition.

## Code of conduct

This platform serves a community of minors. Be strict about code, kind about people: explain, don't just reject. UI copy is written in Chinese and must stay age-appropriate.

## Roles and promotion

Roles depend only on contribution records — never on age or grade.

| Role | Permissions | How to get it |
| --- | --- | --- |
| Peripheral contributor | Open issues, docs, translations, testing feedback | Join the community |
| Contributor | Push branches, open PRs | 1 merged PR |
| Reviewer (merge rights) | Approve and merge PRs | (1) ≥ 5 merged PRs; (2) ≥ 1 of them graded M or L; (3) ≥ 3 effective reviews; (4) confirmation by the tech committee / maintainer |
| Maintainer | Merge, deploy, revise rules | Nominated by the committee, appointed by the owner. Currently: @Alumin-Hydro |

**AI supplies the numbers, people grant the rights.** Difficulty grades (S/M/L) are published by the AI review on every PR. They are display information only — they never unlock anything automatically; the last step of every promotion is a human decision.

Anti-gaming: splitting one piece of work into multiple PRs may be counted as one by reviewers; disputed difficulty grades are settled by human review. Details: [`docs/governance.md`](docs/governance.md), section 3.

Everyone — including maintainers — goes through PRs. **Direct pushes to `main` are forbidden.**

## Pull request workflow

1. Fork the repository and create a branch from `dev` (or `main` if `dev` is unavailable).
2. Make your change. One PR does exactly one thing and links its issue (`closes #xx`).
3. Commit messages use conventional prefixes: `feat:` / `fix:` / `docs:` / `refactor:` / `chore:`. The weekly report classifies by them.
4. Run the **BVT** (below) and keep the output for the PR description.
5. Open the PR with the matching template (feature / bugfix / documentation).
6. The PR then passes three layers: CI (hard gate) → AI review (24/7) → human merge decision.
7. Review findings are graded: 🔴 must be fixed before merge; 🟡 requires a written response; 🟢 is optional.

## BVT — Build Verification Test (mandatory)

**No PR merges without a passing BVT.** CI automates part of it; the rest you run locally and paste into the PR template. If a check genuinely does not apply, say so in the PR instead of silently skipping it.

### Backend BVT — every PR

```sh
go build ./...           # syntax / compilation check — must be clean
go vet ./...
golangci-lint run ./...  # v1.64.8, same as CI
go test ./...
```

Additional checks when you touch the corresponding areas:

| You changed... | Also run / verify |
| --- | --- |
| `mirc/` (API definitions) | `make gen-mir`, and confirm `auto/` contains no hand edits |
| Database schema | Migration pair for **both** dialects under `scripts/migration/{postgres,mysql}/`, verified locally with a `migration`-tagged build (`make migrate`) |
| Configuration keys | `internal/conf/config.yaml` (embedded) and `config.yaml.sample` updated together |
| Behavior covered by E2E scripts | The relevant `scripts/test_*.py`, paste PASS/FAIL counts into the PR |

### Frontend BVT — every PR touching `web/`

```sh
cd web
npm install
npm run lint             # ESLint, 0 errors
npm run build            # Vite production build succeeds
```

Then the visual checks — **page content must not overlap or overflow** at any of the standard viewports (1920 / 1600 / 1366 / 1200 / 1000 / 821 / 375):

```sh
python scripts/verify_sidebar_830.py   # sidebar overlap check at the 830px breakpoint
python scripts/measure_width.py        # column widths across 7 viewports
python scripts/screenshot.py           # screenshots of key pages for manual comparison
```

Requires Python 3 with Playwright (`pip install playwright && playwright install chromium`) and a local instance running at `http://127.0.0.1:8008` (see [`docs/deploy/local.md`](docs/deploy/local.md)). Attach the screenshots or script output to the PR as evidence.

### Documentation BVT — docs-only PRs

Verify every link you added or touched resolves, and that referenced commands match the current `Makefile` / `scripts/`. CI is skipped for `docs/**` changes, so this is on you.

## Rules that block a merge outright

- Any secret, token, password, or real personal data (phone numbers, addresses) in the diff.
- New dependencies without justification in the PR description; heavyweight dependencies are rejected by default.
- Failing CI, or unresolved 🔴 review findings.
- Generated files (`auto/`, `web/dist/`) hand-edited or committed as build artifacts.

## Related documents

| Topic | Document |
| --- | --- |
| What the platform is and must become (requirements baseline) | [`docs/requirements-starisle-v2.0.md`](docs/requirements-starisle-v2.0.md) |
| Who has which rights, review and promotion mechanics | [`docs/governance.md`](docs/governance.md) |
| Dev environment, architecture, codegen, testing | [`docs/development.md`](docs/development.md) |
| CI pipelines and AI review | [`docs/ci-cd.md`](docs/ci-cd.md) |
| Security vulnerability reporting | [`SECURITY.md`](SECURITY.md) |

## Commit attribution

Commit from your own GitHub account (enable the noreply email: GitHub → Settings → Emails → *Keep my email addresses private*). Only commits linked to your account count toward your contribution graph and the promotion ledger.
