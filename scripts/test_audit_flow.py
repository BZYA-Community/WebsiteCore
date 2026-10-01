"""Issue #94 publication, profile, self-review and public identity regressions."""
import base64
from identity_test_client import Fixture


def main():
    f = Fixture()
    admin = f.admin()
    student, auditor = f.user(), f.user(auditor=True)
    teacher, mentor, teacher_auditor = f.user("teacher"), f.user("teacher", mentor=True), f.user("teacher", auditor=True)
    course, _ = f.course(teacher)
    root = f.data("POST", "/v1/post", teacher, f.content("public root"))
    root_comment = f.data("POST", "/v1/post/comment", teacher, {**f.content("root comment"), "post_id": root["id"]})
    root_course_comment = f.data("POST", "/v1/course/comment", teacher, {**f.content("root course comment"), "course_id": course["id"]})
    png = base64.b64decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=")
    pending_post = None
    for user, label, approved in [(student, "Student", 0), (auditor, "Student Auditor", 0), (teacher, "Teacher", 1), (mentor, "Mentor", 1), (teacher_auditor, "Teacher Auditor", 1), (admin, "Admin", 1), (f.operator, "Operator", 1)]:
        post = f.data("POST", "/v1/post", user, f.content(label + " post"))
        f.check(label + " publication policy", post["audit_status"] == approved)
        if user is auditor:
            pending_post = post
            f.expect("Auditor cannot review own post", "POST", "/v1/admin/audit/post", auditor, {"post_id": post["id"], "action": "approve"}, 20007)
        for path, parent, kind in [("/v1/post/comment", {"post_id": root["id"]}, 0), ("/v1/post/comment/reply", {"comment_id": root_comment["id"]}, 1), ("/v1/course/comment", {"course_id": course["id"]}, 2), ("/v1/course/comment/reply", {"comment_id": root_course_comment["id"]}, 3)]:
            body = {**f.content(label + " comment"), **parent, "content": label + " reply", "at_user_id": 0}
            comment = f.data("POST", path, user, body)
            f.check(label + " policy " + path, comment["audit_status"] == approved)
            if user is auditor:
                f.expect("Auditor cannot self-review comment kind " + str(kind), "POST", "/v1/admin/audit/comment", auditor, {"id": comment["id"], "comment_type": kind, "action": "approve"}, 20007)
        old = f.data("GET", "/v1/user/info", user)
        nickname = "new" + user["username"][-6:]
        f.data("POST", "/v1/user/nickname", user, {"nickname": nickname})
        current = f.data("GET", "/v1/user/info", user)
        f.check(label + " nickname policy", current["nickname"] == (nickname if approved else old["nickname"]))
        avatar = f.upload(user, "/v1/attachment", png, "avatar.png", "image/png", "public/avatar")["content"]
        result = f.data("POST", "/v1/user/avatar", user, {"avatar": avatar})
        f.check(label + " avatar policy", result["pending"] == (not approved))
        current = f.data("GET", "/v1/user/info", user)
        f.check(label + " avatar visibility", current["avatar"] == (avatar if approved else old["avatar"]))
        if user is auditor:
            for field in ["nickname", "avatar"]:
                f.expect("Auditor cannot self-review " + field, "POST", "/v1/admin/audit/" + field, auditor, {"user_id": auditor["id"], "action": "approve"}, 20007)
            f.check("self response exposes Auditor", current["roles"] == ["auditor"])
        public = f.data("GET", "/v1/user/profile?username=" + user["username"])
        f.check(label + " public response hides Auditor and legacy fields", "auditor" not in public["roles"] and "identity" not in public and "is_admin" not in public)
        f.check(label + " public Mentor designation", public["is_mentor"] == (user is mentor))
    f.access(auditor, "teacher", auditor=True)
    detail = f.data("GET", f"/v1/post?id={pending_post['id']}", auditor)
    f.check("identity promotion retains old pending submission", detail["audit_status"] == 0)
    f.expect("another Admin can approve pending submission", "POST", "/v1/admin/audit/post", admin, {"post_id": pending_post["id"], "action": "approve"})
    public = f.data("GET", f"/v1/post?id={pending_post['id']}")
    f.check("content author projection hides Auditor", public["user"]["roles"] == [] and public["user"]["member_identity"] == "teacher")
    f.access(auditor, "student", auditor=True)
    detail = f.data("GET", f"/v1/post?id={pending_post['id']}")
    f.check("identity downgrade keeps approved content public", detail["audit_status"] == 1)
    f.data("POST", "/v1/admin/course/delete", admin, {"id": course["id"]})
    return f.finish()


if __name__ == "__main__":
    raise SystemExit(main())
