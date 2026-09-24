# -*- coding: utf-8 -*-
"""程序化验证中间栏自适应: 在多档视口宽度下实测 .content-wrap 渲染宽度"""
from playwright.sync_api import sync_playwright

BASE = 'http://127.0.0.1:8008/#/'
VIEWPORTS = [1920, 1600, 1366, 1200, 1000, 821, 375]

with sync_playwright() as p:
    browser = p.chromium.launch()
    print(f"{'视口':>6} | {'content-wrap实际宽度':>18} | {'预期(clamp)':>12} | sidebar可见")
    print('-' * 60)
    for w in VIEWPORTS:
        ctx = browser.new_context(viewport={'width': w, 'height': 900})
        page = ctx.new_page()
        try:
            page.goto(BASE, wait_until='networkidle', timeout=20000)
        except Exception:
            pass
        page.wait_for_timeout(800)
        m = page.evaluate("""() => {
            const c = document.querySelector('.content-wrap');
            const s = document.querySelector('.sidebar-wrap, .main-wrap > div:first-child');
            return {
                w: c ? Math.round(c.getBoundingClientRect().width) : -1,
                sidebar: s ? s.getBoundingClientRect().width > 0 : false,
            };
        }""")
        expect = min(max(620, w - 640), 1000) if w > 821 else min(w, 620)
        ok = 'OK' if abs(m['w'] - expect) <= 2 else 'DIFF!'
        print(f"{w:>6} | {m['w']:>18} | {expect:>12} | {m['sidebar']}  {ok}")
        ctx.close()
    browser.close()
print('done')
