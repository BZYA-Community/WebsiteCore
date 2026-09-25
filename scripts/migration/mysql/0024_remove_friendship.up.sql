-- 移除好友功能: 重建用户关系视图(仅保留关注), 删除好友关系表;
-- 存量好友可见(50)帖子转为私密(0); 清理好友申请消息(type=5)
DROP VIEW IF EXISTS `p_user_relation`;
CREATE VIEW `p_user_relation` AS
SELECT user_id, follow_id he_uid, 10 AS style
FROM `p_following` WHERE is_del=0;
DROP TABLE IF EXISTS `p_contact_group`;
DROP TABLE IF EXISTS `p_contact`;
UPDATE `p_post` SET visibility = 0 WHERE visibility = 50;
DELETE FROM `p_message` WHERE type = 5;
