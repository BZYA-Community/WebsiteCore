# Community framework development baseline

This document records the accepted direction for the modular WebsiteCore refactor. New repository artifacts use English; the product keeps Chinese and English translations.

The five phases are implemented and committed for an integrated pull request, with per-feature scopes below. See [verification results and remaining deployment gates](community-verification.md) and the [deployment and browser BVT record](deploy/2026-10-07-bvt.md).

| Phase | Deliverable | Tracking |
| --- | --- | --- |
| 1 | Dynamic identity groups, explicit permissions, independent operator, immediate revocation, audited management | [Local issue 001](issues/001-identity-permissions.md) |
| 2 | Optional course module, nested categories, courses and lessons, multiple attachments, moderated Q&A | [Local issue 002](issues/002-course-lessons.md) |
| 3 | Random review assignment, configurable 48-hour deadline, atomic reassignment and missed-review accounting | [Local issue 003](issues/003-review-assignment.md) |
| 4 | Exclusive email/phone verification mode, default Aliyun email, secure one-time verification | [Local issue 004](issues/004-contact-verification.md) |
| 5 | Upload limits, client video compression, file validation, trusted AliOSS/LocalOSS resource uploads | [Local issue 005](issues/005-media-storage.md) |
| Course UX follow-up | Category directory, direct lesson navigation, dedicated watch page, separate teaching controls | [Local issue 006](issues/006-course-learning-experience.md) |

## Confirmed defaults

Verification of the configured contact method automatically grants Member identity. Unverified accounts use Guest identity. Groups and permissions are managed in the backend UI. Ordinary administrators have no default review exemption. Anonymous visitors can browse public posts and the course catalog; members can view lessons, publish, upload and initiate messages.

Operator accounts are configured separately and cannot be created by assigning a group. Permissions apply at the server, with ownership and content visibility constraints retained. Course questions and replies require review; course uploads do not.

The original development baseline permitted replacing obsolete identity semantics without a legacy-role compatibility layer. The [deployment record](deploy/2026-10-07-bvt.md) now documents a migrated installation; subsequent changes must preserve its data and account for schema compatibility.

## Delivery rules

Each feature has its own issue scope, tests, documentation and review boundary. A passing build does not imply that live database, provider or browser behavior has been verified. Verification notes must distinguish those levels. No public issue, PR, deployment or third-party message is created implicitly from a local test result.

Only PostgreSQL is supported by the current runtime. References to a second migration dialect in older contributor templates are obsolete.
