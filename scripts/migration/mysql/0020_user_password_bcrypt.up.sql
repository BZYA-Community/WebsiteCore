-- 密码哈希由 md5(md5(pwd)+salt) 迁移到 bcrypt(60字符), 加宽 password 列
ALTER TABLE p_user MODIFY password VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'bcrypt密码';
