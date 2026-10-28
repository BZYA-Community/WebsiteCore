# Independent Markdown articles and collections (专栏)

Status: implemented and locally verified on `feat/columns`. This document records
the accepted first-release scope and its local verification on 2026-10-28.

## Problem

The community needs a dedicated place for editable Markdown articles, author
series, and administrator-curated collections. Existing Markdown posts remain
posts; their publishing and discussion flows do not provide this independent
article lifecycle, collection organization, or review of article updates.

## Entry and content boundaries

- Add a **专栏** entry to the left sidebar. Public browsing, article search,
  publishing, collections, and the user's content management are accessible
  from this area.
- Articles are independent of posts. Existing Markdown posts remain unchanged;
  converting between posts and articles is outside this release.
- Do not add articles to the post feed or add an article tab to user profiles.
- Approved articles are public, including to anonymous readers. There is no
  followers-only, privately published, or paid article visibility.
- An article has a required title and Markdown body. The server automatically
  extracts its summary from the first 200 characters of plain body text on every
  submission; articles have no custom-summary field or cover image.
- Markdown editing, rendering, media, and upload capabilities match the existing
  Markdown post experience rather than introducing a separate feature subset.
- Limit the body to 50,000 characters and validate the limit on both client and
  server. Report an error instead of silently truncating content.

## Authoring and Markdown import

- Support both the online Markdown editor and importing a single `.md` file
  into that editor. Import reads the file as article content; it does not merely
  publish a downloadable attachment.
- Use the imported filename as an initial title, which the author can change.
  Imported content can be previewed, edited, and submitted like online writing.
- No ZIP import, bulk import, or bundled local images. Relative image references
  such as `./images/a.png` require the author to upload and replace the reference.
- Unsubmitted edits are saved only in the current browser, separated by account
  and article or collection. There is no server-side draft-save feature.
- Another device cannot recover changes that have never been submitted.
- A submission to review is persisted on the server with its complete content.
  Pending and rejected submissions remain retrievable by the author for further
  editing; this is distinct from storing unsubmitted drafts.

## Publication, updates, and review

- Maintain the latest public version separately from the current working
  submission. First publication is not public until approval, unless the author
  holds the existing permission to publish without review.
- An author explicitly submits changes for review. A submitted version is
  immutable while pending: withdraw it before editing and resubmitting.
- A pending or rejected update does not remove or alter the currently approved
  public version. Approval replaces that version with the submitted version.
- Review title, generated summary, and body together. Changing the title or body
  requires a new submission under the same review rules.
- Apply the same public-version and submission behavior to reviewed collection
  metadata: title, introduction, and cover image.
- Reuse the existing review backend, assignment rules, result notifications,
  and audit logs. Reviewers can inspect the submitted version and the current
  public version.
- Reviewers, including administrators, cannot approve their own submissions.
  The existing `content.publish_unreviewed` permission applies to articles and
  collection metadata; an administrator without that permission still needs
  another reviewer.
- Do not provide a version-history browser, historical diff viewer, or rollback
  action. Retain review records for administrative traceability.

## Collections (合集)

Collections organize references to articles; they are not copies of article
content or the existing post-bookmark feature.

- A collection has a title, introduction, and optional cover image. These public
  fields require review, with the exemption described above.
- Ordinary publishers create collections containing only their own articles.
- Administrator-created collections may contain any author's public articles.
  Cross-author collection capability is restricted to administrators.
- An article can belong to any number of collections, or none. Each collection
  has its own manually editable article order.
- Adding or removing an already public article and changing its position take
  effect immediately; these operations do not trigger metadata or article review.
- Readers see only public articles in collection directories. Do not expose
  draft titles, pending submissions, or placeholders for unpublished articles.
- A collection with no publicly visible articles is absent from the public
  collection list.
- Administrator inclusion does not need author approval. Notify the author;
  the author cannot independently remove the article from that collection.
- A collection displays the original author and links to the original article.
  Article updates appear through the same article reference.
