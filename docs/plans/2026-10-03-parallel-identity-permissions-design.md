# Parallel identity permissions after human review (#94 / PR #102)

## Accepted requirements

The user supplied a human review and a permission diagram on 2026-10-03. The
diagram explicitly permits Teacher plus Mentor and resolves the contradictory
last sentence of the review text. These rules supersede conflicting rules in
the original issue. All changes are prepared locally until the user authorizes
updating GitHub.

| Group | Added capabilities above Student |
| --- | --- |
| Operator | All capabilities, system information, Admin creation; only one account |
| Admin | Member and content-management capabilities, without system information or Admin-account management; created by Operator |
| Auditor | Review other users' content, without publication bypass |
| Teacher | Create/upload own course, receive and reply to Student messages |
| Mentor | Independently receive and initiate messages, without course access |
| Student | Community posts and replies; contact Teachers/Mentors, never initiate Student-to-Student contact |

Only Operator/Admin bypass public UGC review. Course entities and uploaded
videos do not require review. Existing course comments remain UGC and retain
comment moderation. Private posts remain visible only to their author; making
them public requires review for every non-management identity.

## Model and implementation

Keep immutable `account_type` for dedicated staff accounts. For members,
`member_identity=teacher` represents Teacher access and `is_mentor` independently
represents Mentor access. `member_identity=student,is_mentor=true` therefore
means Mentor-only. Existing Auditor access can coexist with either capability.
Staff accounts inherit capabilities through predicates and need no redundant
member flags. The UI displays all six groups and independent member switches.
Public projections continue hiding Auditor access, as required by the issue.

An alternative of expanding `member_identity` into mutually exclusive role
names would make overlapping Teacher/Mentor access awkward and require a new
wire format. Independent capabilities give the reviewed behavior with the
existing API. No production dependency is added.

Recheck current identities inside existing account/course/chat transactions.
Only course ownership blocks removal of Teacher access or member deletion.
Staff may create courses, always owned by the creator, and receive transfers.
Removing Mentor cancels an invalid proactive pending request to a Student;
removing Teacher while retaining Mentor does not cancel valid contact.
Established conversations retain history and are checked against current
permissions. Teachers can reply to Student requests, but cannot initiate a new
Student contact without Mentor access. All senders still require a bound phone;
member pending limits and block semantics remain unchanged.

System information is guarded by an Operator capability in the API and UI.
Admin creation and lifecycle management are Operator-only. The user explicitly
confirmed that Admins cannot disable, restore or revoke other Admin accounts.
Both management groups still manage member accounts; neither can act on itself
or the Operator. Creation and lifecycle management use separate capabilities.
The PostgreSQL migration allows independent Mentor access and adds a unique
partial index for the Operator account. Empty-table preflight, immutable account
types, forced first Admin password change and self-review denial remain intact.

## Verification

Cover all eight relevant identity combinations in the first-contact matrix,
independent/overlapping capability changes, cancellation, course ownership,
staff course creation, system information denial and publication policy. Run
real PostgreSQL up/down/up and constraint tests, concurrent DAO tests, API
regressions, repository backend/frontend BVT and required browser layout checks.
The previously uploaded video demonstrates earlier rules and cannot validate
this revision.
