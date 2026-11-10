"""Fixture-only browser BVT of the production Projects bundle.

Run after `npm run build` with Python Playwright and its Chromium installed:
    python -m pip install playwright
    python -m playwright install --with-deps chromium
    python scripts/project-browser.test.py

No backend, real account, uploads, or external HTTP services are used. Failure
screenshots/traces and the geometry report go to browser-test-artifacts/projects.
"""

import asyncio
from copy import deepcopy
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
import json
from pathlib import Path
import re
from threading import Thread
from urllib.parse import parse_qs, urlsplit

from playwright.async_api import async_playwright, expect


WEB = Path(__file__).resolve().parents[1]
OUTPUT = WEB / "browser-test-artifacts" / "projects"
WIDTHS = (1920, 1600, 1366, 1200, 1000, 821, 375)
TITLE = "CommunityProjectShowcase2026XY"
SECOND_TITLE = "Second fixture project"
TAGS = ["AI/ML", "Web", "Mobile", "Hardware", "Data", "In Progress", "Completed", "Recruiting"]
IMAGE_ORIGIN = "https://images.example.test"
PERMISSIONS = ["post.view", "post.create", "content.upload", "community.interact", "profile.edit"]
OWNER = {
    "id": 1, "username": "fixture-owner", "nickname": "Community Owner",
    "avatar": f"{IMAGE_ORIGIN}/owner.png", "permissions": PERMISSIONS,
    "identity_groups": [],
}
CONTRIBUTOR = {
    "id": 2, "username": "fixture-contributor", "nickname": "Community Contributor",
    "avatar": f"{IMAGE_ORIGIN}/contributor.png",
}
PROJECT = {
    "id": 1, "creator_id": 1, "creator": OWNER, "post_id": 5, "title": TITLE,
    "intro": "## Project overview\nA useful community project with an accessible introduction.",
    "url": "https://github.com/BZYA-Community/WebsiteCore",
    "images": [f"{IMAGE_ORIGIN}/original-1.png", f"{IMAGE_ORIGIN}/original-2.png"],
    "tags": TAGS,
    "members": [{"user": OWNER, "role": "owner"}, {"user": CONTRIBUTOR, "role": "member"}],
    "follow_count": 3, "followed": True, "created_on": 1700000000,
}
assert len(TITLE) == 30


class StaticHandler(SimpleHTTPRequestHandler):
    def log_message(self, *_args):
        pass


class Fixtures:
    def __init__(self, base, user):
        self.base = base
        self.user = deepcopy(user)
        self.projects = {1: deepcopy(PROJECT), 2: {**deepcopy(PROJECT), "id": 2, "title": SECOND_TITLE}}
        self.writes = []
        self.unexpected_writes = []
        self.blocked_external = []
        self.gate = None
        self.received = asyncio.Event()
        self.fail_next_update = False

    def hold_writes(self):
        self.received.clear()
        self.gate = asyncio.Event()

    def release_writes(self):
        if self.gate:
            self.gate.set()
            self.gate = None

    async def route(self, route):
        request = route.request
        parsed = urlsplit(request.url)
        path = parsed.path
        if path.startswith("/v1/"):
            await self.api(route, path)
        elif f"{parsed.scheme}://{parsed.netloc}" == IMAGE_ORIGIN:
            await route.fulfill(content_type="image/svg+xml", body=(
                '<svg xmlns="http://www.w3.org/2000/svg" width="400" height="240">'
                '<rect width="400" height="240" fill="#345678"/></svg>'
            ))
        elif f"{parsed.scheme}://{parsed.netloc}" == self.base:
            await route.continue_()
        else:
            # Optional editor CDN resources must never reach the public network.
            self.blocked_external.append(request.url)
            await route.abort("blockedbyclient")

    async def api(self, route, path):
        request = route.request
        data = {"list": [], "topics": [], "extral_topics": [], "messages": [],
                "pager": {"total_rows": 0, "page": 1, "page_size": 12}, "count": 0}
        if path == "/v1/site/profile":
            data = {"projects_enabled": True, "courses_enabled": False,
                    "default_msg_loop_interval": 600000, "trusted_external_domains": ["github.com"]}
        elif path == "/v1/user/info":
            data = self.user
        elif path == "/v1/identity":
            data = {"permissions": ["post.view"]}
        elif path == "/v1/projects":
            data = {"list": list(self.projects.values()),
                    "pager": {"total_rows": len(self.projects), "page": 1, "page_size": 12}}
        elif path == "/v1/project" and request.method == "GET":
            project_id = int(parse_qs(urlsplit(request.url).query)["id"][0])
            data = {"project": self.projects[project_id]}
        elif path in ("/v1/project", "/v1/project/update") and request.method == "POST":
            payload = request.post_data_json
            self.writes.append({"path": path, "payload": deepcopy(payload)})
            gate = self.gate
            self.received.set()
            if gate:
                await asyncio.wait_for(gate.wait(), timeout=15)
            if path.endswith("/update") and self.fail_next_update:
                self.fail_next_update = False
                await route.fulfill(json={"code": 30026, "msg": "Fixture update rejected"})
                return
            project_id = payload.get("id", 10)
            data = {**deepcopy(self.projects.get(project_id, PROJECT)), **payload, "id": project_id}
            self.projects[project_id] = deepcopy(data)
        elif request.method not in ("GET", "HEAD"):
            self.unexpected_writes.append({"path": path, "method": request.method})
            await route.fulfill(status=400, json={"code": 400, "msg": "Unexpected fixture write"})
            return
        await route.fulfill(json={"code": 0, "data": deepcopy(data)})


