"""Local-only HTTP upload regression. Requires a disposable server and operator.

Set TEST_OPERATOR_USERNAME/PASSWORD and optional TEST_API_BASE (localhost:18008).
Pass --large to stream a full 2 GiB resource; it remains in the test storage.
Uses only Python's standard library; never prints credentials or signed URLs.
"""
import argparse
import http.client
import json
import os
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid

BASE = os.getenv("TEST_API_BASE", "http://127.0.0.1:18008").rstrip("/")
parsed = urllib.parse.urlsplit(BASE)
if parsed.hostname not in {"localhost", "127.0.0.1", "::1"}:
    raise SystemExit("This destructive-fixture check only accepts a local test server")
CHECKS = []


def check(label, value):
    CHECKS.append({"name": label, "passed": bool(value)})
    print(("PASS " if value else "FAIL ") + label, flush=True)
    if not value:
        raise AssertionError(label)


def request(method, path, token=None, body=None, content_type="application/json", headers=None):
    data = json.dumps(body).encode() if isinstance(body, dict) else body
    req = urllib.request.Request(urllib.parse.urljoin(BASE + "/", path), method=method, data=data)
    if token:
        req.add_header("Authorization", "Bearer " + token)
    if data is not None:
        req.add_header("Content-Type", content_type)
    for key, value in (headers or {}).items():
        req.add_header(key, value)
    try:
        response = urllib.request.urlopen(req, timeout=60)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        return response.status, response.headers, response.read()


def api(label, method, path, token=None, body=None, success=True):
    status, _, raw = request(method, path, token, body)
    result = json.loads(raw)
    check(label, (result.get("code") == 0) == success and status < 500)
    return result.get("data")


def direct_put(path, size):
    """A bounded 1 MiB buffer, including for --large; no sparse-file shortcut."""
    target = urllib.parse.urlsplit(urllib.parse.urljoin(BASE, path))
    if target.netloc != parsed.netloc:
        raise AssertionError("Direct upload must stay on the local test server")
    conn = http.client.HTTPConnection(target.hostname, target.port, timeout=120)
    try:
        conn.putrequest("PUT", target.path + "?" + target.query)
        conn.putheader("Content-Type", "application/zip")
        conn.putheader("Content-Length", str(size))
        conn.endheaders()
        header = b"PK\x03\x04" + bytes(508)
        conn.send(header[:size])
        remaining = size - min(size, len(header))
        chunk = bytes(1 << 20)
        while remaining:
            count = min(remaining, len(chunk))
            conn.send(chunk[:count])
            remaining -= count
        response = conn.getresponse()
        response.read()
        return response.status
    finally:
        conn.close()


def normal_upload(token, size, *, name="ordinary.zip", mime="application/zip"):
    boundary = "WebsiteCoreTest" + uuid.uuid4().hex
    before = (f"--{boundary}\r\nContent-Disposition: form-data; name=\"type\"\r\n\r\nattachment\r\n"
              f"--{boundary}\r\nContent-Disposition: form-data; name=\"file\"; filename=\"{name}\"\r\n"
              f"Content-Type: {mime}\r\n\r\n").encode()
    data = before + b"PK\x03\x04" + bytes(size-4) + f"\r\n--{boundary}--\r\n".encode()
    status, _, raw = request("POST", "/v1/attachment", token, data, "multipart/form-data; boundary=" + boundary)
    result = json.loads(raw)
    return status < 500 and result.get("code") == 0