- Administrators may manage any collection. Deleting a collection removes its
  organization and membership relations, not the articles themselves.
- An administrator-created collection remains when its creator loses management
  permission; other administrators can take over its management.

## Comments and replies

Match the product behavior of post comments, including review, permissions, and
notifications. Existing implementation inconsistencies are not requirements to
copy: for example, do not display a moderator's featured-comment action only to
reject that same authorized action on the server.

- Use top-level comments and flat replies within each comment. A reply can
  address another user but does not create a deeper nested reply tree.
- Top-level comments support text, user mentions, and images; replies support
  plain text and an addressed user. Retain the post-comment limits and controls
  (currently 300-character inputs and at most nine images per top-level comment).
- Retain recommended, hot, and newest comment sorting, featured comments first,
  and the current paging behavior: 20 top-level comments per page, with their
  visible replies returned in chronological order.
- Support likes and dislikes on comments and replies, including toggling and
  mutual exclusion. Show like counts without exposing dislike totals.
- Require current comment permissions and access to the parent article. Support
  locking article comments: existing discussion stays readable, but new comments
  and replies are blocked by both UI and API.
- Apply the post-comment review and visibility rules. Authors can see their own
  pending or rejected content; other readers see approved content; authorized
  review administrators can inspect all review states. Rejected parent comments
  cannot receive replies; pending parents accept replies only from their author
  or authorized review administrators.
- Comment authors can delete their own content; content managers can delete
  others' content. Article ownership alone does not grant deletion of other
  people's comments. Deleting a top-level comment also removes its replies.
- Article authors can feature comments. Management controls must agree with
  server authorization. Do not add comment or reply editing.
- Notify the article author, comment author, addressed users, and mentioned
  users as appropriate, excluding the sender and duplicate recipients. Send
  interaction notifications when content becomes public, and send review-result
  notifications to the content author.
- Use the existing configurable comment-count limit and count approved comments
  and replies. Pending content must not inflate public counts.
- Discussion belongs to the article identity and survives public-version
  replacement. Preserve comment timestamps and show the article's update time;
  do not create separate discussions per version or attach comments to historical
  paragraphs.
- Article likes, article bookmarks, and collection subscriptions are not part of
  this release. Article links can be copied for sharing.

## Official invitation (官方特邀)

- An administrator may add or revoke this recommendation marker on an article.
  It is not an author identity or a publishing permission.
- The marker survives publication of updated article versions automatically.
- Display the marker and support filtering articles by it. It does not imply
  review exemption or automatic pinning in the article list.

## Permissions and moderation

- Introduce an independently configurable article publishing permission in the
  existing identity-group permission system. Effective permission is the final
  authority, rather than an additional hard-coded contact-verification gate.
- Initially grant publishing to verified members (道友) and administrators.
  Administrators can change group grants, including allowing other groups.
  Existing automatic member identity follows the site's active email or phone
  verification mode.
- Recheck current publishing permission when saving/submitting author changes
  and completing publication. After revocation, authors may still view their
  own saved/submitted content and delete their own articles, but cannot create,
  save new edits, or submit updates. Do not automatically unpublish old articles.
- Cancel incomplete publication reviews after permission revocation and return
  them to an editable local-draft flow once permission is restored. Preserve
  already submitted content for retrieval; do not introduce server draft saves.
- Administrators can review, take down, delete, lock comments, and manage the
  official-invitation marker. They cannot edit an author's article title, summary,
  or body on the author's behalf. Collection management remains as specified above.
- Authors can delete their own articles. Administrators can take down or delete
  any article. Removal immediately blocks public reading, comments, and public
  collection/search visibility, and cancels incomplete review tasks.
- Preserve removed-content and discussion records for management traceability.
  An author cannot republish an administrator-taken-down article until an
  administrator lifts the restriction.

## Listing and search

- Order articles by first publication time, newest first. Updating an article
  does not move it back to the beginning. Collection contents use their manual
  order instead.
