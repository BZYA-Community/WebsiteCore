# Optional courses, hierarchical categories, and lessons

Status: implemented and locally verified. This local issue tracks Phase 2 of the community-framework request; deployment-provider and load checks remain separate.

## Accepted behavior

- Course categories form a tree through `parent_id`; `0` means a root category. Category deletion is rejected while direct child categories or courses remain. Concurrent moves cannot form a cycle.
- A course has catalog metadata, a teacher, and `teacher_intro`. Course editing does not replace lessons. Filtering a category includes its descendant categories.
- Each lesson has a stable ID, title, introduction, sort order, and multiple attachment references. Lessons are edited independently. Omitting or sending `null` for an update's attachment array preserves references; an explicit `[]` removes them.
- Regular lesson attachments are at most 50 MiB; course resources are at most 2 GiB. Limits use server-recorded sizes, not client assertions.
- New references require an existing, verified upload owned by the actor. `kind=attachment` requires `purpose=course_attachment`; `kind=resource` requires `purpose=course_resource`. Existing references in that same lesson can be retained by its current manager after teacher reassignment.
- A file may be reused in its owner's lessons. Deleting a lesson/course removes references and database content, but never deletes shared upload objects.
- The catalog exposes metadata only. Lessons and signed resource URLs require `course.view`. `course.manage_own` permits changes only to courses currently owned by the actor. Global taxonomy management requires `course.manage`.
- Course publication does not require moderation. Questions and replies always enter moderation, including those from teachers and operators; Phase 3 supplies automatic review assignment.

## API contract

Existing category/course URLs remain. Category requests and responses add `parent_id`; course requests and responses add `teacher_intro`. New courses no longer require a video. Raw `video` values are rejected by the new editor API; use verified lesson attachments.

| Route | Body/query | Response data |
| --- | --- | --- |
| `GET /v1/course/lessons` | `course_id` | `{ "lessons": [...] }` |
| `GET /v1/course/attachment` | `id` = lesson attachment reference ID | `{ "signed_url": "..." }` |
| `POST /v1/admin/course/lesson` | `course_id`, `title`, `intro`, `sort`, `attachments` | lesson object |
| `POST /v1/admin/course/lesson/update` | same fields plus `id`; parent course cannot change | lesson object |
| `POST /v1/admin/course/lesson/delete` | `id` | empty success |

Each input attachment is `{ "attachment_id": 123, "name": "Notes.pdf", "kind": "attachment" }`. Output adds its reference `id`, `file_size`, and `mime_type`; it never includes a storage key or an unsigned URL. Signed links expire after one hour. Upload completion/verification and AliOSS/LocalOSS transport are supplied by Phase 5.

Titles are 1–128 characters, category names 1–64, introductions at most 2,000, attachment display names 1–255, and a lesson accepts at most 100 references. Attachment order follows the input array; lesson order is `sort`, then `id`. Duplicate upload IDs in one lesson are rejected.

## Integrity and recovery

Mutations re-read the current account and grants inside the transaction, lock the current course, and record an operation log in the same transaction. Failure to write the log aborts the mutation. Lesson validation completes before any references are changed. A failed resource query aborts the edit rather than interpreting an unreadable list as empty.

Migration `0026_course_lessons` adds category parents, teacher biographies, lessons, attachment references, verification metadata, and the shared `operation_log` table. It leaves existing development records and upload files intact. Historical `video_url` values remain in storage and can still be read through the authorized legacy `/course/video` endpoint; they are not returned by the catalog and are not silently converted into verified uploads.

Export lessons and operation logs before an explicitly requested down migration: the down migration drops these new records and metadata. It does not remove original courses, users, or physical upload files. Never apply a down migration automatically.

The root configuration/route registry controls the optional course module. Disabling it must remove course routes and the frontend entry; hiding the UI alone is insufficient.

## Verification

- Unit checks cover attachment size/purpose/verification boundaries, resource-free catalog JSON, resource permission checks, and omitted versus explicit-empty attachment arrays.
- `TEST_POSTGRES_DSN=... go test ./internal/dao/jinzhu -run TestCourse -count=1` uses temporary random-prefix tables. It checks category cycles and concurrent reparenting, descendants, ownership, multiple attachments, stable reference IDs, invalid uploads, reassignment/revocation, failed-log rollback, and shared-upload preservation.
- `go test ./internal/servants/web` checks the service/API boundaries after generated routes are refreshed.
- Live browser checks against the isolated PostgreSQL/Redis API created root and child categories, a course without a legacy video URL, and a teacher biography. Two files completed signed LocalOSS uploads before being bound to a lesson; signed downloads and lesson edits preserving both attachment references passed. Direct upload requests did not carry the session token.
- A question submitted through the course UI entered moderation and appeared in the task dialog with its complete content. Approval sent the current task revision. A subsequent operator reply still entered moderation and displayed its pending state, confirming that course answers do not inherit the general publication bypass.
- Catalog, detail, and lesson-editor layouts were checked at 375, 821, 1000, 1200, 1366, 1600, and 1920 pixels. The lesson editor retained keyboard focus through a full tab cycle. A separate browser capability-response fixture checked that disabling courses removes navigation and rejects direct course routes; actual backend module-off checks are documented with Phase 3.
- These browser checks use synthetic local accounts and files. They do not establish AliOSS provider availability or production concurrency.
