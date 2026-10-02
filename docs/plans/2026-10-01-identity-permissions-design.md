# Explicit identity and permissions (#94)

The issue defines the initial specification. The human review confirmed on
2026-10-03 supersedes its conflicting permission rules; see
[the revised design](2026-10-03-parallel-identity-permissions-design.md).
Operator, Admin, Auditor, Teacher, Mentor and Student are parallel product
groups. Teacher and Mentor capabilities are independent and may overlap.
Operator and Admin retain dedicated accounts and inherit all member capabilities.
Only Operator can view system information and create Admins; only management
accounts bypass publication review. Auditor remains private in public projections.

## Implementation

- Use nullable `member_identity`, `is_mentor`, canonical `roles`, and an immutable
  internal `account_type` to retain an Admin account's kind after role removal.
  `member_identity` expresses course access, while `is_mentor` independently
  expresses proactive messaging. Neither flag implies the other.
  Database checks reject invalid combinations. Registration always creates a
  Student; only the configured bootstrap creates the single Operator, enforced
  by a unique partial database index.
- Replace role-presence and legacy boolean checks with explicit capabilities.
  Separate public user projections from self/admin responses.
- Authorize account changes and course ownership changes inside transactions,
  locking affected users in ascending ID order. A course owner cannot lose
  Teacher access or be deleted until courses are transferred; Mentor access
  does not block either operation. Banning remains available.
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

Retaining compatibility fields contradicts the breaking API requirement.
Making Mentor an independent capability preserves existing wire fields while
supporting the reviewed product groups. Deriving pending state from historical messages loses
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
