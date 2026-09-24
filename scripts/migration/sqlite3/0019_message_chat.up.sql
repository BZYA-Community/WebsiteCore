-- 私信会话化: sqlite 不强制 VARCHAR 长度无需改列, 仅补复合索引(会话聚合/历史双向查询/未读分组)
CREATE INDEX idx_message_sender_receiver ON p_message (sender_user_id, receiver_user_id);
CREATE INDEX idx_message_receiver_type_read ON p_message (receiver_user_id, type, is_read);
