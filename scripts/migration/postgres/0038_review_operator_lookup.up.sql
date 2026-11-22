CREATE INDEX idx_user_active_operator ON p_user (id) WHERE is_operator AND is_del = 0;
