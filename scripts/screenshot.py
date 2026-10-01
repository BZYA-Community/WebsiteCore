"""Capture key identity pages on a disposable local instance (see development.md)."""
import os
from pathlib import Path
from playwright.sync_api import sync_playwright
from verify_sidebar_830 import BASE, login

OUT = Path(os.environ.get("E2E_SCREENSHOT_DIR", Path(__file__).parent / "shots"))
OUT.mkdir(parents=True, exist_ok=True)
PAGES = [("home", "/"), ("courses", "/courses"), ("users", "/admin/users"),
         ("audit", "/admin/audit"), ("messages", "/messages")]
token = login(os.environ["E2E_OPERATOR_USERNAME"])
with sync_playwright() as p:
    browser = p.chromium.launch()
    for width in (1600, 375):
        context = browser.new_context(viewport={"width": width, "height": 900}, locale="zh-CN")
        context.add_init_script("localStorage.setItem('PAOPAO_TOKEN', %s)" % __import__("json").dumps(token))
        page = context.new_page()
        for name, path in PAGES:
            page.goto(BASE + "/#" + path, wait_until="networkidle")
            page.locator(".content-wrap").wait_for()
            page.screenshot(path=str(OUT / f"{name}-{width}.png"))
            print(f"saved {name}-{width}.png")
        context.close()
    browser.close()
print("RESULT: 10 screenshots saved")
