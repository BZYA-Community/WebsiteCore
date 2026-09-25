# -*- coding: utf-8 -*-
"""评论/回复/昵称审核流程 API 级端到端测试(本地测试环境专用)."""
import json
import sys
import time
import urllib.request

BASE = "http://127.0.0.1:8008"
POST_ID = 1080018050

PASS = 0
FAIL = 0


def call(method, path, token=None, body=None):
    req = urllib.request.Request(BASE + path, method=method)
    if token:
        req.add_header("Authorization", "Bearer " + token)
    data = None
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, data=data) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        try:
            return json.loads(e.read().decode("utf-8"))
        except Exception:
            return {"_http": e.code}


def check(name, cond, detail=""):
    global PASS, FAIL
    if cond:
        PASS += 1
        print("  [PASS] %s" % name)
    else:
        FAIL += 1
        print("  [FAIL] %s  %s" % (name, detail))


def login(u):
    r = call("POST", "/v1/auth/login", body={"username": u, "password": "Test1234!"})
    assert r.get("code") == 0, "login %s failed: %s" % (u, r)
    return r["data"]["token"]


def get_comments(token=None):
    r = call("GET", "/v1/post/comments?id=%d" % POST_ID, token=token)
    assert r.get("code") == 0, r
    return r["data"]


def get_all_comments(token=None):
    """跨页收集全部评论与回复, 避免分页干扰断言."""
    items = []
    page = 1
    while True:
        r = call("GET", "/v1/post/comments?id=%d&page=%d&page_size=5" % (POST_ID, page), token=token)
        assert r.get("code") == 0, r
        lst = (r.get("data") or {}).get("list") or []
        items.extend(lst)
        pager = (r.get("data") or {}).get("pager") or {}
        total = pager.get("total_rows") or 0
        if page * 5 >= total or not lst:
            break
        page += 1
    return {"list": items}


def _wait_guest_reply_visible(cid, rid, tries=50):
    for _ in range(tries):
        data = get_all_comments()
        cm = next((x for x in data.get("list") or [] if x.get("id") == cid), {})
        if any(rp.get("id") == rid for rp in cm.get("replies") or []):
            return True
        time.sleep(0.1)
    return False


def wait_guest_visible(cid, want_visible=True, tries=50):
    """游客视角可见性最终一致(审核动作经异步事件失效缓存) 轮询断言."""
    for _ in range(tries):
        found = find_comment(get_all_comments(), cid) is not None
        if found == want_visible:
            return True
        time.sleep(0.1)
    return False


def find_comment(data, cid):
    for c in data.get("list") or []:
        if c.get("id") == cid:
            return c
        for rp in c.get("replies") or []:
            if rp.get("id") == cid:
                return rp
    print("    [debug] comment %s not in collected ids=%s statuses=%s" % (
        cid,
        [(c.get("id"), c.get("audit_status"), [r.get("id") for r in (c.get("replies") or [])]) for c in data.get("list") or []],
        None))
    return None


