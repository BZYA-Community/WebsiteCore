"""Issue #121: exercise the real chat UI with synthetic API responses.

Start the frontend with `cd web && yarn dev`, then run:
python scripts/verify_chat_notifications.py --base http://127.0.0.1:5173
Requires Playwright and Chromium. No database, account or credentials are used.
"""

import argparse
import asyncio
import json
import tempfile
from pathlib import Path
from urllib.parse import parse_qs, urlparse

from playwright.async_api import async_playwright, expect


async def verify(base, output):
    output.mkdir(parents=True, exist_ok=True)
    follow = dict(id=1, sender_id=12, sender_name="测试关注者", sender_username="follower",
                  brief="关注了你", content="快去看看 TA 的主页吧", type=99,
                  timestamp=1791331200, seen=True)
    audit = dict(follow, id=2, sender_id=0, sender_name="", sender_username="",
                 brief="动态审核通过", content="您的动态已通过审核。", post_id=42)
    hostile = dict(follow, id=3, sender_name="<img src=x onerror=alert(1)>",
                   sender_username="x&next=javascript:alert(1)", content="长通知" * 30 + "x" * 160)
    peer = dict(user_id=12, nickname="测试关注者", username="follower", avatar="",
                last_content="私信内容", last_time=1791331200, unread=0)
    mode = "normal"
    history_gate = asyncio.Event()
    history_gate.set()

    async def api(route):
        path = urlparse(route.request.url).path
        data = {}
        if path.endswith('/user/info'):
            data = dict(id=34, username="viewer", nickname="测试用户", permissions=[
                "profile.edit", "post.view", "message.initiate", "message.reply"], identity_groups=[])
        elif path.endswith('/site/profile'):
            data = dict(enable_trends_bar=False)
        elif path.endswith('/chat/contacts'):
            data = dict(system=dict(unread=2, last_content="测试关注者关注了你", last_time=1791331200), contacts=[peer])
        elif path.endswith('/chat/history'):
            await history_gate.wait()
            if mode == "failure":
                await route.fulfill(status=500, json={"code": 500, "msg": "测试加载失败"})
                return
            system = parse_qs(urlparse(route.request.url).query).get('user_id') == ['0']
            messages = [] if mode == "empty" else [follow, audit, hostile] if system else [dict(follow, type=4, content="私信内容")]
            data = dict(messages=messages, total_rows=len(messages), can_send=not system, peer=None if system else peer)
        elif path.endswith('/unread'):
            data = dict(count=0)
        else:
            data = dict(list=[], topics=[], pager=dict(total_rows=0))
        await route.fulfill(json={"code": 0, "data": data})

    async with async_playwright() as p:
        browser = await p.chromium.launch()
        for theme in ['light', 'dark']:
            for width in [1920, 1600, 1366, 1200, 1000, 830, 821, 375]:
                context = await browser.new_context(viewport={"width": width, "height": 900})
                await context.add_init_script(f"""
                    localStorage.setItem('PAOPAO_TOKEN', 'synthetic-ui-test');
                    localStorage.setItem('PAOPAO_LOCALE', 'zh-CN');
                    localStorage.setItem('PAOPAO_THEME', {json.dumps(theme)});
                """)
                await context.route('**/v1/**', api)
                page = await context.new_page()
                errors = []
                page.on('pageerror', lambda error: errors.append(str(error)))
                await page.goto(base + '/#/messages')
                # At narrow viewports the room list is shown first.
                if width <= 821:
                    await page.locator('.vac-room-item').filter(has_text='系统通知').click()
                cards = page.locator('.system-notification')
                await expect(cards).to_have_count(3)
                await expect(cards.first).to_be_visible()
                await expect(cards.first.locator('header a')).to_have_attribute('href', '#/u?s=follower')
                await expect(cards.nth(1).get_by_role('link', name='查看动态')).to_have_attribute('href', '#/post?id=42')
                await expect(cards.nth(1).get_by_role('link', name='查看主页')).to_have_count(0)
                await expect(cards.nth(2).locator('img')).to_have_count(0)
                await expect(cards.nth(2).locator('header a')).to_have_text('<img src=x onerror=alert(1)>')
                await expect(page.get_by_text('会话开始于:', exact=False)).to_have_count(0)
                for card in await cards.all():
                    bounds = await card.bounding_box()
                    assert bounds and bounds['x'] >= 0 and bounds['x'] + bounds['width'] <= width + 1, bounds
                    assert await card.evaluate('(el) => el.scrollWidth <= el.clientWidth + 1'), 'card overflows'
                assert await page.evaluate('document.documentElement.scrollWidth <= innerWidth + 1'), 'page overflows'
                await cards.first.locator('header a').focus()
                await expect(cards.first.locator('header a')).to_be_focused()
                assert not errors, errors
                await page.screenshot(path=str(output / f'chat-{theme}-{width}.png'), full_page=True)
                if width == 1920 and theme == 'light':
                    await page.keyboard.press('Enter')
                    await expect(page).to_have_url(base + '/#/u?s=follower')
                    await page.goto(base + '/#/messages')
                    await page.locator('.system-notification').nth(1).get_by_role('link', name='查看动态').click()
                    await expect(page).to_have_url(base + '/#/post?id=42')
                    await page.goto(base + '/#/messages?to=12')
                    await expect(page.locator('vue-advanced-chat textarea')).to_be_visible()
                    await expect(page.locator('.system-notification')).to_have_count(0)
                # Errors on synthetic destination pages are outside this screen's fixture.
                if width != 1920 or theme != 'light':
                    assert not errors, errors
                print(f'PASS {theme} {width}: links, text escaping, keyboard focus, layout')
                await context.close()

        context = await browser.new_context(viewport={"width": 1200, "height": 900})
        await context.add_init_script("localStorage.setItem('PAOPAO_TOKEN', 'synthetic-ui-test'); localStorage.setItem('PAOPAO_LOCALE', 'zh-CN');")
        await context.route('**/v1/**', api)
        page = await context.new_page()
        history_gate.clear()
        await page.goto(base + '/#/messages')
        await expect(page.locator('.vac-loader-wrapper').first).to_be_visible()
        await expect(page.locator('.system-notification')).to_have_count(0)
        mode = 'empty'
        history_gate.set()
        await expect(page.get_by_text('暂无消息', exact=True)).to_be_visible()
        mode = 'failure'
        await page.reload()
        await expect(page.locator('.n-message')).to_be_visible()
        await expect(page.locator('.system-notification')).to_have_count(0)
        mode = 'normal'
        await page.reload()
        await expect(page.locator('.system-notification')).to_have_count(3)
        print('PASS loading, empty, failure and recovery states')
        await context.close()
        await browser.close()
    print(f'Screenshots: {output}')


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--base', default='http://127.0.0.1:5173')
    parser.add_argument('--output', type=Path, default=Path(tempfile.gettempdir()) / 'websitecore-chat-121')
    args = parser.parse_args()
    asyncio.run(verify(args.base.rstrip('/'), args.output))
