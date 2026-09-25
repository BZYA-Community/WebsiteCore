# -*- coding: utf-8 -*-
"""课程模块 API 端到端验证(本地测试环境专用).

前置: 服务运行于 127.0.0.1:8008 且含 course 模块与迁移.
依赖账号: daoyou1(无角色)/daoyou2(无角色)/testaudit(审核员; 测试中临时提升为admin, 结束还原).
"""
import json
import os
import urllib.request
import urllib.error

BASE = "http://127.0.0.1:8008"
PASS = 0
FAIL = 0


def check(name, cond, detail=""):
    global PASS, FAIL
    if cond:
        PASS += 1
        print("  [PASS] %s" % name)
    else:
        FAIL += 1
        print("  [FAIL] %s  %s" % (name, detail))


def call(method, path, token=None, body=None, raw=False):
    req = urllib.request.Request(BASE + path, method=method)
    if token:
        req.add_header("Authorization", "Bearer " + token)
    data = None
    if body is not None:
        if raw:  # multipart 原始字节
            data = body
        else:
            data = json.dumps(body).encode("utf-8")
            req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, data=data) as resp:
            return resp.status, json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        try:
            return e.code, json.loads(e.read().decode("utf-8"))
        except Exception:
            return e.code, {}


def upload_mp4(token):
    """代理模式上传一个最小mp4(仅含ftyp头, 供API级验证)"""
    boundary = "----coursetestboundary"
    # 最小ftyp box
    payload = b"\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom"
    body = (
        "--%s\r\nContent-Disposition: form-data; name=\"file\"; filename=\"t.mp4\"\r\n"
        "Content-Type: video/mp4\r\n\r\n" % boundary
    ).encode() + payload + ("\r\n--%s--\r\n" % boundary).encode()
    req = urllib.request.Request(BASE + "/v1/admin/course/video", method="POST", data=body)
    req.add_header("Authorization", "Bearer " + token)
    req.add_header("Content-Type", "multipart/form-data; boundary=" + boundary)
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read().decode("utf-8"))


def login(u):
    st, r = call("POST", "/v1/auth/login", body={"username": u, "password": "Test1234!"})
    assert r.get("code") == 0, "login %s failed: %s" % (u, r)
    return r["data"]["token"]


def comment_ids(comments_resp):
    return {c["id"] for c in comments_resp.get("data", {}).get("list", [])}