def main(large):
    token = api("Operator login", "POST", "/v1/auth/login", body={
        "username": os.environ["TEST_OPERATOR_USERNAME"],
        "password": os.environ["TEST_OPERATOR_PASSWORD"],
    })["token"]
    profile = api("Public module profile", "GET", "/v1/site/profile")
    check("Configured module enabled", profile["courses_enabled"])
    limits = profile["upload_limits"]
    check("Public profile omits signing TTL", "credential_ttl_seconds" not in limits)
    check("Ordinary attachment exact limit accepted", normal_upload(token, limits["attachment_max_bytes"]))
    check("Ordinary attachment one byte above limit denied", not normal_upload(token, limits["attachment_max_bytes"] + 1))
    check("Forged image signature denied", not normal_upload(token, 1024, name="forged.jpg", mime="image/jpeg"))
    for kind, maximum in [("attachment", limits["course_attachment_max_bytes"]), ("resource", limits["course_resource_max_bytes"])]:
        api(kind + " oversized init rejected", "POST", "/v1/admin/course/upload/init", token,
            {"name": "bound.zip", "kind": kind, "mime_type": "application/zip", "size": maximum + 1}, False)
    size = 2 << 30 if large else 2 << 20
    upload = api("Initialize resource", "POST", "/v1/admin/course/upload/init", token,
                 {"name": "stream.zip", "kind": "resource", "mime_type": "application/zip", "size": size})
    check("Local direct capability", upload["method"] == "PUT" and upload["mode"] == "direct")
    status, _, _ = request("PUT", upload["upload_url"] + "x", body=b"PK\x03\x04", content_type="application/zip")
    check("Forged signature denied", status == 403)
    status, _, _ = request("PUT", upload["upload_url"], body=b"PK\x03\x04", content_type="application/zip")
    check("Interrupted body not published", status == 400)
    started = time.monotonic()
    check("Full streamed resource accepted", direct_put(upload["upload_url"], size) == 204)
    print(f"Streamed {size} bytes in {time.monotonic() - started:.2f}s", flush=True)
    complete = {"attachment_id": upload["attachment_id"]}
    data = api("Complete verified resource", "POST", "/v1/admin/course/upload/complete", token, complete)
    check("Server records exact size", data["file_size"] == size)
    api("Completion retry is idempotent", "POST", "/v1/admin/course/upload/complete", token, complete)
    status, _, _ = request("PUT", upload["upload_url"], body=b"PK\x03\x04", content_type="application/zip")
    check("Old capability cannot replace resource", status == 400)
    suffix = uuid.uuid4().hex[:8]
    category = api("Create category", "POST", "/v1/admin/course/group", token, {"name": "Upload QA " + suffix})
    course = api("Create course without raw video", "POST", "/v1/admin/course", token,
                 {"group_id": category["id"], "title": "Stream QA " + suffix, "teacher_intro": "Synthetic QA"})
    lesson = api("Attach verified resource", "POST", "/v1/admin/course/lesson", token,
                 {"course_id": course["id"], "title": "First lesson", "attachments": [{"attachment_id": upload["attachment_id"], "name": "Resource.zip", "kind": "resource"}]})
    api("Anonymous lesson access denied", "GET", f"/v1/course/lessons?course_id={course['id']}", success=False)
    api("Course metadata update preserves lessons", "POST", "/v1/admin/course/update", token,
        {"id": course["id"], "group_id": category["id"], "title": "Edited QA " + suffix})
    lessons = api("Load stable lesson", "GET", f"/v1/course/lessons?course_id={course['id']}", token)["lessons"]
    check("Metadata save preserves attachment", len(lessons) == 1 and lessons[0]["id"] == lesson["id"] and len(lessons[0]["attachments"]) == 1)
    check("Lesson JSON contains no resource URL", "http" not in json.dumps(lessons))
    url = api("Issue download capability", "GET", f"/v1/course/attachment?id={lesson['attachments'][0]['id']}", token)["signed_url"]
    unsigned = urllib.parse.urlsplit(url)._replace(query="").geturl()
    status, _, _ = request("GET", unsigned)
    check("Unsigned resource denied", status == 403)
    status, _, _ = request("GET", unsigned.replace("/attachment/", "/Attachment/"))
    check("Windows case alias denied", status == 403)
    status, headers, raw = request("GET", url, headers={"Range": "bytes=0-511"})
    check("Signed range download", status == 206 and len(raw) == 512 and raw.startswith(b"PK\x03\x04"))
    check("Safe download headers", headers.get("X-Content-Type-Options") == "nosniff" and headers.get("Content-Disposition", "").startswith("attachment"))
    status, headers, _ = request("HEAD", url)
    check("Signed HEAD exact length", status == 200 and int(headers["Content-Length"]) == size)
    api("Remove test course", "POST", "/v1/admin/course/delete", token, {"id": course["id"]})
    api("Remove empty test category", "POST", "/v1/admin/course/group/delete", token, {"id": category["id"]})
    print(f"{len(CHECKS)} checks passed", flush=True)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--large", action="store_true")
    args = parser.parse_args()
    main(args.large)
