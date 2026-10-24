# -*- coding: utf-8 -*-
"""侧栏830px视口重叠验证: 按站点能力计算菜单数量，用户卡片不得压在菜单上."""
import json, os, urllib.request
from playwright.sync_api import sync_playwright

BASE = os.environ.get("WEBSITE_BASE_URL", "http://127.0.0.1:8008").rstrip("/")
API_BASE = os.environ.get("WEBSITE_API_URL", BASE).rstrip("/")
SHOTS = os.path.join(os.path.dirname(os.path.abspath(__file__)), "shots")

def login(username, password):
    r = urllib.request.Request(API_BASE + "/v1/auth/login", method="POST")
    r.add_header("Content-Type", "application/json")
    d = json.dumps({"username": username, "password": password}).encode()
    with urllib.request.urlopen(r, d) as resp:
        body = json.loads(resp.read())
    assert body["code"] == 0, body
    return body["data"]["token"]

def site_capabilities():
    with urllib.request.urlopen(API_BASE + "/v1/site/profile") as resp:
        return json.loads(resp.read())["data"]

def measure(page):
    return page.evaluate("""() => {
        const menu = document.querySelector('.sidebar-wrap .n-menu');
        const items = [...menu.querySelectorAll('.n-menu-item')];
        const last = items[items.length - 1];
        const card = document.querySelector('.sidebar-wrap .user-wrap');
        const r = (el) => { const b = el.getBoundingClientRect(); return {top: b.top, bottom: b.bottom, left: b.left, right: b.right}; };
        return {
            count: items.length,
            labels: items.map(i => i.innerText.trim()),
            lastItem: r(last),
            card: r(card),
            menuScrollable: menu.scrollHeight > menu.clientHeight,
            menuOverflow: menu.scrollHeight - menu.clientHeight,
            vh: window.innerHeight,
        };
    }""")

def main():
    os.makedirs(SHOTS, exist_ok=True)
    username = os.environ.get("SIDEBAR_TEST_USERNAME", "")
    password = os.environ.get("SIDEBAR_TEST_PASSWORD", "")
    token = os.environ.get("PAOPAO_TOKEN", "")
    if not token and username and password:
        token = login(username, password)
    capabilities = site_capabilities()
    courses_count = int(bool(capabilities.get("courses_enabled")))
    projects_count = int(bool(capabilities.get("projects_enabled")))
    expected_count = 2 + courses_count + projects_count
    if token:
        expected_count = 9 + courses_count + projects_count
    fails = 0
    with sync_playwright() as p:
        browser = p.chromium.launch()
        for w, h, name in [(1280, 830, "830px"), (1280, 900, "900px"), (1280, 740, "740px-紧凑")]:
            ctx = browser.new_context(viewport={"width": w, "height": h})
            if token:
                ctx.add_init_script("localStorage.setItem('PAOPAO_TOKEN', '%s')" % token)
            page = ctx.new_page()
            page.goto(BASE + "/", wait_until="domcontentloaded")
            page.wait_for_selector(".sidebar-wrap .user-wrap", timeout=10000)
            page.wait_for_timeout(800)
            m = measure(page)
            gap = round(m["card"]["top"] - m["lastItem"]["bottom"], 1)
            overlap = m["card"]["top"] < m["lastItem"]["bottom"] - 0.5
            ok_count = m["count"] == expected_count
            print("[%s] vh=%d 菜单项=%d %s | 最后菜单(%s) bottom=%.0f 卡片 top=%.0f 间距=%+s %s | 菜单可滚动=%s(溢出%dpx)" % (
                name, m["vh"], m["count"], "OK" if ok_count else "≠%d!" % expected_count,
                m["labels"][-1] if m["labels"] else "?", m["lastItem"]["bottom"], m["card"]["top"],
                gap, "重叠!" if overlap else "无重叠",
                m["menuScrollable"], m["menuOverflow"]))
            if overlap or not ok_count: fails += 1
            page.screenshot(path=os.path.join(SHOTS, "sidebar_%d.png" % h))
            ctx.close()
        browser.close()
    print("RESULT:", "FAIL" if fails else "ALL PASS")

if __name__ == "__main__":
    main()
