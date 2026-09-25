# -*- coding: utf-8 -*-
"""私信权限矩阵验证(本地测试环境专用).

前置: 服务运行于 127.0.0.1:8008; daoyou1/daoyou2/testaudit 已绑定手机号(私信发送前提).
规则: 未绑手机禁发; 道友互发禁止; 道友->有角色者首条限制(对方回复后解除且不限量); 有角色者不限量.
"""
import json, urllib.request, urllib.error

BASE = "http://127.0.0.1:8008"
PASS = FAIL = 0
def check(name, cond, detail=""):
    global PASS, FAIL
    if cond: PASS += 1; print("  [PASS] %s" % name)
    else: FAIL += 1; print("  [FAIL] %s  %s" % (name, detail))

def call(method, path, token=None, body=None):
    req = urllib.request.Request(BASE+path, method=method)
    if token: req.add_header("Authorization", "Bearer "+token)
    data = json.dumps(body).encode() if body is not None else None
    if data: req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, data=data) as resp: return resp.status, json.loads(resp.read())
    except urllib.error.HTTPError as e:
        try: return e.code, json.loads(e.read())
        except Exception: return e.code, {}

def login(u):
    st, r = call("POST", "/v1/auth/login", body={"username": u, "password": "Test1234!"})
    assert r.get("code") == 0, r
    return r["data"]["token"]

u1, u2, auditor = login("daoyou1"), login("daoyou2"), login("testaudit")

print("== 0. 好友API已移除 ==")
st, r = call("GET", "/v1/user/contacts?page=1&page_size=5", u1)
check("GET /v1/user/contacts 404", st == 404, "%s %s" % (st, r))
st, r = call("POST", "/v1/friend/requesting", u1, {"user_id": 13, "greetings": "hi"})
check("POST /v1/friend/requesting 404", st == 404, "%s %s" % (st, r))

print("== 1. 私信权限矩阵 ==")
st, r = call("POST", "/v1/user/chat/send", u1, {"user_id": 13, "content": "道友间私信测试A"})
check("道友→道友 被拒", r.get("code") != 0, r)
st, r = call("POST", "/v1/user/chat/send", u1, {"user_id": 12, "content": "首条私信-请教问题"})
check("道友→审核员 首条成功", r.get("code") == 0, r)
st, r = call("POST", "/v1/user/chat/send", u1, {"user_id": 12, "content": "第二条-未获回复前"})
check("道友→审核员 未回复前第二条被拒", r.get("code") != 0, r)
st, r = call("POST", "/v1/user/chat/send", auditor, {"user_id": 11, "content": "审核员回复: 可以的"})
check("审核员→道友 回复成功", r.get("code") == 0, r)
ok3 = True
for i in range(3):
    st, r = call("POST", "/v1/user/chat/send", u1, {"user_id": 12, "content": "回复后连发%d" % (i+1)})
    ok3 = ok3 and r.get("code") == 0
check("道友获回复后连发3条全部成功(无日限)", ok3)
ok3 = True
for i in range(3):
    st, r = call("POST", "/v1/user/chat/send", auditor, {"user_id": 13, "content": "高级身份连发%d" % (i+1)})
    ok3 = ok3 and r.get("code") == 0
check("审核员连发3条全部成功(不限量)", ok3)

print("\nPASS=%d FAIL=%d" % (PASS, FAIL))
