# Documentation Index

Start here if you are looking for anything beyond the project overview in the [root README](../README.md).

## Getting started

| Document | For whom |
| --- | --- |
| [INSTALL.md](INSTALL.md) | Anyone running the project for the first time |
| [deploy/local.md](deploy/local.md) | Developers setting up a local stack |
| [development.md](development.md) | Contributors writing code |
| [../CONTRIBUTING.md](../CONTRIBUTING.md) | Everyone opening an issue or PR (roles, BVT, review rules) |

## Deployment and operations

| Document | Topic |
| --- | --- |
| [deploy/README.md](deploy/README.md) | Deployment overview and path selection |
| [deploy/configuration.md](deploy/configuration.md) | Full `config.yaml` reference, feature flags, admin-managed settings |
| [deploy/database.md](deploy/database.md) | PostgreSQL setup, migrations, backup and restore |
| [deploy/sms.md](deploy/sms.md) | SMS provider (Juhe) and phone-binding configuration |
| [deploy/production.md](deploy/production.md) | Recommended production layout: native binary + Dockerized dependencies + Nginx |
| [deploy/docker-compose.md](deploy/docker-compose.md) | Fully containerized alternative |
| [deploy/public-launch.md](deploy/public-launch.md) | Checklist before opening a site to the public internet |
| [ci-cd.md](ci-cd.md) | GitHub Actions pipelines, AI review, weekly report |

## Reference

| Document | Topic |
| --- | --- |
| [features-status.md](features-status.md) | Feature flag maturity matrix |
| [CHANGELOG.md](CHANGELOG.md) | WebsiteCore change history |
| [openapi/](openapi/) | OpenAPI assets served at `/docs/openapi` with the `docs` build tag |
| [governance.md](governance.md) | Community governance charter (Chinese): review process, promotion rules, release flow |
| [requirements-starisle-v2.0.md](requirements-starisle-v2.0.md) | Product requirements baseline (Chinese): what the platform is and must become |
| [archive/](archive/) | Historical upstream paopao-ce documents, kept for reference only |

## Reading paths

**New contributor**: root [README](../README.md) → [INSTALL.md](INSTALL.md) → [development.md](development.md) → [../CONTRIBUTING.md](../CONTRIBUTING.md). Skim [governance.md](governance.md) to understand how reviews and promotions work.

**Operator / self-hoster**: [INSTALL.md](INSTALL.md) → [deploy/production.md](deploy/production.md) → [deploy/configuration.md](deploy/configuration.md) → [deploy/database.md](deploy/database.md) → [deploy/public-launch.md](deploy/public-launch.md).

**API / integration work**: [openapi/](openapi/) (run with `make run TAGS='docs'`) → [development.md](development.md) (how APIs are defined in `mirc/` and generated into `auto/`).

## Conventions

- Documentation is maintained in English; the root [README_ZH.md](../README_ZH.md) provides a Chinese overview, and the two community charters (governance, requirements) remain in Chinese as authored.
- Docs live next to the code they describe and change through the same PR flow. If a PR changes behavior, it updates the affected page in the same change.
- Anything under `archive/` is frozen history; do not link to it from user-facing guides.
