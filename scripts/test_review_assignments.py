"""Synthetic review checks against the disposable loopback API only."""
import json
import subprocess
import time
import argparse
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

parser = argparse.ArgumentParser(description="Run synthetic review checks on a disposable loopback WebsiteCore instance.")
parser.add_argument("--context", type=Path, required=True, help="Private JSON with OperatorUsername and OperatorPassword for the disposable instance")
parser.add_argument("--database-context", type=Path, required=True, help="Private JSON with the disposable PostgreSQL Container name")
parser.add_argument("--base", default="http://127.0.0.1:18008")
parser.add_argument("--prefix", default="rvqa", help="Synthetic username/group prefix, 2-6 lowercase letters or digits")
parser.add_argument("--existing-reviewers", default="", help="Comma-separated synthetic reviewers using the same generated test password")
parser.add_argument("--output", type=Path, help="Optional result JSON; never contains credentials or tokens")
args = parser.parse_args()
url = urllib.parse.urlsplit(args.base)
if url.scheme not in ("http", "https") or url.hostname not in ("127.0.0.1", "localhost", "::1") or url.username or url.password or url.query or url.fragment or url.path not in ("", "/"):
    parser.error("Only a disposable loopback origin is allowed")
if not (2 <= len(args.prefix) <= 6 and all(c in "abcdefghijklmnopqrstuvwxyz0123456789" for c in args.prefix)):
    parser.error("Invalid synthetic prefix")
CONTEXT = json.loads(args.context.read_text(encoding="utf-8-sig"))
DATABASE = json.loads(args.database_context.read_text(encoding="utf-8-sig"))
if not DATABASE.get("Container", "").startswith("websitecore-rbac-check-"):
    parser.error("The database must be a disposable websitecore-rbac-check-* container")
BASE, RESULTS = args.base.rstrip("/"), []
REVIEW_A, REVIEW_B = args.prefix + "a", args.prefix + "b"
VIEWER, AUTHOR = args.prefix + "viewer", args.prefix + "author"
HTTP = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def call(method, path, token=None, body=None):
    data = json.dumps(body).encode() if body is not None else None
    request = urllib.request.Request(BASE + path, data=data, method=method)
    if token:
        request.add_header("Authorization", "Bearer " + token)
    if data is not None:
        request.add_header("Content-Type", "application/json")
    try:
        with HTTP.open(request, timeout=15) as response:
            return response.status, json.load(response)
    except urllib.error.HTTPError as response:
        return response.code, json.load(response)


def check(name, condition, detail=""):
    RESULTS.append({"name": name, "passed": bool(condition), "detail": detail})
    print(("PASS " if condition else "FAIL ") + name + (": " + detail if not condition else ""))
    if not condition:
        raise AssertionError(name + ": " + detail)


def expect(name, method, path, token=None, body=None, code=0):
    status, response = call(method, path, token, body)
    check(name, response.get("code") == code, f"HTTP {status}, application code {response.get('code')}")
    return response.get("data")


def login(username):
    data = expect("Login " + username, "POST", "/v1/auth/login", body={"username": username, "password": CONTEXT["OperatorPassword"]})
    return data["token"]


def assign(operator, user_id, group_ids, name):
    expect(name, "POST", "/v1/admin/user/identity", operator, {"user_id": user_id, "group_ids": group_ids})


def sql(statement):
    container = DATABASE["Container"]
    if not container.startswith("websitecore-rbac-check-"):
        raise RuntimeError("Refusing non-test database")
    return subprocess.run(["docker", "exec", container, "psql", "-U", "rbac_test", "-d", "rbac_test", "-v", "ON_ERROR_STOP=1", "-At", "-c", statement], check=True, capture_output=True, text=True).stdout.strip()


