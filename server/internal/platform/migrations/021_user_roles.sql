-- 021_user_roles.sql — 多用户与角色权限
-- 设计依据：docs/27-多用户与权限控制方案.md
-- 背景：原「单一组织 + 单一管理员」模型，user 表无角色/状态字段，无法区分「谁能看、谁能改、谁能管」。
-- 说明：
--   ① user 新增 role（owner 所有者 / admin 高级用户 / viewer 一般用户）与 status（active/disabled 软停用）；
--   ② 现有账号（当前系统最多 1 个）一律升级为 owner，账本数据零改动；
--   ③ change_log 新增 operator（冗余用户名快照，非外键）：多用户后仍可追溯「谁改的」；
--      历史日志 operator 为空，前端按「（迁移前）」占位展示，不回填。

ALTER TABLE user ADD COLUMN role TEXT NOT NULL DEFAULT 'viewer'
    CHECK (role IN ('owner','admin','viewer'));
ALTER TABLE user ADD COLUMN status TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('active','disabled'));

-- 现有唯一管理员升级为所有者（注册制下单用户部署，回填安全）
UPDATE user SET role = 'owner' WHERE role = 'viewer';

ALTER TABLE change_log ADD COLUMN operator TEXT;
