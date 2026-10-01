BEGIN;
LOCK TABLE p_user IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM p_user) THEN
        RAISE EXCEPTION 'Identity rollback requires an empty user table; no data migration is supported';
    END IF;
END $$;
DROP INDEX course_teacher;
DROP TABLE p_whisper_conversation;
DROP TRIGGER preserve_account_type ON p_user;
DROP FUNCTION p_preserve_account_type();
ALTER TABLE p_user DROP CONSTRAINT user_mentor_teacher;
ALTER TABLE p_user DROP CONSTRAINT user_identity_valid;
ALTER TABLE p_user DROP COLUMN must_change_password;
ALTER TABLE p_user DROP COLUMN is_mentor;
ALTER TABLE p_user DROP COLUMN member_identity;
ALTER TABLE p_user DROP COLUMN account_type;
ALTER TABLE p_user ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT FALSE;
COMMIT;
