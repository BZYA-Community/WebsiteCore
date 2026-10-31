# -*- coding: utf-8 -*-
"""桌面端页面截图: 首页/长文编辑页/帖子详情(MD帖)"""
from playwright.sync_api import sync_playwright

import os

BASE = os.environ.get('WEBSITE_BASE_URL', 'http://127.0.0.1:8008').rstrip('/')
OUT = os.environ.get('SCREENSHOT_DIR', os.path.join(os.path.dirname(os.path.abspath(__file__)), 'shots'))
TOKEN = os.environ.get('PAOPAO_TOKEN', '')

PAGES = [
    ('home.png', f'{BASE}/#/', None),
    ('projects.png', f'{BASE}/#/projects', None),
    ('compose-md.png', f'{BASE}/#/compose-md', TOKEN),
    ('post-md.png', f'{BASE}/#/post?id=1080018043', None),
]

with sync_playwright() as p:
    os.makedirs(OUT, exist_ok=True)
    browser = p.chromium.launch()
    for name, url, token in PAGES:
        ctx = browser.new_context(viewport={'width': 1600, 'height': 1000})
        if token:
            ctx.add_init_script(f"localStorage.setItem('PAOPAO_TOKEN', '{token}')")
        page = ctx.new_page()
        try:
            page.goto(url, wait_until='domcontentloaded', timeout=30000)
        except Exception:
            pass  # networkidle 超时也继续截已渲染内容
        page.wait_for_timeout(1500)
        page.screenshot(path=f'{OUT}/{name}', full_page=False)
        print(f'saved {name}')
        ctx.close()
    browser.close()
print('done')
