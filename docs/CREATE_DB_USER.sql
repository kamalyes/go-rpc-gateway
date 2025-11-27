-- ====================================
-- MySQL 用户创建脚本
-- 创建 idev 用户并授予权限
-- ====================================

-- 1. 创建用户 (密码包含特殊字符)
-- 注意: MySQL 8.0+ 默认使用 caching_sha2_password 认证插件
CREATE USER IF NOT EXISTS 'idev'@'%' IDENTIFIED BY 'iDev@2025#Secure!';

-- 如果需要使用旧版认证方式 (mysql_native_password)，使用以下命令:
-- CREATE USER IF NOT EXISTS 'idev'@'%' IDENTIFIED WITH mysql_native_password BY 'iDev@2025#Secure!';

-- 2. 授予数据库权限
-- 授予 engine_im_agent 数据库的所有权限
GRANT ALL PRIVILEGES ON engine_im_agent.* TO 'idev'@'%';

-- 或者只授予常用权限 (SELECT, INSERT, UPDATE, DELETE)
-- GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, DROP, INDEX, ALTER ON engine_im_agent.* TO 'idev'@'%';

-- 3. 刷新权限
FLUSH PRIVILEGES;

-- 4. 验证用户创建
SELECT user, host, plugin FROM mysql.user WHERE user = 'idev';

-- 5. 查看用户权限
SHOW GRANTS FOR 'idev'@'%';

-- ====================================
-- 使用说明:
-- ====================================
-- 1. 登录 MySQL: mysql -u root -p -h 120.77.38.35 -P 13306
-- 2. 执行脚本: source /path/to/create-mysql-user.sql
-- 或复制粘贴以上 SQL 语句执行
--
-- 3. 测试新用户连接:
--    mysql -u idev -p'iDev@2025#Secure!' -h 120.77.38.35 -P 13306 engine_im_agent
--
-- ====================================
-- 密码说明:
-- ====================================
-- 用户名: idev
-- 密码: iDev@2025#Secure!
-- 
-- 密码包含的特殊字符:
--   @ - At 符号
--   # - 井号
--   ! - 感叹号
--
-- ====================================
-- 如果需要删除用户重新创建:
-- ====================================
-- DROP USER IF EXISTS 'idev'@'%';
-- 然后重新执行上面的创建语句
--
-- ====================================
-- 配置文件更新 (gateway-im-dev.yaml):
-- ====================================
-- mysql:
--     host: 120.77.38.35
--     port: "13306"
--     username: idev
--     password: "iDev@2025#Secure!"
--     db-name: engine_im_agent
-- ====================================
