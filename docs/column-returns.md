# Returning administrator-deleted articles to their authors

When a content administrator deletes another author's article, the article stays
deleted and its latest submitted content becomes privately retrievable by that
author. Retrieval is available even after publication permission is revoked.

## Behavior

- Deleting another author's article requires a reason of 1–255 characters. Record
  the administrator, deletion time, and reason, and notify the author once.
- Delete the original article from public reading, profiles, search, and collection
  membership. Cancel incomplete article/comment review tasks. Old links and
  discussion do not become public again through retrieval or recreation.
- In **专栏 → 我的内容 → 被管理员删除**, the author can retrieve the latest
  submission, including an update that was pending or rejected when deletion
  occurred. Return only that submission, rather than the older public version.
- The author can preview the returned Markdown, copy it, download a `.md` file,
  and download its attachments. Other users, including administrators, cannot use
  the private retrieval endpoints for someone else's content.
- With current `column.publish` permission, the author can create a new article
  from the returned content. It has a new identity and does not inherit the old
  article's comments, collection memberships, invitation marker, or lock state.
- First publication of a recreated article always requires review, including for
  users with `content.publish_unreviewed`. Withdrawal and resubmission retain this
  requirement. After first approval, normal publication rules apply to updates.
- Newly edited, unsubmitted content remains a browser-local draft. The return
  record preserves an immutable submission; it is not a server-side draft editor.
- Authors deleting their own articles, including administrators deleting their
  own articles, retain the existing deletion behavior. Collection deletion is
  unchanged. Historical deletions without administrator metadata are not inferred
  to be returnable.

## API and storage

- `GET /v1/columns?kind=article&mine=true&returned=true`: the authenticated author's
  returned articles, paginated. Optional keywords match the latest submission.
- `GET /v1/column/returned?id=...`: the original author's latest returned submission.
- `GET /v1/column/returned/attachment?id=...&index=...`: owner-authorized attachment
  download. Ordinary public/submission attachment routes continue to deny access
  to deleted articles.
- `POST /v1/column/action`: pass `action: "delete"` and `reason` for an
  administrator deleting another author's article.
- `POST /v1/column/submit`: `recreated_from_id` starts a new article from the
  current author's returned content. The server persists its origin so first
  approval cannot be bypassed by withdrawing and resubmitting through the ordinary
  article editor.

Apply PostgreSQL migration `0035_column_returns` with `make migrate` before starting
the upgraded application. It adds deletion metadata and recreation provenance to
articles, an owner-list index, and a structured return-notification flag. All
content changes, cancellation, notification, and audit writes share a transaction;
the search index and unread caches update after commit.

## Verification

Backend integration coverage is in
[columns_test.go](../internal/dao/jinzhu/columns_test.go). The browser/HTTP acceptance
runner uses a disposable loopback fixture produced by
[test_columns.py](../scripts/test_columns.py):

```sh
node web/scripts/columns-returns.test.mjs /tmp/columns-fixtures.json /tmp/column-return-screenshots
```

The runner covers mandatory reasons, owner-only retrieval, latest-submission
selection, notification deduplication, Markdown/attachment exports, publication
revocation, local-draft recovery, and mandatory first review. Set
`COLUMNS_BROWSER_EXECUTABLE` if using an installed browser and make Playwright
available to Node's module resolver.
