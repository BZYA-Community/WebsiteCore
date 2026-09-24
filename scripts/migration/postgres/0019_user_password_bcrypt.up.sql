-- 密码哈希由 md5(md5(pwd)+salt) 迁移到 bcrypt(60字符), 加宽 password 列
ALTER TABLE p_user ALTER COLUMN password TYPE VARCHAR(255);
