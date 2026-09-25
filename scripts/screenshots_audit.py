# -*- coding: utf-8 -*-
"""审核功能前端页面无头浏览器截图(本地测试环境专用).

截图项:
  1. 首页(游客)
  2. 帖子详情 + comment_id 定位高亮(审核员视角, 校验 .audit-highlight)
  3. 审核后台·评论队列(含待审核条目)
  4. 审核后台·昵称队列(含待审核条目)
  5. 用户主页(昵称审核跳转目标)
"""
import json
import os
import sys
import time
import urllib.request

from playwright.sync_api import sync_playwright

BASE = "http://127.0.0.1:8008"
POST_ID = 1080018050
SHOTS = os.path.join(os.path.dirname(os.path.abspath(__file__)), "shots")

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


def call(method, path, token=None, body=None):
    req = urllib.request.Request(BASE + path, method=method)
    if token:
        req.add_header("Authorization", "Bearer " + token)
    data = None
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        req.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(req, data=data) as resp:
        return json.loads(resp.read().decode("utf-8"))


def login(u):
    r = call("POST", "/v1/auth/login", body={"username": u, "password": "Test1234!"})
    assert r.get("code") == 0, "login %s failed: %s" % (u, r)
    return r["data"]["token"]


def shot(page, name):
    page.screenshot(path=os.path.join(SHOTS, name), full_page=True)
    print("  [shot] %s" % name)


def main():
    os.makedirs(SHOTS, exist_ok=True)
    token_a = login("testaudit")
    token_1 = login("daoyou1")

    # 造数: 一条待审核评论 + 一个待审核昵称(队列截图内容)
    stamp = int(time.time()) % 10000
    r = call("POST", "/v1/post/comment", token=token_1, body={
        "contents": [{"content": "无头浏览器截图用待审核评论%d" % stamp, "type": 2, "sort": 100}],
        "post_id": POST_ID, "users": []})
    assert r.get("code") == 0, r
    cid = r["data"]["id"]
    r = call("POST", "/v1/user/nickname", token=token_1, body={"nickname": "昵称待审%d" % stamp})
    assert r.get("code") == 0, r
    print("prepared pending comment=%d nickname=昵称待审%d" % (cid, stamp))

    with sync_playwright() as p:
        browser = p.chromium.launch()

        # ---- 1. 首页(游客) ----
        print("== 1. 首页(游客) ==")
        ctx = browser.new_context(viewport={"width": 1280, "height": 900}, locale="zh-CN")
        page = ctx.new_page()
        page.goto(BASE + "/#/", wait_until="networkidle")
        check("首页渲染出动态流", page.locator(".post-item, .dynamic-line, article").count() > 0,
              "post items not found")
        shot(page, "01-home-guest.png")
        ctx.close()

        # ---- 2. 帖子详情+评论高亮(审核员) ----
        print("== 2. 帖子详情+评论高亮 ==")
        ctx = browser.new_context(viewport={"width": 1280, "height": 900}, locale="zh-CN")
        ctx.add_init_script("localStorage.setItem('PAOPAO_TOKEN','%s')" % token_a)
        page = ctx.new_page()
        page.goto(BASE + "/#/post?id=%d&comment_id=%d" % (POST_ID, cid), wait_until="networkidle")
        hl = page.locator(".audit-highlight")
        ok = hl.count() > 0
        if not ok:  # 高亮5秒内需捕获 重试等待
            try:
                page.wait_for_selector(".audit-highlight", timeout=4000)
                ok = True
            except Exception:
                pass
        if ok:
            try:
                page.wait_for_function(
                    "window.scrollY > 20 || document.documentElement.scrollTop > 20", timeout=2500)
            except Exception:
                pass
            time.sleep(0.6)
        check("目标评论被高亮定位", ok and hl.count() > 0)
        if hl.count():
            check("高亮元素为目标评论", hl.first.get_attribute("id") == "comment-%d" % cid,
                  hl.first.get_attribute("id"))
        shot(page, "02-post-highlight.png")
        ctx.close()

        # ---- 3/4. 审核后台·评论/昵称队列 ----
        print("== 3. 审核后台·评论队列 ==")
        ctx = browser.new_context(viewport={"width": 1280, "height": 900}, locale="zh-CN")
        ctx.add_init_script("localStorage.setItem('PAOPAO_TOKEN','%s')" % token_a)
        page = ctx.new_page()
        page.goto(BASE + "/#/admin/audit", wait_until="networkidle")
        page.wait_for_selector("text=内容审核", timeout=5000)
        page.locator('.n-tabs-tab:has-text("评论")').first.click()
        page.wait_for_timeout(800)
        body = page.inner_text("body")
        check("评论队列含待审核条目", "无头浏览器截图用待审核评论%d" % stamp in body)
        shot(page, "03-audit-comments.png")

        print("== 4. 审核后台·昵称队列 ==")
        page.locator('.n-tabs-tab:has-text("昵称")').first.click()
        page.wait_for_timeout(800)
        body = page.inner_text("body")
        check("昵称队列含待审核条目", ("昵称待审%d" % stamp) in body)
        check("昵称队列含daoyou1", "daoyou1" in body)
        shot(page, "04-audit-nicknames.png")
        ctx.close()

        # ---- 5. 用户主页 ----
        print("== 5. 用户主页 ==")
        ctx = browser.new_context(viewport={"width": 1280, "height": 900}, locale="zh-CN")
        ctx.add_init_script("localStorage.setItem('PAOPAO_TOKEN','%s')" % token_a)
        page = ctx.new_page()
        page.goto(BASE + "/#/u?s=daoyou1", wait_until="networkidle")
        page.wait_for_timeout(800)
        body = page.inner_text("body")
        check("用户主页展示daoyou1", "daoyou1" in body)
        shot(page, "05-user-profile.png")
        ctx.close()

        browser.close()

    print()
    print("PASS=%d FAIL=%d" % (PASS, FAIL))
    sys.exit(1 if FAIL else 0)


if __name__ == "__main__":
    main()
