-- 私信会话化: 内容列扩容(单条上限500字留余量) + 复合索引(会话聚合/历史双向查询/未读分组)
ALTER TABLE p_message MODIFY COLUMN content VARCHAR(1000) NOT NULL DEFAULT '' COMMENT '详细内容';

CREATE INDEX idx_message_sender_receiver ON p_message (sender_user_id, receiver_user_id);
CREATE INDEX idx_message_receiver_type_read ON p_message (receiver_user_id, type, is_read);