- Put article search in the 专栏 area, not the existing global/post search page.
- Use a dedicated Meilisearch index, separate from the post index. Search the
  title, summary, and body of the latest public version only.
- Never expose local edits, pending updates, rejected versions, or removed
  articles through public search. An update awaiting review leaves the approved
  version searchable until replacement.
- Fall back to database search when Meilisearch is disabled or unavailable.
  The same publication and removal rules apply to both search paths.
- Collections do not participate in full-text search in this release. Do not
  introduce article tags. Support the official-invitation filter.

## Implementation notes

- Reuse the existing Markdown editor/rendering, media, identity permissions,
  review assignment, notifications, and comment behavior where appropriate.
- Existing review-task revisions are not article-content version storage.
  Persist complete submissions and preserve the approved public version so that
  stale decisions cannot publish a withdrawn, superseded, or unauthorized version.
- Public detail, collection membership reads, comment APIs, and both search
  implementations must enforce the same current publication and takedown state.
- Follow the repository's API-generation and migration workflow. Keep Chinese
  and English UI strings in the existing localization system and preserve the
  site's responsive layouts and light/dark themes.
- Relevant existing local issues: [identity permissions](001-identity-permissions.md),
  [review assignment](003-review-assignment.md), and
  [media storage](005-media-storage.md).

## Acceptance checks

Implemented acceptance checks; repeatable verification and deployment notes follow.

- [x] Reach article browsing, search, collections, authoring, and management from
  专栏 without adding articles to the post feed or user-profile tabs.
- [x] Import a `.md` file, edit and preview it with the existing Markdown
  capabilities, automatically extract the body summary, and reject an over-limit body on
  both client and server without truncation.
- [x] Recover separate browser-local edits for multiple articles/accounts;
  unsubmitted edits are absent from server storage and another device.
- [x] Submit and retrieve complete review content without a server draft-save
  endpoint; withdraw before modifying a pending submission.
- [x] Keep the approved version readable and searchable during a pending or
  rejected update; replace it only on valid approval or authorized exempt publication.
- [x] Prevent self-review and stale, withdrawn, superseded, or revoked-permission
  publication; retain review results and administrative logs.
- [x] Verify default member/admin publishing grants, configurable group grants,
  exemption behavior, and immediate revocation without hiding existing articles.
- [x] Create author-only collections and administrator collections containing
  other authors' articles; notify those authors without requesting approval or
  allowing them to remove the membership independently.
- [x] Put one article in multiple collections with independent ordering; change
  membership/order immediately while collection metadata updates await review.
- [x] Hide unpublished entries and publicly empty collections; delete a collection
  without deleting its articles; retain administrator collections after creator
  management-permission loss.
- [x] Exercise comment images/mentions, addressed replies, paging/sorting,
  likes/dislikes, featuring, ownership/management deletion, limits, and review.
- [x] Keep one discussion across article updates, block writes when comments are
  locked, and send deduplicated interaction and review notifications.
- [x] Add/revoke 官方特邀, filter by it, and preserve it across article updates
  without granting exemption or changing the default list order.
- [x] Confirm updates do not change first-publication ordering.
- [x] Search only current public article content in an independent Meilisearch
  index; repeat with database fallback and verify pending/rejected content is absent.
- [x] Delete or take down an article and immediately block direct public access,
  public discussion, collection visibility, and search exposure; prevent author
  republication of an administrator-taken-down article.
- [x] Verify administrators cannot edit another author's article body or metadata;
  do not expose version-history, rollback, article-like/bookmark, or subscription UI.
- [x] Check both languages, light/dark themes, responsive layouts, permission-aware
  controls, and corresponding server enforcement.


## Deployment and implementation references

Apply PostgreSQL migration `0034_columns` before starting the upgraded application
(`make migrate`). It creates independent article/submission, collection-membership,
comment, and reaction tables, extends review-task kinds, and adds article targets
to system notifications and audit logs. The migration grants `column.publish` to
existing member/admin groups without replacing their other grants. Operators retain
the existing independent full-permission behavior.