# Measure actual boxes, including nested scroll widths, so overflow:hidden cannot
# silently mask a clipped heading, contributor, or action row. Vertical page
# scrolling is expected; only the modal itself must fit inside the viewport.
GEOMETRY = """({selectors, modal}) => {
    const errors = [];
    const boxes = [];
    const width = document.documentElement.clientWidth;
    if (document.documentElement.scrollWidth > innerWidth + 1)
        errors.push('document horizontal overflow');
    for (const selector of selectors) {
        const elements = [...document.querySelectorAll(selector)];
        if (!elements.length) errors.push('missing ' + selector);
        for (const el of elements) {
            if (!el.getClientRects().length) continue;
            const r = el.getBoundingClientRect();
            boxes.push({selector, x: r.x, y: r.y, width: r.width, height: r.height,
                clientWidth: el.clientWidth, scrollWidth: el.scrollWidth});
            if (r.left < -1 || r.right > width + 1)
                errors.push(selector + ' outside horizontal viewport');
            if (el.scrollWidth > el.clientWidth + 1)
                errors.push(selector + ' clipped/scrolling horizontally');
        }
    }
    if (modal) {
        const el = document.querySelector(modal);
        if (el) {
            const r = el.getBoundingClientRect();
            if (r.top < -1 || r.bottom > innerHeight + 1)
                errors.push('modal outside vertical viewport');
        }
    } else {
        const a = document.querySelector('.detail-heading')?.getBoundingClientRect();
        const b = document.querySelector('.detail-header > .project-actions')?.getBoundingClientRect();
        if (a && b && Math.min(a.right, b.right) > Math.max(a.left, b.left) + 1 &&
            Math.min(a.bottom, b.bottom) > Math.max(a.top, b.top) + 1)
            errors.push('detail heading overlaps actions');
    }
    return {viewport: [innerWidth, innerHeight], documentWidth: document.documentElement.scrollWidth,
        errors, boxes};
}"""


class BVT:
    def __init__(self, browser, base):
        self.browser = browser
        self.base = base
        self.report = []

    async def run_case(self, name, test, user=OWNER):
        context = await self.browser.new_context(
            viewport={"width": 1366, "height": 900}, reduced_motion="reduce", service_workers="block"
        )
        context.set_default_timeout(8000)
        fixtures = Fixtures(self.base, user)
        await context.route("**/*", fixtures.route)
        await context.add_init_script("""
            localStorage.setItem('PAOPAO_TOKEN', 'bvt-fixture-token');
            localStorage.setItem('PAOPAO_LOCALE', 'en');
            localStorage.setItem('PAOPAO_THEME', 'light');
        """)
        await context.tracing.start(screenshots=True, snapshots=True, sources=True)
        page = await context.new_page()
        errors = []
        page.on("pageerror", lambda error: errors.append(str(error)))
        failed = False
        try:
            await test(page, fixtures)
            assert not errors, f"Uncaught browser errors: {errors}"
            assert not fixtures.unexpected_writes, fixtures.unexpected_writes
            print(f"PASS {name}", flush=True)
        except Exception:
            failed = True
            try:
                await page.screenshot(path=str(OUTPUT / f"{name}.png"), full_page=True, timeout=5000)
            except Exception as error:
                print(f"Could not capture screenshot: {error}", flush=True)
            (OUTPUT / f"{name}-diagnostics.json").write_text(json.dumps({
                "url": page.url, "pageErrors": errors, "writes": fixtures.writes,
                "blockedExternal": fixtures.blocked_external,
                "unexpectedWrites": fixtures.unexpected_writes,
            }, indent=2), encoding="utf-8")
            raise
        finally:
            fixtures.release_writes()
            await context.tracing.stop(path=str(OUTPUT / f"{name}-trace.zip") if failed else None)
            await context.close()
            (OUTPUT / "geometry.json").write_text(json.dumps(self.report, indent=2), encoding="utf-8")

    async def widths(self, page, name, selectors, modal=None):
        for width in WIDTHS:
            await page.set_viewport_size({"width": width, "height": 900 if width > 821 else 844})
            arguments = {"selectors": selectors, "modal": modal}
            try:
                # Retry through Vue's resize render and Naive UI's modal transition.
                await page.wait_for_function(
                    f"args => ({GEOMETRY})(args).errors.length === 0", arg=arguments, timeout=5000
                )
            finally:
                measurements = await page.evaluate(GEOMETRY, arguments)
                self.report.append({"screen": name, **measurements})
                if measurements["errors"]:
                    print(json.dumps(self.report[-1], indent=2), flush=True)
            assert not measurements["errors"], f"{name} {width}px: {measurements['errors']}"
            print(f"PASS {name} {width}px geometry", flush=True)
        await page.set_viewport_size({"width": 1366, "height": 900})