def main():
    tokens, users = {}, {}
    for name in dict.fromkeys([CONTEXT["OperatorUsername"], *filter(None, args.existing_reviewers.split(",")), REVIEW_A, REVIEW_B, VIEWER, AUTHOR]):
        if name in (REVIEW_A, REVIEW_B, VIEWER, AUTHOR):
            _, response = call("POST", "/v1/auth/register", body={"username": name, "password": CONTEXT["OperatorPassword"]})
            check("Register synthetic " + name, response.get("code") in (0, 20001), f"application code {response.get('code')}")
        tokens[name] = login(name)
        users[name] = expect("Inspect synthetic " + name, "GET", "/v1/user/info", tokens[name])["id"]
    operator = tokens[CONTEXT["OperatorUsername"]]
    groups = expect("Read identity catalog", "GET", "/v1/admin/identity/groups", operator)["groups"]
    def group(key, permissions):
        existing = next((g for g in groups if g["key"] == key), {})
        return expect("Configure " + key, "POST", "/v1/admin/identity/groups", operator,
                      {**existing, "key": key, "name": key, "permissions": permissions})["groups"][0]
    reviewers = group(args.prefix + "_reviewers", ["content.review"])
    viewonly = group(args.prefix + "_readonly", ["audit.view_all"])
    for name in (REVIEW_A, REVIEW_B):
        assign(operator, users[name], [reviewers["id"]], "Grant reviewer " + name)
    assign(operator, users[VIEWER], [viewonly["id"]], "Grant audit visibility only")
    sql(f"UPDATE p_user SET email = username || '@fixture.invalid' WHERE username = '{AUTHOR}'")
    author = tokens[AUTHOR]
    viewer = tokens[VIEWER]
    by_id = {users[n]: token for n, token in tokens.items()}
    def queue(kind="posts", token=operator, status=0):
        return expect("Read " + kind + " queue", "GET", "/v1/admin/audit/" + kind + f"?status={status}&page=1&page_size=100", token)["list"] or []
    def post(label):
        result = expect("Submit " + label, "POST", "/v1/post", author,
                        {"contents": [{"type": 2, "sort": 1, "content": label}], "tags": [], "users": [], "visibility": 0})
        check(label + " is pending", result["audit_status"] == 0)
        return result
    def task_for(post_id):
        return next(item["review_task"] for item in queue() if item["id"] == post_id)
    def decision(task, action="approve"):
        return {"post_id": task["target_id"], "task_id": task["id"], "revision": task["revision"], "action": action, "reason": "Synthetic rejection" if action == "reject" else ""}

    p = post("Review HTTP assignment scope")
    task = task_for(p["id"])
    check("Immediate random assignment excludes author", task["assignee_id"] in by_id and task["assignee_id"] != users[AUTHOR])
    check("Default deadline exactly 48 hours", task["deadline_on"] - task["assigned_on"] == 48 * 3600)
    check("Task snapshot not leaked in JSON", "snapshot" not in task)
    check("View-only sees all pending tasks", any(item["id"] == p["id"] for item in queue(token=viewer)))
    expect("View-only cannot decide", "POST", "/v1/admin/audit/post", viewer, decision(task), code=20007)
    other = next(n for n in (REVIEW_A, REVIEW_B) if users[n] != task["assignee_id"])
    check("Unassigned reviewer queue excludes target", all(item["id"] != p["id"] for item in queue(token=tokens[other])))
    expect("Unassigned reviewer cannot decide", "POST", "/v1/admin/audit/post", tokens[other], decision(task), code=20007)
    expect("Unassigned reviewer cannot read history", "GET", f"/v1/admin/audit/history?task_id={task['id']}", tokens[other], code=20007)
    expect("Missing revision rejected", "POST", "/v1/admin/audit/post", operator, {"post_id": p["id"], "action": "approve"}, code=10001)
    expect("Wrong target-task pairing rejected", "POST", "/v1/admin/audit/post", operator, {**decision(task), "post_id": p["id"] + 1000000}, code=20007)
    expect("Assigned reviewer approves", "POST", "/v1/admin/audit/post", by_id[task["assignee_id"]], decision(task))
    expect("Duplicate decision rejected", "POST", "/v1/admin/audit/post", operator, decision(task), code=10001)
    approved = expect("Approved post visible", "GET", f"/v1/post?id={p['id']}", author)
    check("Approved post state persisted", approved["audit_status"] == 1)

    comment = expect("Submit pending comment", "POST", "/v1/post/comment", author,
                     {"post_id": p["id"], "contents": [{"type": 2, "sort": 1, "content": "Full question content " * 30}], "users": []})
    row = next(item for item in queue("comments") if item["id"] == comment["id"] and item["comment_type"] == 0)
    check("Queue preserves full review content", len(row["content"]) > 300 and len(row["contents"]) == 1)
    ct = row["review_task"]
    expect("Assigned reviewer approves comment", "POST", "/v1/admin/audit/comment", by_id[ct["assignee_id"]], {"id": comment["id"], "comment_type": 0, "task_id": ct["id"], "revision": ct["revision"], "action": "approve"})
    approved = expect("Read post count", "GET", f"/v1/post?id={p['id']}", author)
    check("Review count committed exactly once", approved["comment_count"] == 1)

    expect("First pending nickname", "POST", "/v1/user/nickname", author, {"nickname": "HTTPone"})
    first = next(row for row in queue("nicknames") if row["user_id"] == users[AUTHOR])["review_task"]
    expect("Replace pending nickname", "POST", "/v1/user/nickname", author, {"nickname": "HTTPtwo"})
    row = next(row for row in queue("nicknames") if row["user_id"] == users[AUTHOR])
    second = row["review_task"]
    check("Profile replacement advances revision", second["revision"] == first["revision"] + 1 and row["pending_nickname"] == "HTTPtwo")
    body = {"user_id": users[AUTHOR], "task_id": first["id"], "revision": first["revision"], "action": "approve"}
    expect("Stale profile review cannot approve replacement", "POST", "/v1/admin/audit/nickname", operator, body, code=10001)
    expect("Fresh profile review succeeds", "POST", "/v1/admin/audit/nickname", by_id[second["assignee_id"]], {**body, "revision": second["revision"]})
    info = expect("Read approved profile", "GET", "/v1/user/info", author)
    check("Only replacement nickname becomes visible", info["nickname"] == "HTTPtwo")

    timed = post("Review HTTP deadline transfer")
    before = task_for(timed["id"])
    sql(f"UPDATE p_review_task SET deadline_on = EXTRACT(EPOCH FROM now())::bigint - 1 WHERE id = {int(before['id'])}")
    until = time.monotonic() + 75
    after = before
    while time.monotonic() < until:
        time.sleep(2)
        after = task_for(timed["id"])
        if after["assignee_id"] != before["assignee_id"]:
            break
    check("Worker automatically transfers expired task", after["assignee_id"] > 0 and after["assignee_id"] != before["assignee_id"])
    events = expect("Read transferred task history", "GET", f"/v1/admin/audit/history?task_id={after['id']}", viewer)["items"]
    check("Exactly one timeout recorded", sum(e["event"] == "timeout" for e in events) == 1)
    stats = expect("Read reviewer timeout statistics", "GET", "/v1/admin/audit/statistics", viewer)["items"]
    check("Reviewer timeout statistic increments", any(s["user_id"] == before["assignee_id"] and s["timeout_count"] >= 1 for s in stats))
    expect("Transferred reviewer completes task", "POST", "/v1/admin/audit/post", by_id[after["assignee_id"]], decision(after, "reject"))

    override = post("Review HTTP operator override")
    ot = task_for(override["id"])
    # Random assignment can select the operator. Keep submitting until a
    # non-operator task exists, without manipulating any assignment row.
    for attempt in range(12):
        if ot["assignee_id"] != users[CONTEXT["OperatorUsername"]]:
            break
        expect("Complete operator's own assignment", "POST", "/v1/admin/audit/post", operator, decision(ot))
        override = post("Review HTTP operator override " + str(attempt))
        ot = task_for(override["id"])
    check("Fixture has non-operator assignment", ot["assignee_id"] != users[CONTEXT["OperatorUsername"]])
    expect("Operator overrides assignment", "POST", "/v1/admin/audit/post", operator, decision(ot))
    events = expect("Read override history", "GET", f"/v1/admin/audit/history?task_id={ot['id']}", operator)["items"]
    check("Operator override is explicitly recorded", any(e["event"] == "operator_override" for e in events))
    expect("Revoke synthetic reviewers permission", "POST", "/v1/admin/identity/groups", operator, {**reviewers, "permissions": []})
    expect("Same reviewer JWT loses queue access", "GET", "/v1/admin/audit/posts", tokens[REVIEW_A], code=20007)
    expect("Restore synthetic review permission", "POST", "/v1/admin/identity/groups", operator, reviewers)
    print("Synthetic read-only username:", VIEWER)


if __name__ == "__main__":
    try:
        main()
    finally:
        if args.output:
            args.output.write_text(json.dumps(RESULTS, indent=2), encoding="utf-8")
        print("Review HTTP checks:", sum(r["passed"] for r in RESULTS), "passed;", sum(not r["passed"] for r in RESULTS), "failed")