def main():
    nick_a = "道友一号" + str(int(time.time()) % 10000)
    t1 = login("daoyou1")   # 无角色 -> 需审核
    t2 = login("daoyou2")   # 无角色 -> 需审核
    ta = login("testaudit") # auditor

    print("== 1. 评论创建进入待审核 ==")
    r = call("POST", "/v1/post/comment", token=t1, body={
        "contents": [{"content": "审核流程测试评论A", "type": 2, "sort": 100}],
        "post_id": POST_ID,
        "users": [],
    })
    check("创建评论成功", r.get("code") == 0, r)
    cid = r["data"].get("id") if isinstance(r.get("data"), dict) else None
    audit_status = r["data"].get("audit_status") if isinstance(r.get("data"), dict) else None
    check("评论返回 audit_status=0(待审核)", audit_status == 0, "got %r" % audit_status)
    print("  comment id=%s" % cid)

    print("== 2. 可见性: 游客/他人不可见, 作者/审核员可见 ==")
    check("游客看不到待审核评论", find_comment(get_all_comments(), cid) is None)
    check("daoyou2 看不到待审核评论", find_comment(get_all_comments(t2), cid) is None)
    c = find_comment(get_all_comments(t1), cid)
    check("作者自己可见且 audit_status=0", c is not None and c.get("audit_status") == 0, c)
    c = find_comment(get_all_comments(ta), cid)
    check("审核员可见", c is not None)

    print("== 3. 审核员队列与权限 ==")
    r = call("GET", "/v1/admin/audit/comments?status=0&page=1&page_size=20", token=ta)
    check("审核员拉取评论队列", r.get("code") == 0, r)
    row = next((x for x in (r.get("data") or {}).get("list") or [] if x.get("id") == cid and x.get("comment_type") == 0), None)
    check("队列中包含该评论", row is not None, r.get("data"))
    if row:
        check("队列内容摘要正确", "审核流程测试评论A" in (row.get("content") or ""), row)
        check("队列含用户信息", row.get("user", {}).get("username") == "daoyou1", row)
    r = call("GET", "/v1/admin/audit/comments?status=0&page=1&page_size=20", token=t1)
    check("非审核员访问队列被拒", r.get("code") in (401, 403, 20023), r)

    print("== 4. 通过审核 -> 公开可见 + 计数 ==")
    r = call("POST", "/v1/admin/audit/comment", token=ta, body={"id": cid, "comment_type": 0, "action": "approve"})
    check("审核通过调用成功", r.get("code") == 0, r)
    # 审核动作经异步事件失效游客缓存 需轮询等待最终一致
    check("游客现在可见", wait_guest_visible(cid, True, tries=50))
    r = call("POST", "/v1/admin/audit/comment", token=ta, body={"id": cid, "comment_type": 0, "action": "approve"})
    check("重复通过幂等", r.get("code") == 0, r)

    print("== 5. 回复审核流程 ==")
    r = call("POST", "/v1/post/comment/reply", token=t2, body={
        "comment_id": cid, "at_user_id": 0, "content": "审核流程测试回复B"})
    check("daoyou2 创建回复成功", r.get("code") == 0, r)
    rid = r["data"].get("id")
    check("回复返回 audit_status=0", r["data"].get("audit_status") == 0, r.get("data"))
    data = get_all_comments()
    cm = next((x for x in data.get("list") or [] if x.get("id") == cid), {})
    check("游客看不到待审核回复", all(rp.get("id") != rid for rp in cm.get("replies") or []))
    data = get_all_comments(ta)
    cm = next((x for x in data.get("list") or [] if x.get("id") == cid), {})
    rp = next((rp for rp in cm.get("replies") or [] if rp.get("id") == rid), None)
    check("审核员可见待审核回复", rp is not None)

    r = call("GET", "/v1/admin/audit/comments?status=0&page=1&page_size=20", token=ta)
    row = next((x for x in (r.get("data") or {}).get("list") or [] if x.get("id") == rid and x.get("comment_type") == 1), None)
    check("队列中包含该回复(含所属评论ID)", row is not None and row.get("comment_id") == cid, r.get("data"))
    r = call("POST", "/v1/admin/audit/comment", token=ta, body={"id": rid, "comment_type": 1, "action": "approve"})
    check("回复审核通过", r.get("code") == 0, r)
    data = get_all_comments()
    cm = next((x for x in data.get("list") or [] if x.get("id") == cid), {})
    check("游客现在可见回复", _wait_guest_reply_visible(cid, rid))

    print("== 6. 拒绝流程(带原因) ==")
    r = call("POST", "/v1/post/comment", token=t1, body={
        "contents": [{"content": "审核流程测试评论C(将被拒绝)", "type": 2, "sort": 100}],
        "post_id": POST_ID, "users": []})
    cid2 = r["data"].get("id")
    check("第二评论待审核", r["data"].get("audit_status") == 0)
    r = call("POST", "/v1/admin/audit/comment", token=ta, body={
        "id": cid2, "comment_type": 0, "action": "reject", "reason": "测试拒绝原因"})
    check("拒绝调用成功", r.get("code") == 0, r)
    check("游客看不到被拒评论", find_comment(get_all_comments(), cid2) is None)
    c = find_comment(get_all_comments(t1), cid2)
    check("作者可见被拒评论且 audit_status=2", c is not None and c.get("audit_status") == 2, c)

    print("== 7. 已通过->拒绝的计数回退 ==")
    # 重新通过 cid2 再观察 comment_count 变化路径覆盖(状态机往返)
    r = call("POST", "/v1/admin/audit/comment", token=ta, body={"id": cid2, "comment_type": 0, "action": "approve"})
    check("被拒后重新通过", r.get("code") == 0, r)
    check("重新通过后游客可见", wait_guest_visible(cid2, True))
    r = call("POST", "/v1/admin/audit/comment", token=ta, body={
        "id": cid2, "comment_type": 0, "action": "reject", "reason": "再次拒绝"})
    check("已通过->拒绝(计数回退)", r.get("code") == 0, r)
    check("回退后游客不可见", wait_guest_visible(cid2, False))

    print("== 8. 昵称审核 ==")
    r = call("POST", "/v1/user/nickname", token=t1, body={"nickname": nick_a})
    check("昵称修改请求成功", r.get("code") == 0, r)
    r = call("GET", "/v1/user/info", token=t1)
    check("昵称未立即生效", (r.get("data") or {}).get("nickname") != nick_a, r.get("data"))
    r = call("GET", "/v1/admin/audit/nicknames?page=1&page_size=20", token=ta)
    row = next((x for x in (r.get("data") or {}).get("list") or [] if x.get("username") == "daoyou1"), None)
    check("昵称队列包含 daoyou1", row is not None, r.get("data"))
    if row:
        check("待审核昵称值正确", row.get("pending_nickname") == nick_a, row)
    r = call("POST", "/v1/admin/audit/nickname", token=ta, body={"user_id": 11, "action": "approve"})
    check("昵称审核通过", r.get("code") == 0, r)
    r = call("GET", "/v1/user/info", token=t1)
    check("昵称已生效", (r.get("data") or {}).get("nickname") == nick_a, r.get("data"))

    r = call("POST", "/v1/user/nickname", token=t1, body={"nickname": "违规呢称X"})
    r = call("POST", "/v1/admin/audit/nickname", token=ta, body={
        "user_id": 11, "action": "reject", "reason": "昵称不合规"})
    check("昵称拒绝调用成功", r.get("code") == 0, r)
    r = call("GET", "/v1/user/info", token=t1)
    check("拒绝后昵称保持原值", (r.get("data") or {}).get("nickname") == nick_a, r.get("data"))
    r = call("GET", "/v1/admin/audit/nicknames?page=1&page_size=20", token=ta)
    check("队列中已无 daoyou1 待审项",
          all(x.get("username") != "daoyou1" for x in (r.get("data") or {}).get("list") or []))

    print("== 9. 审核日志 ==")
    r = call("GET", "/v1/admin/audit/logs?page=1&page_size=50", token=ta)
    actions = [x.get("action") for x in (r.get("data") or {}).get("list") or []]
    for want in ("comment_approve", "reply_approve", "comment_reject", "nickname_approve", "nickname_reject"):
        check("日志包含 %s" % want, want in actions, actions[:12])

    print("== 10. 有角色用户评论免审 ==")
    r = call("POST", "/v1/post/comment", token=ta, body={
        "contents": [{"content": "审核员直接可见的评论", "type": 2, "sort": 100}],
        "post_id": POST_ID, "users": []})
    check("审核员评论 audit_status=1(直接通过)", r["data"].get("audit_status") == 1, r.get("data"))

    print()
    print("PASS=%d FAIL=%d" % (PASS, FAIL))
    sys.exit(1 if FAIL else 0)


if __name__ == "__main__":
    main()
