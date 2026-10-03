"""Parallel identity UI regressions on a disposable local instance."""
import json
import os
from pathlib import Path

from playwright.sync_api import expect, sync_playwright
from identity_test_client import Fixture


def main():
    f = Fixture()
    accounts = {
        "operator": f.operator, "admin": f.admin(), "student": f.user(),
        "auditor": f.user(auditor=True), "teacher": f.user("teacher"),
        "mentor": f.user(mentor=True), "dual": f.user("teacher", mentor=True),
    }
    managed_admin = f.admin()
    disabled_admin, revoked_admin = f.admin(), f.admin()
    f.expect("Operator prepares stopped Admin", "POST", "/v1/admin/user/status", f.operator, {"id": disabled_admin["id"], "status": 2})
    f.expect("Operator prepares revoked Admin", "POST", "/v1/admin/user/role", f.operator, {"user_id": revoked_admin["id"], "role": "admin", "action": "remove"})
    out = Path(os.environ.get("E2E_SCREENSHOT_DIR", Path(__file__).parent / "shots"))
    out.mkdir(parents=True, exist_ok=True)

    def ready(page):
        page.locator(".content-wrap").wait_for()
        expect(page.locator(".n-spin-content--spinning")).to_have_count(0)

    with sync_playwright() as p:
        browser = p.chromium.launch()
        for group, user in accounts.items():
            context = browser.new_context(viewport={"width": 1600, "height": 900}, locale="zh-CN")
            context.add_init_script("localStorage.setItem('PAOPAO_TOKEN', %s); localStorage.setItem('PAOPAO_LOCALE', 'zh-CN')" % json.dumps(user["token"]))
            page = context.new_page()
            page.goto(f.base + "/#/courses", wait_until="networkidle")
            ready(page)
            create = page.get_by_role("button", name="新建课程", exact=True)
            f.check(group + " course creation visibility", create.count() == int(group in ("operator", "admin", "teacher", "dual")))
            f.check(group + " system information visibility", page.locator(".site-info").count() == int(group == "operator"))
            if group in ("operator", "admin"):
                page.goto(f.base + "/#/admin/users", wait_until="networkidle")
                ready(page)
                expect(page.locator(".permission-groups .n-tag")).to_have_count(6)
                labels = page.locator(".permission-groups .n-tag").all_text_contents()
                f.check(group + " sees six parallel groups", labels == ["运维", "管理员", "审核", "老师", "导师", "学生（道友）"])
                f.check(group + " Admin creation visibility", page.get_by_role("button", name="创建管理账户", exact=True).count() == int(group == "operator"))
                for target, action, allowed in (
                    (managed_admin, "禁言", group == "operator"),
                    (disabled_admin, "解封", group == "operator"),
                    (revoked_admin, "解封", group == "operator"),
                    (accounts["student"], "禁言", True),
                    (user, "禁言", False),
                    (f.operator, "禁言", False),
                ):
                    page.locator(".keyword-input input").fill(target["username"])
                    with page.expect_response(lambda response: "/v1/admin/user/list?" in response.url):
                        page.locator(".keyword-input input").press("Enter")
                    ready(page)
                    f.check(group + " account lifecycle action " + action, page.get_by_role("button", name=action, exact=True).count() == int(allowed))
                    if target is managed_admin:
                        page.get_by_role("button", name="详情", exact=True).click()
                        drawer = page.locator(".n-drawer")
                        expect(drawer).to_be_visible()
                        f.check(group + " Admin revocation visibility", drawer.get_by_role("button", name="移除", exact=True).count() == int(group == "operator"))
                        page.keyboard.press("Escape")
                        expect(drawer).to_have_count(0)
                for target, action, allowed in (
                    (managed_admin, "禁言", group == "operator"),
                    (disabled_admin, "解封", group == "operator"),
                    (accounts["student"], "禁言", True),
                    (f.operator, "禁言", False),
                ):
                    page.goto(f.base + "/#/u?s=" + target["username"], wait_until="networkidle")
                    ready(page)
                    page.locator(".user-opts button").click()
                    f.check(group + " public account lifecycle action " + action, page.get_by_text(action, exact=True).count() == int(allowed))
            if group in ("student", "teacher", "mentor", "dual"):
                page.goto(f.base + "/#/u?s=" + accounts["student"]["username"], wait_until="networkidle")
                ready(page)
                # The menu combines follow and private-message actions.
                page.locator(".user-opts button").click()
                contact = page.get_by_text("私信", exact=True)
                f.check(group + " Student first-contact menu", contact.count() == int(group in ("mentor", "dual")))
            context.close()

        for width in (1600, 375):
            context = browser.new_context(viewport={"width": width, "height": 900}, locale="zh-CN")
            context.add_init_script("localStorage.setItem('PAOPAO_TOKEN', %s); localStorage.setItem('PAOPAO_LOCALE', 'zh-CN')" % json.dumps(f.operator["token"]))
            page = context.new_page()
            page.goto(f.base + "/#/admin/users", wait_until="networkidle")
            ready(page)
            page.locator(".keyword-input input").fill(accounts["mentor"]["username"])
            with page.expect_response(lambda response: "/v1/admin/user/list?" in response.url):
                page.locator(".keyword-input input").press("Enter")
            ready(page)
            page.get_by_role("button", name="详情", exact=True).click()
            drawer = page.locator(".n-drawer")
            switches = drawer.get_by_role("switch")
            expect(switches).to_have_count(3)
            expect(switches.nth(0)).not_to_be_checked()
            expect(switches.nth(1)).to_be_checked()
            f.check(str(width) + " Mentor does not require Teacher switch", switches.nth(1).is_enabled())
            switches.nth(0).click()
            with page.expect_response(lambda response: response.url.endswith("/v1/admin/user/access")):
                drawer.get_by_role("button", name="保存", exact=True).click()
            ready(page)
            own = f.data("GET", "/v1/user/info", accounts["mentor"])
            f.check(str(width) + " UI grants Teacher plus Mentor", own["member_identity"] == "teacher" and own["is_mentor"])
            switches.nth(0).click()
            with page.expect_response(lambda response: response.url.endswith("/v1/admin/user/access")):
                drawer.get_by_role("button", name="保存", exact=True).click()
            ready(page)
            own = f.data("GET", "/v1/user/info", accounts["mentor"])
            f.check(str(width) + " UI removes Teacher and retains Mentor", own["member_identity"] == "student" and own["is_mentor"])
            expect(drawer.locator(".n-descriptions")).to_contain_text("导师")
            page.screenshot(path=str(out / f"mentor-drawer-{width}.png"), animations="disabled")
            context.close()

            context = browser.new_context(viewport={"width": width, "height": 900}, locale="zh-CN")
            page = context.new_page()
            for group, expected in (("mentor", ["导师"]), ("dual", ["老师", "导师"]), ("auditor", ["学生（道友）"])):
                page.goto(f.base + "/#/u?s=" + accounts[group]["username"], wait_until="networkidle")
                ready(page)
                expect(page.locator(".profile-baseinfo .username")).to_contain_text(accounts[group]["username"])
                expect(page.locator(".username .top-tag")).to_have_count(len(expected))
                labels = [label.strip() for label in page.locator(".username .top-tag").all_text_contents()]
                f.check(str(width) + " public " + group + " badges", labels == expected)
                page.screenshot(path=str(out / f"{group}-profile-{width}.png"), animations="disabled")
            context.close()
        browser.close()
    return f.finish()


if __name__ == "__main__":
    raise SystemExit(main())