async def open_create(page):
    await page.get_by_role("button", name="Share a project", exact=True).click()
    modal = page.locator(".project-editor")
    await expect(modal).to_be_visible()
    return modal


async def open_edit(page):
    await page.get_by_role("button", name="Edit", exact=True).click()
    modal = page.locator(".project-editor-modal")
    await expect(modal).to_be_visible()
    return modal


async def add_image(modal, name):
    url = f"{IMAGE_ORIGIN}/{name}.png"
    field = modal.get_by_placeholder("Paste an image URL and press Enter or click Add", exact=True)
    before = await modal.locator(".n-upload-file").count()
    await field.fill(url)
    await modal.get_by_role("button", name="Add", exact=True).click()
    await expect(modal.locator(".n-upload-file")).to_have_count(before + 1)
    await expect(field).to_have_value("")
    return url


async def remove_first_image(modal):
    files = modal.locator(".n-upload-file")
    before = await files.count()
    await files.first.hover()
    # Naive UI image-card actions render Preview followed by Remove (no label).
    await files.first.locator(".n-upload-file-info__action button").last.click()
    await expect(files).to_have_count(before - 1)


async def fill_create(page, modal):
    await modal.get_by_placeholder("Give your project a clear name", exact=True).fill(TITLE)
    editor = modal.locator(".cm-content[contenteditable='true']")
    await editor.fill(PROJECT["intro"])
    await modal.get_by_placeholder("https://", exact=True).fill(PROJECT["url"])
    # Exercise the actual Naive UI tag picker, not component internals.
    tag_input = modal.locator(".tags-select input")
    for tag in TAGS:
        await modal.locator(".tags-select .n-base-selection").click(position={"x": 5, "y": 5})
        await tag_input.fill(tag)
        await page.locator(".n-base-select-option__content").filter(has_text=re.compile("^" + re.escape(tag) + "$")).click()
    await modal.get_by_placeholder("Give your project a clear name", exact=True).click()
    await expect(modal.locator(".tags-select .n-tag")).to_have_count(8)


async def assert_limit(page, modal):
    field = modal.get_by_placeholder("Paste an image URL and press Enter or click Add", exact=True)
    await field.fill(f"{IMAGE_ORIGIN}/seventh-rejected.png")
    await modal.get_by_role("button", name="Add", exact=True).click()
    await expect(page.get_by_text("Maximum 6 images allowed", exact=True)).to_be_visible()
    await expect(modal.locator(".n-upload-file")).to_have_count(6)
    await field.fill("")


async def submit_once(page, modal, fixtures, button_name, path, images):
    before = len(fixtures.writes)
    fixtures.hold_writes()
    await modal.get_by_role("button", name=button_name, exact=True).click()
    await asyncio.wait_for(fixtures.received.wait(), timeout=8)
    await expect(modal.get_by_role("button", name="Cancel", exact=True)).to_be_disabled()
    # A second form submission while the first response is pending must be ignored.
    await modal.locator("form").dispatch_event("submit")
    async with page.expect_response(lambda response: urlsplit(response.url).path == path and
                                    response.request.method == "POST"):
        fixtures.release_writes()
    await expect(modal).not_to_be_visible()
    assert len(fixtures.writes) == before + 1, "Repeated submit sent multiple API writes"
    payload = fixtures.writes[-1]["payload"]
    assert payload["images"] == images, f"Image order/removal mismatch: {payload['images']}"
    assert len(payload["images"]) == len(set(payload["images"])) == 6, "Duplicate image payload"
    assert payload["tags"] == TAGS


