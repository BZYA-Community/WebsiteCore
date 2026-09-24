DROP INDEX idx_message_receiver_type_read ON p_message;
DROP INDEX idx_message_sender_receiver ON p_message;

ALTER TABLE p_message MODIFY COLUMN content VARCHAR(255) NOT NULL DEFAULT '' COMMENT '详细内容';