def main():
    # PHASE=auditor 仅跑权限边界(要求testaudit为纯审核员);
    # PHASE=admin 跑管理流程(要求testaudit已提升is_admin)
    phase = os.environ.get("PHASE", "all")
    token_u1 = login("daoyou1")
    token_u2 = login("daoyou2")
    token_audit = login("testaudit")

    if phase in ("all", "auditor"):
        # ===== 0. 权限: 审核员(非admin)不可访问课程管理接口 =====
        print("== 0. 权限边界 ==")
        st, r = call("POST", "/v1/admin/course/group", token=token_audit, body={"name": "x", "sort": 0})
        check("审核员(非admin)建分组被拒(403/20022)", st == 403 and r.get("code") == 20022, "%s %s" % (st, r))
        st, r = call("GET", "/v1/admin/audit/comments?status=0&page=1&page_size=5", token=token_audit)
        check("审核员可看审核队列", r.get("code") == 0, r)
        if phase == "auditor":
            print("\nPASS=%d FAIL=%d" % (PASS, FAIL))
            return FAIL

    # ===== 1. 分组管理(需要admin身份, 由外部预置testaudit为admin后重登) =====
    print("== 1. 分组管理 ==")
    token_admin = login("testaudit")  # 外部已提升为admin
    st, r = call("POST", "/v1/admin/course/group", token=token_admin, body={"name": "测试分组A", "sort": 1})
    check("创建分组", r.get("code") == 0, r)
    gid = r.get("data", {}).get("id")
    st, r = call("GET", "/v1/course/groups")
    names = [g["name"] for g in r.get("data", {}).get("groups", [])]
    check("游客可见分组列表", "测试分组A" in names, names)
    st, r = call("POST", "/v1/admin/course/group", token=token_admin, body={"name": "   "})
    check("空白分组名被拒(400)", st == 400, "%s %s" % (st, r))

    # ===== 2. 课程创建 =====
    print("== 2. 课程创建 ==")
    v = upload_mp4(token_admin)
    check("代理上传视频", v.get("code") == 0 and v["data"]["video_url"], v)
    video_url = v["data"]["video_url"]
    check("视频URL位于attachment/course/前缀", "attachment/course/" in video_url, video_url)

    st, r = call("POST", "/v1/admin/course", token=token_admin, body={
        "group_id": gid, "teacher_id": 999999, "title": "x", "video": video_url})
    check("不存在的老师被拒(70005)", r.get("code") == 70005, "%s %s" % (st, r))
    st, r = call("POST", "/v1/admin/course", token=token_admin, body={
        "group_id": 999999, "teacher_id": 11, "title": "x", "video": video_url})
    check("不存在的分组被拒(70004)", r.get("code") == 70004, "%s %s" % (st, r))
    st, r = call("POST", "/v1/admin/course", token=token_admin, body={
        "group_id": gid, "teacher_id": 11, "title": "x", "video": "public/video/hack.mp4"})
    check("非课程前缀视频键被拒(70010)", r.get("code") == 70010, "%s %s" % (st, r))

    st, r = call("POST", "/v1/admin/course", token=token_admin, body={
        "group_id": gid, "teacher_id": 11,
        "title": "Go语言入门到精通", "intro": "本课程讲解Go语言基础语法与并发模型",
        "video": video_url, "cover": ""})
    check("创建课程(创建即上线)", r.get("code") == 0, r)
    course_id = r.get("data", {}).get("id")
    check("新课程无状态字段且计数为零", r.get("data", {}).get("play_count") == 0, r.get("data"))

    # ===== 3. 浏览与搜索(游客) =====
    print("== 3. 浏览与搜索 ==")
    st, r = call("GET", "/v1/course/list?page=1&page_size=10")
    titles = [c["title"] for c in r.get("data", {}).get("list", [])]
    check("游客可见课程列表", "Go语言入门到精通" in titles, titles)
    st, r = call("GET", "/v1/course/list?group_id=%d&page=1&page_size=10" % gid)
    check("按分组过滤", any(c["id"] == course_id for c in r["data"]["list"]), r["data"]["pager"])
    st, r = call("GET", "/v1/course/list?keyword=%s" % urllib.parse.quote("并发模型"))
    check("简介关键词搜索命中", any(c["id"] == course_id for c in r["data"]["list"]), r["data"]["pager"])
    st, r = call("GET", "/v1/course/list?keyword=%s" % urllib.parse.quote("入门"))
    check("标题关键词搜索命中", any(c["id"] == course_id for c in r["data"]["list"]), r["data"]["pager"])
    st, r = call("GET", "/v1/course/list?keyword=%s" % urllib.parse.quote("zz不存在zz"))
    check("无匹配关键词返回空", len(r["data"]["list"]) == 0, r["data"]["pager"])
    st, r = call("GET", "/v1/course?id=%d" % course_id)
    check("课程详情含老师信息", r.get("data", {}).get("course", {}).get("teacher", {}).get("username") == "daoyou1", r)

    # ===== 4. 签名播放地址 =====
    print("== 4. 签名播放地址 ==")
    st, r = call("GET", "/v1/course/video?id=%d" % course_id)
    signed = r.get("data", {}).get("signed_url", "")
    check("返回签名地址", "sign=" in signed and "expired=" in signed, signed)
    check("签名地址可访问", urllib.request.urlopen(signed.replace("127.0.0.1:8008", "127.0.0.1:8008")).status == 200)
    st_raw, _ = call("GET", video_url.replace(BASE, ""))  # 未签名直取
    check("未签名直取被拒", st_raw in (401, 403, 404), st_raw)

    # ===== 5. 播放量 =====
    print("== 5. 播放量 ==")
    st, r = call("POST", "/v1/course/play", body={"id": course_id})
    c1 = r.get("data", {}).get("play_count")
    st, r = call("POST", "/v1/course/play", body={"id": course_id})
    c2 = r.get("data", {}).get("play_count")
    check("每次播放+1(不去重)", c2 is not None and c1 is not None and c2 - c1 == 1, "%s -> %s" % (c1, c2))

    # ===== 6. 评论审核流 =====
    print("== 6. 课程评论审核 ==")
    st, r = call("POST", "/v1/course/comment", token=token_u1, body={
        "course_id": course_id,
        "contents": [{"content": "这个课程讲得真好", "type": 2, "sort": 100}],
        "users": []})
    check("无角色用户评论进入待审核", r.get("data", {}).get("audit_status") == 0, r)
    ccid = r.get("data", {}).get("id")

    st, r = call("GET", "/v1/course/comments?id=%d&page=1&page_size=10" % course_id)
    check("游客看不到待审核评论", ccid not in comment_ids(r), r)
    st, r = call("GET", "/v1/course/comments?id=%d&page=1&page_size=10" % course_id, token=token_u1)
    check("作者可见自己的待审核评论", ccid in comment_ids(r), r)

    st, r = call("GET", "/v1/admin/audit/comments?status=0&page=1&page_size=20", token=token_admin)
    row = None
    for it in r.get("data", {}).get("list", []):
        if it["comment_type"] == 2 and it["id"] == ccid:
            row = it
            break
    check("审核队列出现课程评论(comment_type=2)", row is not None, r.get("data", {}).get("list"))
    check("队列条目post_id列为课程id", row and row["post_id"] == course_id, row)
    check("队列条目含内容摘要", row and "讲得真好" in row["content"], row)

    st, r = call("POST", "/v1/admin/audit/comment", token=token_admin, body={
        "id": ccid, "comment_type": 2, "action": "approve"})
    check("审核通过课程评论", r.get("code") == 0, r)
    st, r = call("GET", "/v1/course/comments?id=%d&page=1&page_size=10" % course_id)
    check("过审后游客可见", ccid in comment_ids(r), r)
    st, r = call("GET", "/v1/course?id=%d" % course_id)
    check("课程评论计数=1", r["data"]["course"]["comment_count"] == 1, r["data"]["course"])

    # 回复(无角色 → 待审)
    st, r = call("POST", "/v1/course/comment/reply", token=token_u2, body={
        "comment_id": ccid, "at_user_id": 0, "content": "同感, 受益匪浅"})
    check("无角色用户回复进入待审核", r.get("data", {}).get("audit_status") == 0, r)
    rid = r.get("data", {}).get("id")
    st, r = call("GET", "/v1/admin/audit/comments?status=0&page=1&page_size=20", token=token_admin)
    has_reply = any(it["comment_type"] == 3 and it["id"] == rid for it in r["data"]["list"])
    check("审核队列出现课程回复(comment_type=3)", has_reply, r["data"]["list"])
    st, r = call("POST", "/v1/admin/audit/comment", token=token_admin, body={
        "id": rid, "comment_type": 3, "action": "approve"})
    check("审核通过课程回复", r.get("code") == 0, r)
    st, r = call("GET", "/v1/course/comments?id=%d&page=1&page_size=10" % course_id)
    comments = {c["id"]: c for c in r["data"]["list"]}
    check("过审后回复可见", any(rep["id"] == rid for rep in comments[ccid]["replies"]), comments[ccid])
    check("父评论reply_count=1", comments[ccid]["reply_count"] == 1, comments[ccid])
    st, r = call("GET", "/v1/course?id=%d" % course_id)
    check("课程评论计数=2(评论+回复)", r["data"]["course"]["comment_count"] == 2, r["data"]["course"])

    # 拒绝已过审评论 → 隐藏且计数回减
    st, r = call("POST", "/v1/admin/audit/comment", token=token_admin, body={
        "id": ccid, "comment_type": 2, "action": "reject", "reason": "测试打回"})
    check("拒绝已过审评论", r.get("code") == 0, r)
    st, r = call("GET", "/v1/course/comments?id=%d&page=1&page_size=10" % course_id)
    check("打回后游客不可见", ccid not in comment_ids(r), r)
    st, r = call("GET", "/v1/course?id=%d" % course_id)
    check("评论计数回减为1", r["data"]["course"]["comment_count"] == 1, r["data"]["course"])

    # ===== 7. 分组删除保护 =====
    print("== 7. 分组删除保护 ==")
    st, r = call("POST", "/v1/admin/course/group/delete", token=token_admin, body={"id": gid})
    check("非空分组删除被拒(70006)", r.get("code") == 70006, "%s %s" % (st, r))

    # ===== 8. 课程硬删除 =====
    print("== 8. 课程硬删除 ==")
    st, r = call("POST", "/v1/admin/course/delete", token=token_admin, body={"id": course_id})
    check("删除课程", r.get("code") == 0, r)
    st, r = call("GET", "/v1/course?id=%d" % course_id)
    check("删除后详情返回70003", r.get("code") == 70003, "%s %s" % (st, r))
    st, r = call("GET", "/v1/admin/audit/comments?status=0&page=1&page_size=50", token=token_admin)
    check("删除后待审队列无该课程评论", all(not (it["comment_type"] == 2 and it["id"] == ccid) for it in r["data"]["list"]), r["data"]["list"])
    st_raw, _ = call("GET", video_url.replace(BASE, "") + "?expired=9999999999&sign=bad")
    check("删除后OSS对象已清理(签名访问404)", st_raw == 404, st_raw)
    st, r = call("POST", "/v1/admin/course/group/delete", token=token_admin, body={"id": gid})
    check("清空后分组可删除", r.get("code") == 0, r)

    # ===== 9. 上传凭证(LocalOSS应为proxy) =====
    print("== 9. 上传凭证 ==")
    st, r = call("GET", "/v1/admin/course/upload-credential?ext=.mp4", token=token_admin)
    check("LocalOSS环境返回proxy模式", r.get("data", {}).get("mode") == "proxy", r)
    st, r = call("GET", "/v1/admin/course/upload-credential?ext=.exe", token=token_admin)
    check("非法扩展名被拒(70010)", r.get("code") == 70010, "%s %s" % (st, r))

    print("\nPASS=%d FAIL=%d" % (PASS, FAIL))
    return FAIL


if __name__ == "__main__":
    import sys
    import urllib.parse
    sys.exit(1 if main() else 0)
