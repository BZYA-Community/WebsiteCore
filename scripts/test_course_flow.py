"""Issue #94 course ownership and dedicated-account API regression suite."""
from identity_test_client import Fixture


def main():
    f = Fixture()
    admin, student = f.admin(), f.user(auditor=True)
    teacher, other = f.user("teacher", mentor=True), f.user("teacher")
    course, body = f.course(teacher)
    for actor, label in [(student, "Student Auditor"), (admin, "Admin"), (f.operator, "Operator")]:
        result = f.call("POST", "/v1/admin/course", actor, body)
        f.check(label + " cannot create course", result.get("code") in (20007, 20022))
    for actor in [teacher, student]:
        result = f.call("POST", "/v1/admin/course/group", actor, {"name": "forbidden"})
        f.check("only management can create groups", result.get("code") in (20007, 20022))
    f.expect("Teacher edits own course", "POST", "/v1/admin/course/update", teacher, body)
    f.expect("Teacher cannot edit other course", "POST", "/v1/admin/course/update", other, body, 20007)
    f.expect("Teacher cannot transfer own course", "POST", "/v1/admin/course/update", teacher, {**body, "teacher_id": other["id"]}, 20007)
    f.expect("Teacher cannot delete course", "POST", "/v1/admin/course/delete", teacher, {"id": course["id"]}, 20007)
    f.expect("Admin edits any course", "POST", "/v1/admin/course/update", admin, body)
    for identity in [student, admin]:
        f.expect("transfer rejects non-Teacher", "POST", "/v1/admin/course/update", admin, {**body, "teacher_id": identity["id"]}, 70005)
    f.expect("Mentor/course owner cannot be demoted", "POST", "/v1/admin/user/access", admin, {"user_id": teacher["id"], "member_identity": "student"}, 11008)
    f.expect("Mentor/course owner cannot be deleted", "POST", "/v1/admin/user/delete", admin, {"id": teacher["id"]}, 11008)
    f.expect("Teacher can always be banned", "POST", "/v1/admin/user/status", admin, {"id": teacher["id"], "status": 2})
    f.expect("banned token cannot edit", "POST", "/v1/admin/course/update", teacher, body, 20006)
    detail = f.data("GET", f"/v1/course?id={course['id']}")
    f.check("banned Teacher course stays public", detail["course"]["id"] == course["id"])
    f.expect("Admin edits banned Teacher course", "POST", "/v1/admin/course/update", admin, body)
    f.expect("Admin transfers banned Teacher course", "POST", "/v1/admin/course/update", admin, {**body, "teacher_id": other["id"]})
    f.expect("cannot transfer back to banned Teacher", "POST", "/v1/admin/course/update", admin, body, 70005)
    f.expect("Mentor must be cancelled separately", "POST", "/v1/admin/user/access", admin, {"user_id": teacher["id"], "member_identity": "student", "is_mentor": False}, 11008)
    f.access(teacher, "teacher", actor=admin)
    f.access(teacher, "student", actor=admin)
    f.expect("released Teacher can be deleted", "POST", "/v1/admin/user/delete", admin, {"id": teacher["id"]})
    f.expect("Admin deletes course", "POST", "/v1/admin/course/delete", admin, {"id": course["id"]})
    f.expect("deleted course absent", "GET", f"/v1/course?id={course['id']}", code=70003)
    f.expect("empty group can be removed", "POST", "/v1/admin/course/group/delete", admin, {"id": course["group_id"]})
    f.expect("Admin cannot create another Admin", "POST", "/v1/admin/accounts", admin, {"username": "forbidden", "temporary_password": student["password"]}, 20007)
    f.expect("Admin cannot stop Admin", "POST", "/v1/admin/user/status", admin, {"id": admin["id"], "status": 2}, 20007)
    f.expect("Operator removes Admin role", "POST", "/v1/admin/user/role", f.operator, {"user_id": admin["id"], "role": "admin", "action": "remove"})
    f.expect("removed Admin token denied immediately", "GET", "/v1/user/info", admin, code=20006)
    f.expect("Operator restores Admin", "POST", "/v1/admin/user/status", f.operator, {"id": admin["id"], "status": 1})
    own = f.data("GET", "/v1/user/info", admin)
    f.check("restored Admin remains dedicated", own["member_identity"] is None and own["roles"] == ["admin"])
    f.expect("Admin cannot convert to member", "POST", "/v1/admin/user/access", f.operator, {"user_id": admin["id"], "member_identity": "teacher"}, 20007)
    f.expect("member cannot become management account", "POST", "/v1/admin/user/role", f.operator, {"user_id": student["id"], "role": "admin", "action": "add"}, 11003)
    return f.finish()


if __name__ == "__main__":
    raise SystemExit(main())
