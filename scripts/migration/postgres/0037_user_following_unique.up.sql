-- Block writes between deduplication and index creation.
LOCK TABLE p_following IN SHARE ROW EXCLUSIVE MODE;

DELETE FROM p_following AS newer
USING p_following AS older
WHERE newer.user_id = older.user_id
  AND newer.follow_id = older.follow_id
  AND newer.id > older.id
  AND newer.is_del = 0
  AND older.is_del = 0;

CREATE UNIQUE INDEX idx_following_user_follow_unique
    ON p_following (user_id, follow_id)
    WHERE is_del = 0;
