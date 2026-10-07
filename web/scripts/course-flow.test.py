"""Real local preview UI checks; keeps the course fixtures for manual review."""
import argparse
import json
import os
import time
import urllib.parse
import urllib.request
from pathlib import Path

from playwright.sync_api import sync_playwright


def main(args):
    base = args.base.rstrip("/")
    api_base = args.api_base.rstrip("/")
    for address in [base, api_base]:
        if urllib.parse.urlsplit(address).hostname not in {"localhost", "127.0.0.1", "::1"}:
            raise SystemExit("This preview check only accepts local URLs.")
    output = Path(args.output)
    output.mkdir(parents=True, exist_ok=True)
    fixture_path = Path(args.fixtures)
    fixtures = json.loads(fixture_path.read_text(encoding="utf-8"))
    accounts = json.loads(Path(args.accounts).read_text(encoding="utf-8-sig"))

    def api(method, path, data=None, session_token=None):
        headers = {"Content-Type": "application/json"}
        if session_token:
            headers["Authorization"] = "Bearer " + session_token
        request = urllib.request.Request(api_base + path, data=json.dumps(data).encode() if data is not None else None, method=method, headers=headers)
        with urllib.request.urlopen(request, timeout=30) as response:
            result = json.load(response)
        if result.get("code") != 0:
            raise AssertionError("Preview API request failed")
        return result.get("data")

    token = api("POST", "/v1/auth/login", {"username": accounts["MemberUsername"], "password": accounts["OperatorPassword"]})["token"]
    checks = []

    def check(name, condition):
        checks.append({"name": name, "passed": bool(condition)})
        print(("PASS " if condition else "FAIL ") + name, flush=True)
        if not condition:
            raise AssertionError(name)

    def storage(locale="zh-CN", theme="light", session_token=None):
        return "Object.entries(" + json.dumps({"PAOPAO_TOKEN": session_token or token, "PAOPAO_LOCALE": locale, "PAOPAO_THEME": theme}) + ").forEach(([key, value]) => localStorage.setItem(key, value));"

    def no_overflow(page, name):
        if not page.evaluate("document.documentElement.scrollWidth <= window.innerWidth + 1"):
            print(json.dumps(page.evaluate("({width: innerWidth, document: document.documentElement.scrollWidth, elements: [...document.querySelectorAll('body *')].filter(e=>e.getBoundingClientRect().right>innerWidth+1).slice(0,15).map(e=>({tag:e.tagName, class:e.className, right:e.getBoundingClientRect().right, width:e.getBoundingClientRect().width}))})")), flush=True)
            page.screenshot(path=str(output / "overflow.png"), full_page=True)
        check(name, page.evaluate("document.documentElement.scrollWidth <= window.innerWidth + 1"))
        if page.viewport_size["width"] > 821:
            page.locator('.sidebar-wrap').wait_for()
            check(name + "; sidebar stays visible and clear of content", page.evaluate("""() => {
                const sidebar = document.querySelector('.sidebar-wrap').getBoundingClientRect();
                const content = document.querySelector('.content-wrap').getBoundingClientRect();
                const rightbar = document.querySelector('.rightbar-wrap')?.getBoundingClientRect();
                return sidebar.left >= 0 && sidebar.right <= content.left + 1 &&
                    (!rightbar || (rightbar.left >= content.right - 1 && rightbar.right <= innerWidth));
            }"""))

    screenshots = []
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(channel=args.browser_channel, headless=True)
        context = browser.new_context(viewport={"width": 1366, "height": 1000}, accept_downloads=True)
        context.add_init_script(storage())
        page = context.new_page()
        errors = []
        page.on("pageerror", lambda error: errors.append(type(error).__name__))
        page.goto(f"{base}/#/", wait_until="networkidle")
        shell_style = """() => {
            const bounds = selector => {
                const rect = document.querySelector(selector).getBoundingClientRect();
                return [rect.x, rect.width];
            };
            const style = getComputedStyle(document.querySelector('.app-container'));
            return {content: bounds('.content-wrap'), sidebar: bounds('.sidebar-wrap'),
                rightbar: bounds('.rightbar-wrap'), font: style.fontFamily,
                background: getComputedStyle(document.body).backgroundColor};
        }"""
        home_style = page.evaluate(shell_style)
        page.screenshot(path=str(output / "home-desktop-zh.png"), full_page=True)
        screenshots.append("home-desktop-zh.png")
        page.goto(f"{base}/#/courses", wait_until="networkidle")
        page.locator(f'.subcategory-link[href*="category={fixtures["category"]}"]').wait_for()
        check("Catalog preserves the home page columns, font and background", page.evaluate(shell_style) == home_style)
        # Wait for the existing menu's color transition after hash navigation.
        accent_matches = "getComputedStyle(document.querySelector('.major-nav a.active')).color === getComputedStyle(document.querySelector('.n-menu-item-content--selected .n-menu-item-content-header')).color"
        page.wait_for_function(accent_matches)
        check("Catalog accent matches the existing selected sidebar item", page.evaluate(accent_matches))
        check("Catalog exposes major categories and subcategory links", page.locator(".major-section").count() >= 1)
        check("Learner catalog hides edit controls", page.get_by_role("button", name="管理课程", exact=True).count() == 0)
        no_overflow(page, "1366px category catalog has no horizontal overflow")
        page.screenshot(path=str(output / "catalog-desktop-zh.png"), full_page=True)
        screenshots.append("catalog-desktop-zh.png")
        direct_link = page.locator('.subcategory-link[href*="course="]').first
        if direct_link.count():
            title = direct_link.locator("strong").inner_text()
            direct_link.click()
            page.locator(".course-outline h3").first.wait_for()
            check("Existing direct course keeps its named entry and outline", page.locator(".course-outline h3").first.inner_text() == title)
            page.screenshot(path=str(output / "direct-course-desktop-zh.png"), full_page=True)
            screenshots.append("direct-course-desktop-zh.png")
            page.goto(f"{base}/#/courses", wait_until="networkidle")
        page.locator(f'.subcategory-link[href*="category={fixtures["category"]}"]').click()
        page.locator(f'#course-{fixtures["course"]} .lesson-link').first.wait_for()
        check("Subcategory shows its lesson links", page.locator(f'#course-{fixtures["course"]} .lesson-link').count() == 2)
        page.screenshot(path=str(output / "subcategory-desktop-zh.png"), full_page=True)
        screenshots.append("subcategory-desktop-zh.png")
        page.locator(f'#course-{fixtures["course"]} .lesson-link').first.click()
        page.wait_for_url(f"**lesson={fixtures['lessons'][0]['id']}")
        page.locator('.lesson-playlist a[aria-current=true]').wait_for()
        check("Opening a lesson starts at the top of the watch page", page.evaluate("window.scrollY < 10"))
        page.go_back(wait_until="networkidle")
        page.locator(f'#course-{fixtures["course"]} .lesson-link').first.wait_for()
        check("Browser Back restores the selected subcategory", f"category={fixtures['category']}" in page.url)
        page.go_forward(wait_until="networkidle")
        page.wait_for_url(f"**lesson={fixtures['lessons'][0]['id']}")
        page.reload(wait_until="networkidle")
        check("Reload keeps the chosen lesson", page.locator("h1").inner_text() == fixtures["lessons"][0]["title"])
        page.locator(".watch-screen").wait_for()
        check("Catalog lesson opens dedicated player page", page.locator(".watch-screen").is_visible())
        check("Watch page inherits the site font", page.locator('.course-watch').evaluate("node => getComputedStyle(node).fontFamily") == home_style["font"])
        page.goto(f"{base}/#/course?id={fixtures['course']}", wait_until="networkidle")
        page.wait_for_url(f"**lesson={fixtures['lessons'][0]['id']}")
        check("Legacy course link selects first lesson", page.locator("h1").inner_text() == fixtures["lessons"][0]["title"])
        check("Lesson outline has two real links", page.locator(".lesson-playlist a").count() == 2)
        check("Learner watch has no management controls", page.get_by_text("管理课节", exact=True).count() == 0)
        page.wait_for_function("document.querySelector('video')?.readyState >= 1")
        check("Real MP4 metadata is loaded", page.locator("video").evaluate("video => video.videoWidth > 0 && video.duration > 0"))
        first_src = page.locator("video").get_attribute("src")
        page.locator("video").evaluate("async video => { video.muted = true; await video.play(); }")
        page.wait_for_function("document.querySelector('video')?.currentTime > .2")
        check("Real MP4 plays", page.locator("video").evaluate("video => !video.paused"))
        check("First lesson disables previous", page.get_by_role("button", name="上一节", exact=True).is_disabled())
        no_overflow(page, "1366px Chinese watch has no horizontal overflow")
        page.screenshot(path=str(output / "watch-desktop-zh.png"), full_page=True)
        screenshots.append("watch-desktop-zh.png")
        page.get_by_role("button", name="下一节", exact=True).click()
        page.wait_for_url(f"**lesson={fixtures['lessons'][1]['id']}")
        page.wait_for_function("document.querySelector('video')?.readyState >= 1")
        check("Next lesson updates heading", page.locator("h1").inner_text() == fixtures["lessons"][1]["title"])
        check("Lesson switch destroys old player", page.locator("video").count() == 1 and page.locator("video").get_attribute("src") != first_src)
        check("Last lesson disables next", page.get_by_role("button", name="下一节", exact=True).is_disabled())
        page.get_by_role("button", name="上一节", exact=True).click()
        page.wait_for_url(f"**lesson={fixtures['lessons'][0]['id']}")
        check("Previous lesson returns correctly", page.locator(".lesson-playlist a[aria-current=true]").inner_text().find(fixtures["lessons"][0]["title"]) >= 0)
        page.get_by_role("tab", name="课程介绍", exact=True).focus()
        page.keyboard.press("ArrowRight")
        check("Tabs support keyboard navigation", page.get_by_role("tab", name="课节资料 (2)", exact=True).get_attribute("aria-selected") == "true")
        check("Materials belong to selected lesson", page.locator(".file-list li").count() == 2)
        downloads = []
        context.on("page", lambda tab: tab.on("download", lambda download: downloads.append(download)))
        page.locator(".file-list li").filter(has_text="course-notes.txt").get_by_role("button").click()
        deadline = time.monotonic() + 15
        while not downloads and time.monotonic() < deadline:
            page.wait_for_timeout(100)
        check("Attachment opens a browser download", len(downloads) == 1)
        download_path = Path(downloads[0].path())
        check("Downloaded handout contains real text", "WebsiteCore 体验课程讲义" in download_path.read_text(encoding="utf-8"))
        page.get_by_role("tab", name="课程问答", exact=True).click()
        page.get_by_placeholder("写下你的课程问题…").wait_for()
        check("Moderation notice remains visible", page.get_by_text("所有问题和回复均需审核，通过后对外可见。", exact=True).is_visible())
        question = fixtures.get("question")
        if not question:
            question = "体验课程：第一节资料中的练习，可以在学完第二节后一起完成吗？"
            page.get_by_placeholder("写下你的课程问题…").fill(question)
            with page.expect_response(lambda response: "/v1/course/comment" in response.url and response.request.method == "POST") as response_info:
                page.get_by_role("button", name="发布", exact=True).click()
            result = response_info.value.json()
            check("Question submission succeeds through UI", result.get("code") == 0 and result.get("data", {}).get("audit_status") == 0)
            fixtures["question"] = question
            fixture_path.write_text(json.dumps(fixtures, ensure_ascii=False, indent=2), encoding="utf-8")
        page.get_by_text(question, exact=True).wait_for()
        check("Own pending question is visible", page.locator(".comment-item").filter(has_text=question).get_by_text("审核中", exact=True).is_visible())
        page.set_viewport_size({"width": 390, "height": 844})
        page.get_by_role("tab", name="课程介绍", exact=True).click()
        no_overflow(page, "390px Chinese watch has no horizontal overflow")
        check("Mobile playlist remains reachable", page.locator(".lesson-sidebar").is_visible())
        page.screenshot(path=str(output / "watch-mobile-zh.png"), full_page=True)
        screenshots.append("watch-mobile-zh.png")
        page.goto(f"{base}/#/courses", wait_until="networkidle")
        page.locator(f'.subcategory-link[href*="category={fixtures["category"]}"]').wait_for()
        no_overflow(page, "390px category catalog has no horizontal overflow")
        page.screenshot(path=str(output / "catalog-mobile-zh.png"), full_page=True)
        screenshots.append("catalog-mobile-zh.png")
        page.locator(f'.subcategory-link[href*="category={fixtures["category"]}"]').click()
        page.locator(f'#course-{fixtures["course"]} .lesson-link').first.wait_for()
        no_overflow(page, "390px subcategory has no horizontal overflow")
        page.screenshot(path=str(output / "subcategory-mobile-zh.png"), full_page=True)
        screenshots.append("subcategory-mobile-zh.png")
        check("No unhandled browser errors", len(errors) == 0)
        for width in [375, 390, 821, 822, 1000, 1366, 1920]:
            page.set_viewport_size({"width": width, "height": 1000})
            page.goto(f"{base}/#/course?id={fixtures['course']}&lesson={fixtures['lessons'][0]['id']}", wait_until="networkidle")
            page.reload(wait_until="networkidle")
            page.locator(".watch-screen").wait_for()
            no_overflow(page, f"{width}px watch has no horizontal overflow")
            check(f"{width}px player has usable width", page.evaluate("document.querySelector('.player-stage').getBoundingClientRect().width >= Math.min(document.querySelector('.learning-layout').getBoundingClientRect().width, 450) - 1"))
            if width in [822, 1000, 1920]:
                name = f"watch-{width}-zh.png"
                page.screenshot(path=str(output / name), full_page=True)
                screenshots.append(name)
        # Start mobile with no sidebar mounted; resize without reload across both breakpoints.
        page.set_viewport_size({"width": 390, "height": 844})
        page.goto(f"{base}/#/courses", wait_until="networkidle")
        page.reload(wait_until="networkidle")
        for width in [822, 1000, 1140, 1141, 1366, 821, 390, 1000]:
            page.set_viewport_size({"width": width, "height": 844})
            page.wait_for_function("document.querySelectorAll('.sidebar-wrap').length === (innerWidth > 821 ? 1 : 0) && document.querySelectorAll('.rightbar-wrap').length === (innerWidth > 1140 ? 1 : 0)")
            no_overflow(page, f"{width}px catalog resize without reload has no horizontal overflow")
        context.close()
        for width in [1366, 390]:
            themed = browser.new_context(viewport={"width": width, "height": 1000 if width > 400 else 844})
            themed.add_init_script(storage("en", "dark"))
            view = themed.new_page()
            view.goto(f"{base}/#/course?id={fixtures['course']}&lesson={fixtures['lessons'][0]['id']}", wait_until="networkidle")
            view.get_by_role("tab", name="Introduction", exact=True).wait_for()
            check(f"{width}px English labels loaded", view.get_by_text("Current lesson", exact=True).is_visible())
            check(f"{width}px dark theme loaded", view.locator(".course-shell.dark").count() == 1)
            no_overflow(view, f"{width}px dark English watch has no horizontal overflow")
            name = f"watch-{width}-en-dark.png"
            view.screenshot(path=str(output / name), full_page=True)
            screenshots.append(name)
            view.get_by_role("tab", name="Materials (2)", exact=True).click()
            check(f"{width}px English attachment actions remain readable", view.get_by_role("button", name="Open / download", exact=True).first.is_visible() and view.locator(".file-info strong").first.is_visible())
            no_overflow(view, f"{width}px English attachments have no horizontal overflow")
            view.goto(f"{base}/#/courses", wait_until="networkidle")
            view.locator(f'.subcategory-link[href*="category={fixtures["category"]}"]').wait_for()
            no_overflow(view, f"{width}px dark English catalog has no horizontal overflow")
            name = f"catalog-{width}-en-dark.png"
            view.screenshot(path=str(output / name), full_page=True)
            screenshots.append(name)
            view.locator(f'.subcategory-link[href*="category={fixtures["category"]}"]').click()
            view.locator(f'#course-{fixtures["course"]} .lesson-link').first.wait_for()
            no_overflow(view, f"{width}px dark English subcategory has no horizontal overflow")
            name = f"subcategory-{width}-en-dark.png"
            view.screenshot(path=str(output / name), full_page=True)
            screenshots.append(name)
            themed.close()
        guest_context = browser.new_context(viewport={"width": 1366, "height": 1000})
        guest_context.add_init_script("localStorage.setItem('PAOPAO_LOCALE', 'zh-CN'); localStorage.removeItem('PAOPAO_TOKEN');")
        guest = guest_context.new_page()
        protected_requests = []
        forbidden_responses = []
        guest.on("request", lambda request: protected_requests.append(True) if any(path in request.url for path in ["/v1/course/lessons", "/v1/course/attachment", "/v1/course/video"]) else None)
        guest.on("response", lambda response: forbidden_responses.append(True) if response.status == 403 else None)
        guest.goto(f"{base}/#/course?id={fixtures['course']}&lesson={fixtures['lessons'][0]['id']}", wait_until="networkidle")
        guest.get_by_text("登录后即可查看课节并开始学习。", exact=True).wait_for()
        check("Guest watch deep link opens a clear catalog sign-in prompt", "/#/courses?" in guest.url)
        check("Guest deep link never requests protected lesson files", not protected_requests)
        check("Guest deep link does not trigger forbidden responses", not forbidden_responses)
        guest_context.close()
        if args.video:
            operator_token = api("POST", "/v1/auth/login", {"username": accounts["OperatorUsername"], "password": accounts["OperatorPassword"]})["token"]
            course = api("GET", f"/v1/course?id={fixtures['course']}", session_token=operator_token)["course"]
            check("Editor fixture is an isolated experience course", "体验课程" in course["title"])
            before = api("GET", f"/v1/course/lessons?course_id={fixtures['course']}", session_token=operator_token)["lessons"][0]
            original_materials = [item["attachment_id"] for item in before["attachments"] if not item["mime_type"].startswith("video/")]
            original_video = next(item["attachment_id"] for item in before["attachments"] if item["mime_type"].startswith("video/"))
            manager_context = browser.new_context(viewport={"width": 1366, "height": 1000})
            manager_context.add_init_script(storage(session_token=operator_token))
            manager = manager_context.new_page()
            manager.goto(f"{base}/#/courses?category={fixtures['category']}&manage=1", wait_until="networkidle")
            manager.get_by_role("button", name=f"编辑课节：{before['title']}", exact=True).click()
            manager.get_by_label("课节标题", exact=True).wait_for()
            manager.screenshot(path=str(output / "editor-desktop-zh.png"), full_page=True)
            screenshots.append("editor-desktop-zh.png")
            with manager.expect_response(lambda response: "/v1/admin/course/upload/complete" in response.url and response.request.method == "POST") as upload_response:
                manager.locator('.course-lesson-editor input[type="file"][accept="video/*"]').set_input_files(args.video)
            check("Teacher uploads replacement video through UI", upload_response.value.json().get("code") == 0)
            manager.get_by_role("button", name="保存课节", exact=True).wait_for(state="visible")
            manager.get_by_label("视频名称", exact=True).fill("experience-replacement.mp4")
            draft_intro = before["intro"] + "\n此视频已通过体验编辑器替换，讲义保持不变。"
            manager.get_by_label("课节简介", exact=True).fill(draft_intro)
            def reject_save(route):
                route.fulfill(status=503, content_type="application/json", body=json.dumps({"code": 50000, "msg": "Simulated preview save failure"}))
            manager.route("**/v1/admin/course/lesson/update", reject_save)
            manager.get_by_role("button", name="保存课节", exact=True).click()
            manager.locator(".editor-error").wait_for()
            check("Save failure preserves teacher draft and new video", manager.get_by_label("课节简介", exact=True).input_value() == draft_intro and manager.get_by_label("视频名称", exact=True).input_value() == "experience-replacement.mp4")
            check("Save failure preserves existing handout", manager.get_by_label("附件名称", exact=True).input_value() == "course-notes.txt")
            manager.unroute("**/v1/admin/course/lesson/update", reject_save)
            with manager.expect_response(lambda response: "/v1/admin/course/lesson/update" in response.url and response.request.method == "POST") as save_response:
                manager.get_by_role("button", name="保存课节", exact=True).click()
            check("Teacher can retry and save the draft", save_response.value.json().get("code") == 0)
            after = api("GET", f"/v1/course/lessons?course_id={fixtures['course']}", session_token=operator_token)["lessons"][0]
            check("Replacement updates only main video and preserves handout", next(item["attachment_id"] for item in after["attachments"] if item["mime_type"].startswith("video/")) != original_video and [item["attachment_id"] for item in after["attachments"] if not item["mime_type"].startswith("video/")] == original_materials)
            manager_context.close()
        browser.close()
    (output / "ui-results.json").write_text(json.dumps({"checks": checks, "screenshots": screenshots}, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"{len(checks)} UI checks passed. Screenshots saved under {output}.")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", default=os.getenv("COURSE_UI_BASE", "http://127.0.0.1:8008"))
    parser.add_argument("--api-base", default=os.getenv("COURSE_API_BASE", "http://127.0.0.1:8008"))
    parser.add_argument("--accounts", required=True, help="Ignored JSON with MemberUsername and OperatorPassword")
    parser.add_argument("--fixtures", required=True, help="Isolated preview course IDs; the check keeps any submitted question")
    parser.add_argument("--output", default="custom/preview/course-ui")
    parser.add_argument("--video", help="Optional local MP4 for replacing only the isolated experience lesson's video")
    parser.add_argument("--browser-channel", default=os.getenv("COURSE_BROWSER_CHANNEL", "msedge" if os.name == "nt" else None), help="Installed browser channel; defaults to Edge on Windows and Playwright Chromium elsewhere")
    main(parser.parse_args())
