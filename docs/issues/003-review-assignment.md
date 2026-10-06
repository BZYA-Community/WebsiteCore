# Issue 003: automatic review assignment

## Problem

The old queues let every moderator act on any pending item. Status changes,
parent counters and audit logs were committed separately, and stale post saves
could overwrite a later moderation decision.

## Implemented behavior

- Posts, comments, replies, course questions/answers and profile changes create
  durable review tasks in the same transaction as the complete submitted content.
  Course uploads remain outside moderation; course questions and answers always
  require review.
- Active accounts holding `content.review` are chosen randomly. The author is
  excluded. An empty reviewer pool leaves the task pending and unassigned.
- `Audit.DeadlineHours` defaults to 48. `Audit.AssignmentIntervalSeconds` defaults
  to 60. Expiration records one timeout event and selects another eligible
  reviewer. The same expired reviewer is not selected again when no alternative
  exists. Revocation or suspension triggers reassignment without a timeout penalty.
- `audit.view_all` grants access to every queue, history and timeout statistic.
  It does not authorize a decision on somebody else's assignment. Independent
  operators may override an assignment; the override is explicitly recorded.
- Decisions require both `task_id` and `revision`. Replacing a pending nickname
  or avatar, or resubmitting a rejected post, invalidates the previous revision.
- Status, counters, assignment history and audit log commit together. Deleting
  content cancels its pending tasks and adjusts counts using current database
  state. Reactions update only their own counts; ordinary post flag updates
  cannot write audit state or visibility.

## API

Existing `/v1/admin/audit/posts`, `/comments`, `/nicknames` and `/avatars`
responses include `review_task`. The corresponding decision endpoints retain
their target/action fields and additionally require `task_id` and `revision`.
Only pending tasks are actionable. A stale revision must be refreshed.

`GET /v1/admin/audit/history?task_id=...` returns assignment and decision events.
`GET /v1/admin/audit/statistics` returns `items` with `user_id` and `timeout_count`.
Reviewers see their own assignments/statistics; `audit.view_all` expands that
scope. Comment queue items include full `contents` for reviewing media.

## Migration and operation

Apply PostgreSQL migration `0030_review_assignment` after the earlier phases.
It enrolls existing pending content once. New requests use atomic submission
methods, so no background scan can assign partially written content. The worker
runs independently of optional analytics jobs and `Audit.Enabled`; already
pending tasks and mandatory course Q&A continue to be serviced.

Keep at least two eligible reviewers available to allow deadline reassignment.
Monitor unassigned tasks and timeout statistics. Search refreshes and user
notifications happen after the database commit; an external search/notification
failure does not reverse a completed decision.

The implementation serializes policy-sensitive writes with the existing policy
row lock. This provides consistent permission revocation and multi-instance
assignment. Load-test the expected submission volume before public launch;
queue throughput beyond that measured capacity may require finer-grained locks.
Export review history before rolling back the migration, which drops only the
new task/history tables and preserves the original content and audit logs.

## Verification

`TestReviewAssignmentsAndAtomicDecisions` uses isolated, randomly prefixed
PostgreSQL tables when `TEST_POSTGRES_DSN` is set. It covers all content kinds,
assignment scope, deadlines, revocation, insufficient reviewers, operator
overrides, revision races, simultaneous decisions, counter consistency, stale
post writes, parent visibility changes and rollback when an audit or assignment
log fails. Submission transactions recheck the current parent after locking the
policy; an earlier readable snapshot cannot authorize a new reply to content
that has become private or hidden.

The isolated localhost HTTP run passed 83 checks against PostgreSQL and Redis,
including generated routes, assigned reviewer queues, view-only administrators,
target/task binding, stale decisions, complete comment content, profile revisions,
automatic deadline transfer, timeout statistics, operator override history and
immediate permission revocation using the same token. A separate configuration
run passed 17 checks with courses disabled, confirming that course and upload
routes disappear while community endpoints remain available.

The reusable HTTP runner is `scripts/test_review_assignments.py`. It requires
a disposable loopback server with review enabled, the default 48-hour deadline,
and a PostgreSQL container named `websitecore-rbac-check-*` containing database
and role `rbac_test`. Apply migrations first. The runner creates synthetic users
and groups, marks only its author fixture's contact verified, and expires only
its own task to test the real worker. It leaves those fixtures for inspection.
Do not use a production database or production credentials.

Provide a private JSON context with `OperatorUsername` and `OperatorPassword`
for the generated operator and a separate JSON file with `Container`. Existing
synthetic reviewers, if any, must use that same test password and be listed in
`--existing-reviewers`, so randomly assigned decisions can be exercised by the
actual reviewer. Tokens remain in memory and result files contain no credentials.

```sh
python scripts/test_review_assignments.py \
  --base http://127.0.0.1:18008 \
  --context /path/to/private-test-context.json \
  --database-context /path/to/private-database-context.json \
  --output /path/to/review-results.json
```

The reusable runner passed 80 checks on the final local build. The exact count
varies with random assignments and the number of worker polling requests.

These tests do not establish production throughput or external provider
availability. Run deployment-specific load and delivery checks before launch.
