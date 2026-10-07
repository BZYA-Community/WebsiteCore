# Identity groups and permission-based authorization

Status: implemented and locally verified. This local issue tracks Phase 1 of the community-framework request; it replaces the conflicting Student/Teacher design in upstream issue #94 for this work.

## Accepted behavior

- Identity groups are editable in the administration UI. A user can have several groups; effective permissions are their union.
- Guest membership is automatic until contact verification; verified accounts automatically receive Member membership. Neither automatic group is manually assignable.
- Guests can browse public posts and the course catalog. Members can view courses, publish posts and comments, upload content, and initiate private messages.
- Admin has community-management and review permissions. Publishing without review is an independent grant and is absent from every default group.
- Operator is a deployment-managed account flag, belongs to no identity group, and has all known permissions while active. Unknown permissions are always denied.
- The built-in `reviewer` (审核员) group grants `content.review` for assigned content, without user, identity or site administration. Migration `0033_reviewer_identity` adds it without overwriting an existing custom group.
- User management accepts an inclusive registration timestamp range (`registered_from` / `registered_to`, Unix seconds), combined with keyword search and pagination. The date picker includes the entire final day in the browser's local timezone.
- `POST /v1/admin/user/identity` accepts either `user_id` or `user_ids` (1–100 IDs), plus `group_ids`. Groups replace explicit memberships; empty groups remove explicit memberships while preserving automatic base identities. A batch is atomic, rechecks current authority, rejects self/operator changes and grants above the actor, and logs each changed user.
- The project is not deployed to production. Existing legacy roles are not automatically promoted into new permissions; no historical data or user documents are deleted.

## Implementation boundary

The permission catalog lives in `internal/authz`. PostgreSQL stores group definitions, grants, memberships and operation logs. Route middleware denies unregistered policies; object ownership and visibility checks remain in the application layer. Authentication reads current account state and grants rather than cached user objects.

Policy and membership changes reauthorize their actor inside the transaction, prevent grants above the actor's permissions, and commit the operation log atomically. Builtin group keys are immutable; automatic groups cannot be assigned; assigned custom groups cannot be deleted. Operator accounts cannot be edited through membership controls. Self-membership changes are rejected.

The web client consumes effective permissions, refreshes session state before protected routes, and treats hidden controls as convenience rather than a security boundary. Permission names have Chinese and English labels.

## Acceptance checks

- [x] Permission union, unknown permission denial, banned operator denial.
- [x] Every protected API definition has an explicit route policy.
- [x] HTTP middleware never executes a denied handler.
- [x] Live PostgreSQL privilege ceilings, revocation, self-edit and operator guards.
- [x] Failed operation-log writes roll back policy/membership mutations.
- [x] Stale profile writes cannot reverse bans or restore deleted users.
- [x] All schema migrations apply; migration 0025 reverses and reapplies without deleting accounts.
- [x] Live HTTP permission-management flows and same-token revocation (70 checks).
- [x] Browser interaction and responsive layout verification (47 checks across seven viewport widths).
- [x] Sixteen simultaneous initial private messages produce exactly one pending message.
- [x] Profile edits cannot overwrite newly changed credentials; visibility caches separate current grants.
- [x] Backend build, vet, configured golangci-lint, frontend build/lint, focused independent review and repository tests.

## Deployment and recovery

Configure `Operator` before applying migration 0025 and start the upgraded server. Legacy `roles` and `is_admin` columns are retained as historical fields but are not sources of authority. Assign new groups from the operator session. Rollback of 0025 drops the new policy and operation-log tables; export them first if they contain information you need. Never run a down migration as an automatic recovery step.

Verification uses isolated PostgreSQL and local synthetic accounts. Actual provider, production load, and external storage validation belong to the later contact-verification and upload phases.

Browser checks cover keyboard focus, accessible input labels, Chinese/English text, identity-group CRUD, operation logs and operator/self-edit protection at widths 1920, 1600, 1366, 1200, 1000, 821 and 375. Frontend lint has zero errors and 181 existing warnings (baseline 193); permission/session-race checks and translation parity checks pass. Local evidence and generated test credentials stay in ignored verification directories.

The sandboxed repository suite uses `go test ./... -skip TestJuheSMS`; the two unchanged SMS HTTP tests were run separately with local-loopback permission and passed. PostgreSQL integration tests require `TEST_POSTGRES_DSN` and use isolated temporary table prefixes. No external SMS or email was sent.
