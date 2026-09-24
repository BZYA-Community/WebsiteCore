DROP INDEX IF EXISTS idx_message_receiver_type_read;
DROP INDEX IF EXISTS idx_message_sender_receiver;

ALTER TABLE p_message ALTER COLUMN content TYPE VARCHAR(255);
