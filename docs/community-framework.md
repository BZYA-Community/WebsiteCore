# Community framework development baseline

This document records the accepted direction for the modular WebsiteCore refactor. New repository artifacts use English; the product keeps Chinese and English translations.

| Phase | Deliverable | Tracking |
| --- | --- | --- |
| 1 | Dynamic identity groups, explicit permissions, independent operator, immediate revocation, audited management | [Local issue 001](issues/001-identity-permissions.md) |
| 2 | Optional course module, nested categories, courses and lessons, multiple attachments, moderated Q&A | Pending Phase 1 acceptance |
| 3 | Random review assignment, configurable 48-hour deadline, atomic reassignment and missed-review accounting | Pending Phase 1 acceptance |
| 4 | Exclusive email/phone verification mode, default Aliyun email, secure one-time verification | Pending Phase 1 acceptance |
| 5 | Upload limits, client video compression, file validation, trusted AliOSS/LocalOSS resource uploads | Pending Phase 2 contracts |

## Confirmed defaults

Verification of the configured contact method automatically grants Member identity. Unverified accounts use Guest identity. Groups and permissions are managed in the backend UI. Ordinary administrators have no default review exemption. Anonymous visitors can browse public posts and the course catalog; members can view lessons, publish, upload and initiate messages.

Operator accounts are configured separately and cannot be created by assigning a group. Permissions apply at the server, with ownership and content visibility constraints retained. Course questions and replies require review; course uploads do not.

There is no production deployment to preserve. This permits replacing obsolete identity semantics without a legacy-role compatibility layer, but does not authorize deletion of files or existing development data.

## Delivery rules

Each feature has its own issue scope, tests, documentation and review boundary. A passing build does not imply that live database, provider or browser behavior has been verified. Verification notes must distinguish those levels. No public issue, PR, deployment or third-party message is created implicitly from a local test result.

Only PostgreSQL is supported by the current runtime. References to a second migration dialect in older contributor templates are obsolete.
