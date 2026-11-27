CREATE INDEX idx_post_feed ON p_post (is_del, audit_status, visibility, is_top DESC, latest_replied_on DESC);
