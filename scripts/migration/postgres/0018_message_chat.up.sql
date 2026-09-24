-- 私信会话化: 内容列扩容(单条上限500字留余量) + 复合索引(会话聚合/历史双向查询/未读分组)
ALTER TABLE p_message ALTER COLUMN content TYPE VARCHAR(1000);

CREATE INDEX idx_message_sender_receiver ON p_message USING btree (sender_user_id, receiver_user_id);
CREATE INDEX idx_message_receiver_type_read ON p_message USING btree (receiver_user_id, type, is_read);