async def create_flow(bvt, page, fixtures):
    await page.goto(bvt.base + "/#/projects")
    await expect(page.locator(".project-card")).to_have_count(2)
    await bvt.widths(page, "Projects", [
        ".project-hero", ".project-search", ".project-card", ".project-body h3",
        ".project-tags", ".project-card .project-actions",
    ])
    modal = await open_create(page)
    await add_image(modal, "cancelled-create")
    await modal.get_by_placeholder("Paste an image URL and press Enter or click Add", exact=True).fill(
        f"{IMAGE_ORIGIN}/unadded-create.png"
    )
    await modal.get_by_role("button", name="Cancel", exact=True).click()
    await expect(modal).not_to_be_visible()
    modal = await open_create(page)
    await expect(modal.locator(".n-upload-file")).to_have_count(0)
    await expect(modal.get_by_placeholder("Paste an image URL and press Enter or click Add", exact=True)).to_have_value("")
    await fill_create(page, modal)
    await add_image(modal, "create-removed")
    retained = await add_image(modal, "create-retained")
    await bvt.widths(page, "Create modal", [
        ".project-editor", ".project-editor .project-markdown-editor", ".project-editor .tags-select",
        ".project-editor .url-input-row", ".project-editor .editor-actions",
    ], ".project-editor")
    await remove_first_image(modal)
    images = [retained]
    for index in range(2, 7):
        images.append(await add_image(modal, f"create-{index}"))
    await assert_limit(page, modal)
    await submit_once(page, modal, fixtures, "Publish project", "/v1/project", images)
    await expect(page.locator(".project-card")).to_have_count(3)


async def edit_flow(bvt, page, fixtures):
    await page.goto(bvt.base + "/#/project?id=1")
    await expect(page.get_by_role("heading", name=TITLE, exact=True)).to_be_visible()
    await expect(page.get_by_role("button", name="Edit", exact=True)).to_be_visible()
    await expect(page.locator(".detail-tags .n-tag")).to_have_count(8)
    await expect(page.locator(".member-row")).to_have_count(2)
    await bvt.widths(page, "ProjectDetail", [
        ".detail-header", ".detail-heading", ".detail-heading h1", ".detail-tags", ".project-actions",
        ".detail-links", ".detail-gallery", ".detail-columns", ".side-card", ".member-row",
    ])
    modal = await open_edit(page)
    await add_image(modal, "cancelled-edit")
    await modal.get_by_placeholder("Paste an image URL and press Enter or click Add", exact=True).fill(
        f"{IMAGE_ORIGIN}/unadded-edit.png"
    )
    await modal.get_by_role("button", name="Cancel", exact=True).click()
    await expect(modal).not_to_be_visible()
    modal = await open_edit(page)
    await expect(modal.locator(".n-upload-file")).to_have_count(2)
    await expect(modal.get_by_placeholder("Paste an image URL and press Enter or click Add", exact=True)).to_have_value("")
    await bvt.widths(page, "Edit modal", [
        ".project-editor-modal", ".project-editor-modal .project-markdown-editor", ".project-editor-modal .tags-select",
        ".project-editor-modal .url-input-row", ".project-editor-modal .editor-actions",
    ], ".project-editor-modal")
    await remove_first_image(modal)
    images = [PROJECT["images"][1]]
    for index in range(2, 7):
        images.append(await add_image(modal, f"edit-{index}"))
    await assert_limit(page, modal)

    # An API rejection keeps the draft and permits a successful retry.
    fixtures.fail_next_update = True
    async with page.expect_response(lambda response: urlsplit(response.url).path == "/v1/project/update"):
        await modal.get_by_role("button", name="Save Changes", exact=True).click()
    await expect(modal).to_be_visible()
    await expect(modal.get_by_role("button", name="Cancel", exact=True)).to_be_enabled()
    await expect(modal.locator(".n-upload-file")).to_have_count(6)
    await expect(modal.get_by_placeholder("Give your project a clear name", exact=True)).to_have_value(TITLE)
    await submit_once(page, modal, fixtures, "Save Changes", "/v1/project/update", images)
    await expect(page.locator(".detail-gallery .n-image")).to_have_count(6)

    # Hold the old project's edit response across a hash-only route change.
    modal = await open_edit(page)
    delayed_title = "Delayed owner edit"
    await modal.get_by_placeholder("Give your project a clear name", exact=True).fill(delayed_title)
    fixtures.hold_writes()
    async with page.expect_response(lambda response: urlsplit(response.url).path == "/v1/project/update") as pending:
        await modal.get_by_role("button", name="Save Changes", exact=True).click()
        await asyncio.wait_for(fixtures.received.wait(), timeout=8)
        await page.evaluate("location.hash = '#/project?id=2'")
        await expect(page.get_by_role("heading", name=SECOND_TITLE, exact=True)).to_be_visible()
        await expect(modal).not_to_be_visible()
        fixtures.release_writes()
    await (await pending.value).finished()
    # Drain rendering after the completed response without an arbitrary sleep.
    await page.evaluate("new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))")
    await expect(page.get_by_role("heading", name=SECOND_TITLE, exact=True)).to_be_visible()
    await expect(page).to_have_url(re.compile(r"/#/project\?id=2$"))
    await expect(page.get_by_role("heading", name=delayed_title, exact=True)).to_have_count(0)
    await page.go_back()
    await expect(page.get_by_role("heading", name=delayed_title, exact=True)).to_be_visible()
    await expect(page).to_have_url(re.compile(r"/#/project\?id=1$"))
    await page.go_forward()
    await expect(page.get_by_role("heading", name=SECOND_TITLE, exact=True)).to_be_visible()
    await expect(page).to_have_url(re.compile(r"/#/project\?id=2$"))
    await expect(page.locator(".project-editor-modal")).not_to_be_visible()


