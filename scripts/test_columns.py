#!/usr/bin/env python3
"""Exercise columns against a disposable loopback instance with dev verification.

Requires requests. Supply the operator password through COLUMNS_TEST_PASSWORD.
Creates synthetic accounts/content; never point this at an existing deployment.
An optional --context writes synthetic fixture credentials for browser checks.
"""

import argparse
import json
import os
import secrets
import struct
import zlib
import time
from pathlib import Path
from urllib.parse import urlparse

import requests


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base", default="http://127.0.0.1:18089")
    parser.add_argument("--operator", default="column_operator")
    parser.add_argument("--context", type=Path)
    args = parser.parse_args()
    if urlparse(args.base).hostname not in {"localhost", "127.0.0.1", "::1"}:
        parser.error("a disposable loopback server is required")
    password = os.environ["COLUMNS_TEST_PASSWORD"]
    checks = 0

    def check(condition, message):
        nonlocal checks
        assert condition, message
        checks += 1

    def call(method, path, token="", data=None, params=None, success=True, **kwargs):
        headers = {"Authorization": f"Bearer {token}"} if token else {}
        response = requests.request(method, args.base + path, headers=headers,
                                    json=data, params=params, timeout=30, **kwargs)
        body = response.json()
        check((body.get("code") == 0) == success,
              f"{method} {path}: HTTP {response.status_code}, code={body.get('code')}, message={body.get('msg')}")
        return body.get("data", {})

    def get(path, token="", **params):
        return call("GET", path, token, params=params)

    def post(path, token="", **data):
        return call("POST", path, token, data=data)

    def login(name):
        return post("/v1/auth/login", username=name, password=password)["token"]

    operator = login(args.operator)
    suffix = secrets.token_hex(3)
    names = {role: f"col{role}{suffix}" for role in ("a", "b", "r", "u")}
    ids, tokens = {}, {}
    for role, name in names.items():
        ids[role] = post("/v1/auth/register", username=name, password=password)["id"]
        tokens[role] = login(name)
        if role != "u":
            address = f"{name}@example.test"
            post("/v1/user/contact/code", tokens[role], mode="email", address=address)
            post("/v1/user/contact/verify", tokens[role], mode="email", address=address, code="123456")
    a, b, reviewer, unverified = (tokens[role] for role in ("a", "b", "r", "u"))
    check("column.publish" in get("/v1/user/info", a)["permissions"], "verified member lacks default publish grant")
    call("POST", "/v1/column/submit", unverified,
         data={"kind": "article", "title": "Denied", "body": "Unverified"}, success=False)

    reviewer_group = post("/v1/admin/identity/groups", operator, key=f"col_review_{suffix}",
                          name="Column test reviewer", permissions=["content.review", "audit.view_all"])["groups"][0]
    post("/v1/admin/user/identity", operator, user_id=ids["r"], group_ids=[reviewer_group["id"]])
    publish_group = post("/v1/admin/identity/groups", operator, key=f"col_publish_{suffix}",
                         name="Column test publish exception", permissions=["column.publish"])["groups"][0]
    post("/v1/admin/user/identity", operator, user_id=ids["u"], group_ids=[publish_group["id"]])
    check("column.publish" in get("/v1/user/info", unverified)["permissions"], "explicit grant blocked by verification")

    def submit(token, kind, title, body="", item=None, **extra):
        return post("/v1/column/submit", token, id=item["id"] if item else 0,
                    expected_submission=item["submission_id"] if item else 0,
                    kind=kind, title=title, body=body, **extra)["item"]

    def review(item, action="approve"):
        queue = get("/v1/admin/audit/columns", operator, page=1, page_size=100)["items"]
        entry = next(row for row in queue if row.get("submission", {}).get("id") == item["submission_id"])
        actor = reviewer if entry["task"]["assignee_id"] == ids["r"] else operator
        task = entry["task"]
        post("/v1/admin/audit/column", actor, task_id=task["id"], kind=task["kind"],
             target_id=task["target_id"], revision=task["revision"], action=action,
             reason="Needs changes" if action == "reject" else "")
        return task

    article = submit(a, "article", f"Column original {suffix}", f"originalonlyword{suffix}",
                     summary="Client-provided summary must be replaced")
    call("GET", "/v1/column", params={"id": article["id"]}, success=False)
    check(get("/v1/columns", keyword=f"originalonlyword{suffix}")["total"] == 0, "unreviewed article searchable")
    review(article)
    public = get("/v1/column", id=article["id"])["item"]
    check(public["title"] == article["submission"]["title"], "approved title not published")
    check(public["summary"] == f"originalonlyword{suffix}", "article summary was not extracted from its body")
    first_publication = public["published_on"]
    post("/v1/column/action", operator, id=article["id"], action="invite")
    check(get("/v1/columns", keyword=f"originalonlyword{suffix}", official="true")["total"] == 1, "public search or invitation filter failed")
    pending = submit(a, "article", "Pending title", f"pendingonlyword{suffix}", item=article)
    check(get("/v1/column", id=article["id"])["item"]["body"] == f"originalonlyword{suffix}", "pending update leaked")
    check(get("/v1/columns", keyword=f"pendingonlyword{suffix}")["total"] == 0, "pending update searchable")
    call("POST", "/v1/column/submit", a, data={"id": pending["id"], "kind": "article",
         "expected_submission": pending["submission_id"], "title": "Mutable", "body": "changed"}, success=False)
    stale_task = next(row["task"] for row in get("/v1/admin/audit/columns", operator, page=1, page_size=100)["items"]
                      if row.get("submission", {}).get("id") == pending["submission_id"])
    post("/v1/column/action", a, id=article["id"], action="withdraw")
    call("POST", "/v1/admin/audit/column", operator, data={"task_id": stale_task["id"], "kind": stale_task["kind"],
         "target_id": stale_task["target_id"], "revision": stale_task["revision"], "action": "approve"}, success=False)
    update = submit(a, "article", f"Column updated {suffix}", f"currentonlyword{suffix}", item=pending,
                    summary=public["summary"])
    review(update)
    public = get("/v1/column", id=article["id"])["item"]
    check(public["summary"] == f"currentonlyword{suffix}", "article update retained an outdated summary")
    check(public["body"] == f"currentonlyword{suffix}" and public["official"] and public["published_on"] == first_publication,
          "update lost public version identity or marker")
    check(get("/v1/columns", keyword=f"originalonlyword{suffix}")["total"] == 0, "replaced version remained searchable")
    rejected = submit(a, "article", "Rejected title", f"rejectedonlyword{suffix}", item=update)
    review(rejected, "reject")
    check(get("/v1/column", id=article["id"])["item"]["body"] == f"currentonlyword{suffix}", "rejection changed publication")
    call("GET", "/v1/column/submission", b, params={"id": article["id"]}, success=False)
    call("POST", "/v1/column/submit", operator, data={"id": article["id"], "expected_submission": rejected["submission_id"],
         "kind": "article", "title": "Hijack", "body": "Admin rewrite"}, success=False)
    call("POST", "/v1/column/submit", a, data={"kind": "article", "title": "Too long", "body": "中" * 50001}, success=False)

    upload = requests.post(args.base + "/v1/attachment", headers={"Authorization": f"Bearer {a}"},
                           data={"type": "attachment"}, files={"file": ("column-notes.md", b"# Column notes\nverified attachment", "text/plain")}, timeout=30).json()
    check(upload.get("code") == 0, f"real attachment upload failed: {upload.get('msg')}")
    with_media = submit(a, "article", f"Column media {suffix}", f"attachmentword{suffix}", item=rejected,
                        attachments=[{"content": upload["data"]["content"], "type": 7, "sort": 1}])
    review(with_media)
    signed = get("/v1/column/attachment", id=article["id"], index=0)["signed_url"]
    download = requests.get(signed, timeout=15)
    check(download.status_code == 200 and b"verified attachment" in download.content, "signed attachment download failed")

    second = submit(b, "article", f"Second article {suffix}", f"secondbodyword{suffix}")
    review(second)
    collection = submit(a, "collection", f"Author collection {suffix}")
    review(collection)
    curated = submit(operator, "collection", f"Curated collection {suffix}")
    check(curated["submission"]["audit_status"] == 1, "operator exemption missing")
    call("POST", "/v1/column/entries", a, data={"id": collection["id"], "articles": [second["id"]]}, success=False)
    post("/v1/column/entries", a, id=collection["id"], articles=[article["id"]])
    for _ in range(20):
        unread_before = get("/v1/user/msgcount/unread", a)["count"]
        if unread_before > 0:
            break
        time.sleep(0.05)
    else:
        raise AssertionError("unread-count cache did not finish priming")
    post("/v1/column/entries", operator, id=curated["id"], articles=[second["id"], article["id"]])
    # Existing unread-count responses are populated asynchronously on a cache miss.
    for _ in range(20):
        if get("/v1/user/msgcount/unread", a)["count"] > unread_before:
            break
        time.sleep(0.05)
    else:
        raise AssertionError("curation notification unread cache was not refreshed")
    checks += 1
    check([row["id"] for row in get("/v1/column/entries", id=curated["id"])["items"]] == [second["id"], article["id"]], "manual order lost")
    call("POST", "/v1/column/entries", a, data={"id": curated["id"], "articles": []}, success=False)
    notifications = get("/v1/user/chat/history", a, user_id=0, page=1, page_size=100)["messages"]
    check(any(row.get("column_id") == curated["id"] for row in notifications), "curation notification lacks article-area link")

    def png_chunk(kind, data):
        return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data) & 0xffffffff)
    png = b"\x89PNG\r\n\x1a\n" + png_chunk(b"IHDR", struct.pack(">IIBBBBB", 1, 1, 8, 6, 0, 0, 0)) + png_chunk(b"IDAT", zlib.compress(b"\x00\xff\x00\x00\xff")) + png_chunk(b"IEND", b"")
    image_upload = requests.post(args.base + "/v1/attachment", headers={"Authorization": f"Bearer {a}"},
                                 data={"type": "public/image"}, files={"file": ("column-comment.png", png, "image/png")}, timeout=30).json()
    check(image_upload.get("code") == 0, "comment image upload failed")
    image_url = image_upload["data"]["content"]
    image = {"type": 3, "content": image_url, "sort": 2}
    comment_contents = [{"type": 2, "content": f"Reviewed discussion @{names['b']}", "sort": 1}, image]
    call("POST", "/v1/column/comment", b, data={"column_id": article["id"], "contents": comment_contents}, success=False)
    call("POST", "/v1/column/comment", a, data={"column_id": article["id"], "contents": [comment_contents[0]] + [image] * 10}, success=False)
    comment = post("/v1/column/comment", a, column_id=article["id"], users=[names["b"]], contents=comment_contents)["item"]
    check(get("/v1/column/comments", id=article["id"])["total"] == 0, "pending comment public")
    check(get("/v1/column/comments", a, id=article["id"])["total"] == 1, "own pending comment invisible")
    call("POST", "/v1/column/comment", b, data={"column_id": article["id"], "parent_id": comment["id"], "content": "Hidden parent"}, success=False)
    entry = next(row for row in get("/v1/admin/audit/columns", operator, page=1, page_size=100)["items"] if row.get("comment", {}).get("id") == comment["id"])
    task = entry["task"]
    actor = reviewer if task["assignee_id"] == ids["r"] else operator
    post("/v1/admin/audit/column", actor, task_id=task["id"], target_id=task["target_id"], kind=task["kind"], revision=task["revision"], action="approve")
    mention_messages = get("/v1/user/chat/history", b, user_id=0, page=1, page_size=100)["messages"]
    check(any(row.get("column_id") == article["id"] and row.get("comment_id") == comment["id"] for row in mention_messages), "approved comment mention notification missing")
    reply = post("/v1/column/comment", operator, column_id=article["id"], parent_id=comment["id"], at_user_id=ids["a"], content="Approved reply")["item"]
    post("/v1/column/comment/action", b, id=comment["id"], action="up")
    post("/v1/column/comment/action", b, id=comment["id"], action="down")
    comments = get("/v1/column/comments", b, id=article["id"], style="hots")["items"]
    check(comments[0]["reaction"] == -1 and comments[0]["likes"] == 0 and len(comments[0]["replies"]) == 1, "reaction/reply state mismatch")
    post("/v1/column/comment/action", operator, id=comment["id"], action="feature")
    post("/v1/column/action", a, id=article["id"], action="lock")
    call("POST", "/v1/column/comment", b, data={"column_id": article["id"], "contents": [{"type": 2, "content": "Locked", "sort": 1}]}, success=False)
    post("/v1/column/action", a, id=article["id"], action="unlock")
    call("POST", "/v1/column/comment/action", a, data={"id": reply["id"], "action": "delete"}, success=False)

    self_review = submit(reviewer, "article", "Reviewer own article", f"selfreviewword{suffix}")
    self_task = next(row["task"] for row in get("/v1/admin/audit/columns", operator, page=1, page_size=100)["items"]
                     if row.get("submission", {}).get("id") == self_review["submission_id"])
    call("POST", "/v1/admin/audit/column", reviewer, data={"task_id": self_task["id"], "target_id": self_task["target_id"],
         "kind": self_task["kind"], "revision": self_task["revision"], "action": "approve"}, success=False)
    post("/v1/column/action", reviewer, id=self_review["id"], action="withdraw")
    audit_logs = get("/v1/admin/audit/logs", operator, page=1, page_size=100)["list"]
    check(any(row.get("column_id") == article["id"] and not row.get("post_id") for row in audit_logs),
          "column audit logs incorrectly target posts")

    # Remove a custom grant while a submission is pending; the same token must fail.
    exception = submit(unverified, "article", "Permission exception", f"exceptionword{suffix}")
    post("/v1/admin/user/identity", operator, user_id=ids["u"], group_ids=[])
    check(not any(row.get("submission", {}).get("id") == exception["submission_id"] for row in get("/v1/admin/audit/columns", operator, page=1, page_size=100)["items"]), "revocation left actionable publication")
    call("POST", "/v1/column/submit", unverified, data={"kind": "article", "title": "Denied again", "body": "body"}, success=False)
    post("/v1/column/action", unverified, id=exception["id"], action="delete")

    # Leave the first article available as a browser fixture; remove a different article.
    post("/v1/column/action", operator, id=second["id"], action="takedown")
    call("GET", "/v1/column", params={"id": second["id"]}, success=False)
    check(get("/v1/columns", keyword=f"secondbodyword{suffix}")["total"] == 0, "takedown searchable")
    check(len(get("/v1/column/entries", id=curated["id"])["items"]) == 1, "takedown still in public collection")
    call("POST", "/v1/column/action", b, data={"id": second["id"], "action": "restore"}, success=False)
    post("/v1/column/action", operator, id=second["id"], action="restore")
    post("/v1/column/action", a, id=collection["id"], action="delete")
    check(get("/v1/column", id=article["id"])["item"]["id"] == article["id"], "collection deletion removed article")

    if args.context:
        args.context.parent.mkdir(parents=True, exist_ok=True)
        args.context.write_text(json.dumps({"base": args.base, "password": password, "names": names,
                                           "operator": args.operator, "ids": ids,
                                           "article": article["id"], "collection": curated["id"]}))
        args.context.chmod(0o600)
    print(f"Column HTTP checks passed: {checks}; publication, review, independent search, real media, collections, discussion, notifications and revocation.")


if __name__ == "__main__":
    main()
