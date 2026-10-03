"""Issue #94 course ownership and dedicated-account API regression suite."""
from identity_test_client import Fixture


def main():
    f = Fixture()
    admin, student = f.admin(), f.user(auditor=True)
    teacher, other = f.user("teacher", mentor=True), f.user("teacher")
    mentor = f.user(mentor=True)
    eligible = [candidate for user in [teacher, other, admin, f.operator, student, mentor]
                for candidate in f.data("GET", "/v1/admin/course/teachers?keyword=" + user["username"], admin)["teachers"]]
    ids = {user["id"] for user in eligible}
    f.check("course owner selector includes Teacher and staff", {teacher["id"], other["id"], admin["id"], f.operator["id"]} <= ids)
    f.check("course owner selector excludes Student and Mentor-only", not ({student["id"], mentor["id"]} & ids))
    course, body = f.course(teacher)
    for actor, label in [(student, "Student Auditor"), (mentor, "Mentor-only")]:
        result = f.call("POST", "/v1/admin/course", actor, body)
        f.check(label + " cannot create course", result.get("code") in (20007, 20022))
    for actor, label in [(admin, "Admin"), (f.operator, "Operator")]:
        owned, _ = f.course(actor)
        f.check(label + " creates own unmoderated course", owned["teacher_id"] == actor["id"])
        f.expect(label + " removes own course", "POST", "/v1/admin/course/delete", actor, {"id": owned["id"]})
        f.expect(label + " removes empty group", "POST", "/v1/admin/course/group/delete", actor, {"id": owned["group_id"]})
    for actor in [teacher, student]:
        result = f.call("POST", "/v1/admin/course/group", actor, {"name": "forbidden"})
        f.check("only management can create groups", result.get("code") in (20007, 20022))
    f.expect("Teacher edits own course", "POST", "/v1/admin/course/update", teacher, body)
    f.expect("Teacher cannot edit other course", "POST", "/v1/admin/course/update", other, body, 20007)
    f.expect("Teacher cannot transfer own course", "POST", "/v1/admin/course/update", teacher, {**body, "teacher_id": other["id"]}, 20007)
    f.expect("Teacher cannot delete course", "POST", "/v1/admin/course/delete", teacher, {"id": course["id"]}, 20007)
    f.expect("Admin edits any course", "POST", "/v1/admin/course/update", admin, body)
    for identity in [student, mentor]:
        f.expect("transfer rejects member without Teacher access", "POST", "/v1/admin/course/update", admin, {**body, "teacher_id": identity["id"]}, 70005)
    for target in [admin, f.operator]:
        f.expect("management can receive course transfer", "POST", "/v1/admin/course/update", admin, {**body, "teacher_id": target["id"]})
    f.expect("transfer back to Teacher", "POST", "/v1/admin/course/update", admin, body)
    f.expect("course owner cannot lose Teacher access", "POST", "/v1/admin/user/access", admin, {"user_id": teacher["id"], "member_identity": "student"}, 11008)
    f.expect("course owner cannot be deleted", "POST", "/v1/admin/user/delete", admin, {"id": teacher["id"]}, 11008)
    f.expect("Teacher can always be banned", "POST", "/v1/admin/user/status", admin, {"id": teacher["id"], "status": 2})
    f.expect("banned token cannot edit", "POST", "/v1/admin/course/update", teacher, body, 20006)
    detail = f.data("GET", f"/v1/course?id={course['id']}")
    f.check("banned Teacher course stays public", detail["course"]["id"] == course["id"])
    f.expect("Admin edits banned Teacher course", "POST", "/v1/admin/course/update", admin, body)
    f.expect("Admin transfers banned Teacher course", "POST", "/v1/admin/course/update", admin, {**body, "teacher_id": other["id"]})
    f.expect("cannot transfer back to banned Teacher", "POST", "/v1/admin/course/update", admin, body, 70005)
    f.access(teacher, "student", mentor=True, actor=admin)
    retained = f.data("GET", f"/v1/admin/user/detail?id={teacher['id']}", admin)
    f.check("Teacher removal retains independent Mentor", retained["member_identity"] == "student" and retained["is_mentor"])
    f.expect("released Teacher can be deleted", "POST", "/v1/admin/user/delete", admin, {"id": teacher["id"]})
    f.expect("Admin deletes course", "POST", "/v1/admin/course/delete", admin, {"id": course["id"]})
    f.expect("deleted course absent", "GET", f"/v1/course?id={course['id']}", code=70003)
    f.expect("empty group can be removed", "POST", "/v1/admin/course/group/delete", admin, {"id": course["group_id"]})
    f.expect("Admin cannot create another Admin", "POST", "/v1/admin/accounts", admin, {"username": "forbidden", "temporary_password": student["password"]}, 20007)
    f.expect("Admin cannot stop itself", "POST", "/v1/admin/user/status", admin, {"id": admin["id"], "status": 2}, 20007)
    f.expect("Admin cannot remove its own role", "POST", "/v1/admin/user/role", admin, {"user_id": admin["id"], "role": "admin", "action": "remove"}, 20007)
    f.expect("Admin cannot stop Operator", "POST", "/v1/admin/user/status", admin, {"id": f.operator["id"], "status": 2}, 20007)
    other_admin = f.admin()
    def check_admin_state(status, roles):
        current = f.data("GET", f"/v1/admin/user/detail?id={other_admin['id']}", admin)
        f.check("managed Admin state unchanged after denied operation", current["status"] == status and current["roles"] == roles)

    f.expect("Admin cannot stop another Admin", "POST", "/v1/admin/user/status", admin, {"id": other_admin["id"], "status": 2}, 20007)
    f.expect("Admin cannot restore another active Admin", "POST", "/v1/admin/user/status", admin, {"id": other_admin["id"], "status": 1}, 20007)
    f.expect("Admin cannot remove another Admin role", "POST", "/v1/admin/user/role", admin, {"user_id": other_admin["id"], "role": "admin", "action": "remove"}, 20007)
    check_admin_state(1, ["admin"])
    f.expect("Operator can stop another Admin", "POST", "/v1/admin/user/status", f.operator, {"id": other_admin["id"], "status": 2})
    f.expect("stopped Admin token denied immediately", "GET", "/v1/user/info", other_admin, code=20006)
    f.expect("Admin cannot restore stopped Admin", "POST", "/v1/admin/user/status", admin, {"id": other_admin["id"], "status": 1}, 20007)
    f.expect("Admin cannot revoke stopped Admin", "POST", "/v1/admin/user/role", admin, {"user_id": other_admin["id"], "role": "admin", "action": "remove"}, 20007)
    check_admin_state(2, ["admin"])
    f.expect("Operator can restore another Admin", "POST", "/v1/admin/user/status", f.operator, {"id": other_admin["id"], "status": 1})
    f.expect("Operator can remove another Admin role", "POST", "/v1/admin/user/role", f.operator, {"user_id": other_admin["id"], "role": "admin", "action": "remove"})
    f.expect("revoked Admin token denied immediately", "GET", "/v1/user/info", other_admin, code=20006)
    f.expect("Admin cannot restore revoked dedicated account", "POST", "/v1/admin/user/status", admin, {"id": other_admin["id"], "status": 1}, 20007)
    check_admin_state(2, [])
    f.expect("Operator can restore revoked dedicated account", "POST", "/v1/admin/user/status", f.operator, {"id": other_admin["id"], "status": 1})
    f.expect("Operator removes Admin role", "POST", "/v1/admin/user/role", f.operator, {"user_id": admin["id"], "role": "admin", "action": "remove"})
    f.expect("removed Admin token denied immediately", "GET", "/v1/user/info", admin, code=20006)
    f.expect("Operator restores Admin", "POST", "/v1/admin/user/status", f.operator, {"id": admin["id"], "status": 1})
    own = f.data("GET", "/v1/user/info", admin)
    f.check("restored Admin remains dedicated", own["member_identity"] is None and own["roles"] == ["admin"])
    f.expect("Admin cannot convert to member", "POST", "/v1/admin/user/access", f.operator, {"user_id": admin["id"], "member_identity": "teacher"}, 20007)
    f.expect("member cannot become management account", "POST", "/v1/admin/user/role", f.operator, {"user_id": student["id"], "role": "admin", "action": "add"}, 11003)
    f.expect("Operator can view system information", "GET", "/v1/admin/site/status", f.operator)
    f.expect("Admin cannot view system information", "GET", "/v1/admin/site/status", admin, code=20007)
    for actor in [student, mentor, other]:
        f.expect("member cannot view system information", "GET", "/v1/admin/site/status", actor, code=20022)
    return f.finish()


if __name__ == "__main__":
    raise SystemExit(main())
