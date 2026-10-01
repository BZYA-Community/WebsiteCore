# Explicit identity and permissions (#94)

The issue is the accepted product specification. A member has exactly one
Student/Teacher identity; Mentor is a Teacher designation. Operator and Admin
are dedicated accounts. Auditor is an optional member permission, never a
public badge or an exemption from moderation.

## Implementation

- Use nullable `member_identity`, `is_mentor`, canonical `roles`, and an immutable
  internal `account_type` to retain an Admin account's kind after role removal.
  Database checks reject invalid combinations. Registration always creates a
  Student; only the configured bootstrap creates Operators.
- Replace role-presence and legacy boolean checks with explicit capabilities.
  Separate public user projections from self/admin responses.
- Authorize account changes and course ownership changes inside transactions,
  locking affected users in ascending ID order. A Teacher with courses or a
  Mentor designation cannot be demoted or deleted; banning remains available.
- Persist private-message state per unordered user pair. Lock both users, then
  the conversation; check current identities, phone binding and blocks; update
  pending/established state and insert the message in the same transaction.
  Identity changes cancel invalid pending requests. Established history survives.
  A transaction advisory lock serializes identity mutations against shared chat
  operations, so concurrent changes to both participants cannot miss cancellation.
- Restrict temporary-password Admin sessions to account inspection and password
  change. Rotate the token salt when changing the password.
- Generate routing through `go generate mirc/gen.go`; update the first-party UI,
  both locale packs and the requirements baseline together.

## Alternatives considered

Keeping `mentor` as a role or retaining compatibility fields contradicts the
breaking API requirement. Deriving pending state from historical messages loses
cancellation state and permits concurrent first messages; an explicit pair row
is required. No new production dependency is needed.

## Migration and verification

PostgreSQL is the sole supported engine (see `docs/deploy/database.md`); older
dual-dialect instructions are stale. The runner checks for any user, including
soft-deleted users, before initializing the migration driver or writing version
metadata. The SQL migration repeats the empty-table check under an exclusive
table lock. There is no data mapping or deletion path.

Verify domain matrices, public projections, moderation, admin lifecycle, course
ownership, real PostgreSQL constraints and concurrent pending requests. Run the
repository backend/frontend BVT and relevant browser checks; report any
environment limits explicitly.
