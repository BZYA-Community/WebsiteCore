"""Shared API fixtures for issue #94; use a disposable local database only.

Set E2E_OPERATOR_USERNAME, E2E_OPERATOR_PASSWORD and TEST_POSTGRES_DSN.
Requires psycopg[binary]. Accounts/passwords are generated per run. SQL is used
only to bind synthetic phone values, avoiding a real SMS provider in tests.
"""
import json
import http.client
import os
import secrets
import urllib.parse

import psycopg


class Fixture:
    def __init__(self):
        self.base = os.environ.get("E2E_BASE_URL", "http://127.0.0.1:8008")
        if urllib.parse.urlsplit(self.base).hostname not in ("127.0.0.1", "localhost", "::1"):
            raise ValueError("E2E fixtures require a local disposable instance")
        self.passed = self.failed = 0
        self.db = psycopg.connect(os.environ["TEST_POSTGRES_DSN"], autocommit=True)
        self.operator = {"username": os.environ["E2E_OPERATOR_USERNAME"], "password": os.environ["E2E_OPERATOR_PASSWORD"]}
        self.login(self.operator)
        self.operator["id"] = self.data("GET", "/v1/user/info", self.operator)["id"]
        self.phone(self.operator, True)

    def call(self, method, path, user=None, body=None, content_type=None):
        data = body if isinstance(body, bytes) else json.dumps(body).encode() if body is not None else None
        target = urllib.parse.urlsplit(self.base)
        connection_type = http.client.HTTPSConnection if target.scheme == "https" else http.client.HTTPConnection
        connection = connection_type(target.hostname, target.port, timeout=20)
        headers = {}
        if user:
            headers["Authorization"] = "Bearer " + user["token"]
        if data is not None:
            headers["Content-Type"] = content_type or "application/json"
        try:
            connection.request(method, target.path + path, body=data, headers=headers)
            response = connection.getresponse()
            try:
                return json.loads(response.read())
            except ValueError:
                return {"code": response.status}
        finally:
            connection.close()

    def data(self, method, path, user=None, body=None):
        result = self.call(method, path, user, body)
        if result.get("code") != 0:
            raise AssertionError(f"{method} {path}: {result.get('code')} {result.get('msg')}")
        return result.get("data") or {}

    def check(self, name, condition):
        if condition:
            self.passed += 1
            print("[PASS] " + name)
        else:
            self.failed += 1
            print("[FAIL] " + name)

    def expect(self, name, method, path, user=None, body=None, code=0):
        result = self.call(method, path, user, body)
        self.check(f"{name} (code={result.get('code')})", result.get("code") == code)
        return result.get("data") or {}

    def login(self, user):
        result = self.data("POST", "/v1/auth/login", body={"username": user["username"], "password": user["password"]})
        user["token"] = result["token"]
        return result

    def user(self, identity="student", auditor=False, mentor=False, phone=True):
        user = {"username": "t" + secrets.token_hex(5), "password": secrets.token_urlsafe(10)}
        user.update(self.data("POST", "/v1/auth/register", body={**user, "member_identity": "teacher", "roles": ["admin"], "is_mentor": True}))
        self.login(user)
        own = self.data("GET", "/v1/user/info", user)
        self.check("registration ignores identity/role injection", own["member_identity"] == "student" and own["roles"] == [] and not own["is_mentor"])
        if identity != "student" or auditor or mentor:
            self.access(user, identity, auditor, mentor)
        self.phone(user, phone)
        return user

    def access(self, user, identity, auditor=False, mentor=False, actor=None):
        return self.data("POST", "/v1/admin/user/access", actor or self.operator, {"user_id": user["id"], "member_identity": identity, "is_auditor": auditor, "is_mentor": mentor})

    def phone(self, user, bound):
        self.db.execute("UPDATE p_user SET phone=%s WHERE id=%s", ("test-bound-" + str(user["id"]) if bound else "", user["id"]))

    def admin(self):
        user = {"username": "a" + secrets.token_hex(5), "password": secrets.token_urlsafe(10)}
        user.update(self.data("POST", "/v1/admin/accounts", self.operator, {"username": user["username"], "temporary_password": user["password"]}))
        result = self.login(user)
        self.check("Admin first login requires password change", result["must_change_password"])
        self.expect("temporary password cannot manage users", "GET", "/v1/admin/user/list", user, code=10404)
        self.expect("temporary password cannot publish", "POST", "/v1/post", user, {}, code=10404)
        self.expect("temporary password cannot use loose write routes", "POST", "/v1/user/follow", user, {"user_id": self.operator["id"]}, code=10404)
        own = self.data("GET", "/v1/user/info", user)
        self.check("dedicated Admin has no member identity", own["member_identity"] is None and own["roles"] == ["admin"] and own["must_change_password"])
        new_password = secrets.token_urlsafe(10)
        self.data("POST", "/v1/user/password", user, {"old_password": user["password"], "password": new_password})
        self.expect("password change revokes old token", "GET", "/v1/user/info", user, code=10006)
        user["password"] = new_password
        self.login(user)
        self.phone(user, True)
        return user

    def upload(self, user, path, payload, filename, mime, extra_type=None):
        boundary = "test" + secrets.token_hex(12)
        body = b""
        if extra_type:
            body += f'--{boundary}\r\nContent-Disposition: form-data; name="type"\r\n\r\n{extra_type}\r\n'.encode()
        body += f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{filename}"\r\nContent-Type: {mime}\r\n\r\n'.encode()
        body += payload + f"\r\n--{boundary}--\r\n".encode()
        result = self.call("POST", path, user, body, "multipart/form-data; boundary=" + boundary)
        if result.get("code") != 0:
            raise AssertionError(f"upload: {result.get('code')} {result.get('msg')}")
        return result["data"]

    def course(self, teacher):
        group = self.data("POST", "/v1/admin/course/group", self.operator, {"name": "test" + secrets.token_hex(4), "sort": 1})
        video = self.upload(teacher, "/v1/admin/course/video", b"\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom", "test.mp4", "video/mp4")
        body = {"group_id": group["id"], "teacher_id": self.operator["id"], "title": "Identity permissions", "intro": "Test course", "video": video["video_url"], "cover": ""}
        course = self.data("POST", "/v1/admin/course", teacher, body)
        self.check("server sets course teacher to creator", course["teacher_id"] == teacher["id"])
        return course, {**body, "id": course["id"], "teacher_id": teacher["id"]}

    @staticmethod
    def content(text):
        return {"contents": [{"content": text, "type": 2, "sort": 100}], "tags": [], "users": [], "visibility": 0}

    def finish(self):
        self.db.close()
        print(f"PASS={self.passed} FAIL={self.failed}")
        return int(self.failed > 0)
