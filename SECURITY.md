# Security Policy

## Supported versions

This project is a community-maintained fork. Security updates are provided for the `main` branch only; we do not maintain separate release lines.

| Version | Supported |
| --- | --- |
| `main` branch (latest commit) | Yes |
| Historical commits / branches | No |
| Upstream paopao-ce and other derivatives | No — report to the upstream project |

## Reporting a vulnerability

Please report security vulnerabilities privately:

- **Preferred channel**: GitHub private vulnerability reporting — open this repository's **Security** tab, then **Report a vulnerability**, and include details plus reproduction steps.
- Do not disclose unpatched vulnerability details in issues, discussions, or any public channel.

### Handling timeline

| Stage | Target |
| --- | --- |
| Acknowledge the report | within 7 days |
| Initial triage (accept / decline, severity) | within 14 days |
| Fix and public note | as soon as practical, typically within 30 days |

- If your report is accepted, we will credit you in the fix commit message unless you prefer to stay anonymous.
- If it is declined, we will explain why (for example: an upstream paopao-ce issue, local-development-only impact, or not reproducible).
- Weak passwords or misconfigurations in private deployments are not code vulnerabilities; harden your instance following the security checklist in [`docs/deploy/public-launch.md`](docs/deploy/public-launch.md).

## Scope notes

Things we treat as security-relevant in this fork include: authentication/JWT bypasses, authorization flaws around identity groups and the content-audit pipeline, private-message permission leaks, path traversal in object storage, injection issues, and secret leakage in code or logs.