async def permissions_flow(bvt, page, _fixtures):
    await page.goto(bvt.base + "/#/project?id=1")
    await expect(page.get_by_role("heading", name=TITLE, exact=True)).to_be_visible()
    await expect(page.get_by_role("button", name="Edit", exact=True)).to_have_count(0)


async def cover_flow(bvt, page, fixtures):
    for count in (0, 1, 2, 4, 6):
        images = [f"{IMAGE_ORIGIN}/cover-{i}.png" for i in range(count)]
        fixtures.projects[1]["images"] = images
        await page.goto(f"{fixtures.base}/#/projects")
        await page.reload()
        card = page.locator(".project-card").first
        await expect(card).to_be_visible()
        cover = card.locator(".project-gallery img")
        await expect(cover).to_have_count(1 if images else 0)
        if images:
            await expect(cover).to_have_attribute("src", images[0])
            await expect(cover).to_have_attribute("alt", TITLE)
        await expect(card.locator(".more-images")).to_have_count(0)
        await bvt.widths(page, f"cover-{count}", [".project-card", ".project-gallery"])
    # All six images remain available on the detail page.
    await page.goto(f"{fixtures.base}/#/project?id=1")
    await expect(page.locator(".project-detail")).to_be_visible()
    await expect(page.locator(".detail-gallery img")).to_have_count(6)


async def main():
    if not (WEB / "dist" / "index.html").is_file():
        raise SystemExit("Build web/dist first with npm run build.")
    OUTPUT.mkdir(parents=True, exist_ok=True)
    server = ThreadingHTTPServer(("127.0.0.1", 0), partial(StaticHandler, directory=str(WEB / "dist")))
    server.daemon_threads = True
    thread = Thread(target=server.serve_forever, daemon=True)
    thread.start()
    base = f"http://127.0.0.1:{server.server_port}"
    try:
        async with async_playwright() as playwright:
            # Use the official Playwright-installed Chromium, not a machine path.
            browser = await playwright.chromium.launch(headless=True)
            try:
                bvt = BVT(browser, base)
                await bvt.run_case("project-cover", lambda page, fixtures: cover_flow(bvt, page, fixtures))
                await bvt.run_case("projects-create", lambda page, fixtures: create_flow(bvt, page, fixtures))
                await bvt.run_case("project-detail-edit", lambda page, fixtures: edit_flow(bvt, page, fixtures))
                subjects = [
                    ("nonowner", {**OWNER, "id": 99, "username": "fixture-viewer"}),
                    ("without-post-create", {**OWNER, "permissions": [p for p in PERMISSIONS if p != "post.create"]}),
                    ("without-content-upload", {**OWNER, "permissions": [p for p in PERMISSIONS if p != "content.upload"]}),
                ]
                for name, user in subjects:
                    await bvt.run_case(name, lambda page, fixtures: permissions_flow(bvt, page, fixtures), user)
            finally:
                await browser.close()
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
    print("PASS Projects production browser BVT (fixture API; not backend integration)", flush=True)


if __name__ == "__main__":
    asyncio.run(main())
