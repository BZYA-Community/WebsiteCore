BEGIN;
LOCK TABLE p_user IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM p_user) THEN
        RAISE EXCEPTION 'Identity upgrade requires an empty user table, including deleted accounts; no data migration is supported';
    END IF;
END $$;

ALTER TABLE p_user DROP COLUMN is_admin;
ALTER TABLE p_user ADD COLUMN account_type VARCHAR(16) NOT NULL DEFAULT 'member';
ALTER TABLE p_user ADD COLUMN member_identity VARCHAR(16) DEFAULT 'student';
ALTER TABLE p_user ADD COLUMN is_mentor BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE p_user ADD COLUMN must_change_password BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE p_user ADD CONSTRAINT user_identity_valid CHECK (
    (account_type = 'member' AND member_identity IS NOT NULL
        AND member_identity IN ('student', 'teacher') AND roles IN ('', 'auditor'))
    OR (account_type = 'operator' AND member_identity IS NULL AND roles = 'operator')
    OR (account_type = 'admin' AND member_identity IS NULL
        AND (roles = 'admin' OR (roles = '' AND status = 2)))
);
ALTER TABLE p_user ADD CONSTRAINT user_mentor_member CHECK (
    NOT is_mentor OR account_type = 'member'
);
CREATE UNIQUE INDEX single_operator_account ON p_user (account_type) WHERE account_type = 'operator';
CREATE FUNCTION p_preserve_account_type() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.account_type <> OLD.account_type THEN
        RAISE EXCEPTION 'Account types cannot be converted';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER preserve_account_type BEFORE UPDATE OF account_type ON p_user
    FOR EACH ROW EXECUTE FUNCTION p_preserve_account_type();

CREATE TABLE p_whisper_conversation (
    low_user_id BIGINT NOT NULL REFERENCES p_user(id),
    high_user_id BIGINT NOT NULL REFERENCES p_user(id),
    pending_sender_id BIGINT,
    established BOOLEAN NOT NULL DEFAULT FALSE,
    admin_contacted BOOLEAN NOT NULL DEFAULT FALSE,
    blocked_by_low BOOLEAN NOT NULL DEFAULT FALSE,
    blocked_by_high BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (low_user_id, high_user_id),
    CHECK (low_user_id < high_user_id),
    CHECK (pending_sender_id IS NULL OR pending_sender_id IN (low_user_id, high_user_id)),
    CHECK (NOT established OR pending_sender_id IS NULL),
    CHECK (NOT (blocked_by_low OR blocked_by_high) OR pending_sender_id IS NULL)
);
CREATE INDEX whisper_conversation_high_user ON p_whisper_conversation(high_user_id);
CREATE INDEX course_teacher ON p_course(teacher_id) WHERE is_del = 0;
COMMIT;
