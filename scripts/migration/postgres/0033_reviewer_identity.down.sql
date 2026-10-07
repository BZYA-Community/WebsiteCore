DROP INDEX IF EXISTS idx_user_created_on_active;

-- Preserve assigned or customized policy when rolling back.
DELETE FROM p_identity_group AS g
WHERE g.key = 'reviewer' AND g.builtin = TRUE AND g.name = '审核员'
  AND g.description = 'Review assigned content; no user or identity administration.'
  AND NOT EXISTS (SELECT 1 FROM p_user_identity_group WHERE group_id = g.id)
  AND NOT EXISTS (SELECT 1 FROM p_identity_group_permission WHERE group_id = g.id AND permission <> 'content.review');
