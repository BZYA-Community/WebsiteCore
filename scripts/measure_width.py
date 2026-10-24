# -*- coding: utf-8 -*-
"""程序化验证中间栏自适应: 在多档视口宽度下实测 .content-wrap 渲染宽度"""
import os
import sys
from playwright.sync_api import sync_playwright

BASE = os.environ.get('WEBSITE_BASE_URL', 'http://127.0.0.1:8008').rstrip('/') + '/#/projects'
VIEWPORTS = [1920, 1600, 1366, 1200, 1000, 821, 375]

with sync_playwright() as p:
    browser = p.chromium.launch()
    failures = 0
    print(f"{'视口':>6} | {'content-wrap实际宽度':>18} | {'预期(clamp)':>12} | 项目列数 | 横向溢出")
    print('-' * 76)
    for w in VIEWPORTS:
        ctx = browser.new_context(viewport={'width': w, 'height': 900})
        page = ctx.new_page()
        try:
            page.goto(BASE, wait_until='domcontentloaded', timeout=20000)
        except Exception:
            pass
        page.wait_for_timeout(800)
        m = page.evaluate("""() => {
            const c = document.querySelector('.content-wrap');
            const s = document.querySelector('.sidebar-wrap, .main-wrap > div:first-child');
            const mw = document.querySelector('.main-wrap');
            const grid = document.querySelector('.project-grid');
            return {
                w: c ? Math.round(c.getBoundingClientRect().width) : -1,
                base: mw ? mw.clientWidth : document.documentElement.clientWidth,
                sidebar: s ? s.getBoundingClientRect().width > 0 : false,
                columns: grid ? getComputedStyle(grid).gridTemplateColumns.split(' ').length : -1,
                overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
            };
        }""")
        # 预期基准=main-wrap内容宽(%的包含块, 桌面端已扣除gutter预留), 与CSS一致
        # >1140双侧栏(-640) 822-1140右栏隐藏(-440) <=821移动全宽
        if w > 1140:
            expect = min(m['base'] - 640, 1000)
        elif w > 821:
            expect = min(m['base'] - 440, 1000)
        else:
            expect = m['base']
        expected_columns = 1 if w <= 720 else 2
        ok = abs(m['w'] - expect) <= 2 and m['columns'] == expected_columns and not m['overflow']
        if not ok:
            failures += 1
        print(f"{w:>6} | {m['w']:>18} | {expect:>12} | {m['columns']:>8} | {m['overflow']}  {'OK' if ok else 'DIFF!'}")
        ctx.close()
    browser.close()
print('done')
sys.exit(1 if failures else 0)
