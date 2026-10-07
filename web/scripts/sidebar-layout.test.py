"""Guest sidebar controls must stay visible and work in either language."""
import argparse
import json
import os
from pathlib import Path
from urllib.parse import urlsplit

from playwright.sync_api import sync_playwright

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--base', default='http://127.0.0.1:8008')
parser.add_argument('--output', default='custom/verification/sidebar-layout')
parser.add_argument('--browser-channel', default='msedge' if os.name == 'nt' else None)
args = parser.parse_args()
assert urlsplit(args.base).hostname in {'localhost', '127.0.0.1', '::1'}
output = Path(args.output)
output.mkdir(parents=True, exist_ok=True)

with sync_playwright() as p:
    browser = p.chromium.launch(channel=args.browser_channel, headless=True)
    for locale in ['en', 'zh-CN']:
        for width, register in [(887, True), (1366, True), (390, True), (887, False), (390, False)]:
            context = browser.new_context(viewport={'width': width, 'height': 844})
            context.add_init_script('localStorage.removeItem("PAOPAO_TOKEN"); localStorage.setItem("PAOPAO_THEME", "light"); localStorage.setItem("PAOPAO_LOCALE", ' + json.dumps(locale) + ');')
            def site_profile(route):
                response = route.fetch()
                body = response.json()
                body['data']['allow_user_register'] = register
                route.fulfill(response=response, json=body)
            context.route('**/v1/site/profile', site_profile)
            page = context.new_page()
            page.goto(args.base.rstrip('/') + '/#/', wait_until='networkidle')
            if width <= 821:
                page.locator('.drawer-btn').click()
            guest = page.locator('.sidebar-wrap .guest-wrap')
            guest.wait_for()
            assert guest.get_by_role('button', name='Sign Up' if locale == 'en' else '注册', exact=True).count() == int(register)
            # Check children, not just the sidebar's own bounds: overflow:hidden can mask a cut icon.
            page.wait_for_function("""() => {
                const sidebar = document.querySelector('.sidebar-wrap').getBoundingClientRect();
                const buttons = [...document.querySelectorAll('.sidebar-wrap .user-wrap button')];
                return buttons.length >= 2 && buttons.every(button => {
                    const r = button.getBoundingClientRect();
                    const content = button.querySelector('.n-button__content, .n-button__icon').getBoundingClientRect();
                    return r.left >= sidebar.left && r.right <= sidebar.right &&
                        r.top >= sidebar.top && r.bottom <= sidebar.bottom &&
                        r.left >= 0 && r.right <= innerWidth && r.bottom <= innerHeight &&
                        content.left >= r.left && content.right <= r.right;
                });
            }""")
            assert guest.evaluate("""el => {
                const buttons = [...el.querySelectorAll('button')].map(button => button.getBoundingClientRect());
                return buttons.every(r => Math.abs(r.y - buttons[0].y) < 1 && r.height >= 28) &&
                    buttons.every((r, i) => i === 0 || r.left >= buttons[i - 1].right);
            }"""), 'guest actions should align on one row without overlap'
            guest.screenshot(path=str(output / f'guest-{locale}-{width}-{register}.png'))
            label = 'Language' if locale == 'en' else '语言'
            guest.get_by_role('button', name=label, exact=True).click()
            page.get_by_role('button', name='简体中文' if locale == 'en' else 'English', exact=True).click()
            page.wait_for_function('document.documentElement.lang === ' + json.dumps('zh-CN' if locale == 'en' else 'en'))
            guest.get_by_role('button', name='登录' if locale == 'en' else 'Sign In', exact=True).wait_for()
            if page.locator('.lang-pop').is_visible():
                guest.get_by_role('button', name='语言' if locale == 'en' else 'Language', exact=True).click()
            guest.get_by_role('button', name='登录' if locale == 'en' else 'Sign In', exact=True).click()
            page.locator('.auth-card').wait_for()
            print(f'PASS {locale} {width}px registration={register}: controls visible, language switches, sign-in opens', flush=True)
            context.close()
    browser.close()