The dedicated Meilisearch index is `<Meili.Index>_columns`; no new bootstrap key is
required. It is initialized from current public articles. Search checks each hit's
published submission against PostgreSQL before returning results, falls back to
PostgreSQL on unavailable or stale search data, and repairs the index. Pending
submissions never overwrite approved content or its indexed document.

- API declarations: [mirc/web/v1/columns.go](../../mirc/web/v1/columns.go).
- Capability contract: [internal/core/columns.go](../../internal/core/columns.go).
- Persistence and publication: [internal/dao/jinzhu/columns.go](../../internal/dao/jinzhu/columns.go).
- Page entry: [web/src/views/Columns.vue](../../web/src/views/Columns.vue).
- Review UI: [web/src/components/columns/column-review-queue.vue](../../web/src/components/columns/column-review-queue.vue).

Unsubmitted drafts are stored under account/content-specific browser keys. A
conflicting server submission does not silently replace a local draft: the author
can explicitly restore or discard local changes. Review-result notifications open
the author's submission; discussion and curation notifications open public content.
New media references require the actor's verified community uploads; administrators
editing collections may retain existing cover references. Private/course attachment
URLs cannot be republished through article download signing.

## Verification and repeatable checks

The implementation was checked against a disposable loopback application with
isolated PostgreSQL, Redis, and Meilisearch. Existing development configuration and
application data were not changed.

- Backend package tests, including PostgreSQL integration tests, passed. New
  integration coverage checks public-version preservation, stale decisions,
  revocation, collection ownership/order, former administrator restrictions,
  media ownership, comment moderation/reactions/deletion, and maximum-length
  rejection reasons.
- The complete migration chain passed on an empty database. Migration 0034 also
  rolled back and reapplied successfully on a separate disposable database.
- `golangci-lint` v2.14.0 reported zero issues; `go vet` passed. API adapters were
  regenerated from `mirc` with the module's Go 1.26.6 toolchain.
- Frontend lint reported zero errors (116 existing warnings), locale parity and
  missing-key checks passed, and the production build passed. Existing permission,
  media, course, sidebar, admin-user, and compose regression suites passed.
- The local HTTP runner passed 134 checks covering automatic summary extraction,
  summary regeneration on edits, actual uploads and signed
  downloads, picture comments and mentions, publication/review, collection ordering,
  notifications/unread-cache refresh, revocation, and access control.
- The browser runner passed 32 checks covering import and length rejection,
  multiple local drafts, account isolation, absence of server drafts, failed-save
  retention, withdraw/resubmit, review approval, comments/replies/locking, and
  collection ordering. Layouts were checked at 375, 390, 821, 822, 830, 1000,
  1200, 1366, 1600, and 1920 pixels, including Chinese/light and English/dark views, with no browser
  runtime errors or horizontal overflow.
- A deliberately stale Meilisearch document was rejected by public search and
  repaired from the published version. Database search remained usable while
  the dedicated Meilisearch container was stopped.

The HTTP runner requires Python `requests`, a disposable loopback server with
email verification's development bypass enabled, and an operator password supplied
through `COLUMNS_TEST_PASSWORD`. It creates synthetic accounts/content and can write
private fixture context for the browser runner. Do not commit that context.

```sh
python scripts/test_columns.py --base http://127.0.0.1:18089 --operator column_operator --context /tmp/columns-fixtures.json
```

The browser runner requires Playwright (available to Node's module resolver) and
an installed browser. `COLUMNS_BROWSER_EXECUTABLE` can select an existing Chrome;
otherwise Playwright uses its bundled Chromium. Screenshots default to
`/tmp/websitecore-columns-ui` or the optional third argument.

```sh
node web/scripts/columns-flow.test.mjs /tmp/columns-fixtures.json /tmp/columns-shots
```

For PostgreSQL unit/integration checks, set `TEST_POSTGRES_DSN` to a disposable
local database; test tables use random prefixes and are removed after the run.

```sh
go test ./internal/dao/jinzhu -run TestColumn -count=1
```
