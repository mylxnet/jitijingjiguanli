# Changelog

## [Unreleased]

### 新增
- 后端基础设施：配置加载（env）、SQLite 连接（modernc 纯 Go 驱动）、版本化迁移机制
  （schema_migrations，事务内执行）
- 首期四张表：user / session / category（含 D6 余额字段与 CHECK 约束）/ txn
- 迁移与约束单元测试（TestMigrate / TestCategoryCheck）
- 002 迁移：change_log / app_setting / transfer / transfer_leg（D6/D7 数据层）
- 统一错误体与响应助手（platform.Fail / platform.OK / error.go）
- HTTP 依赖：gin v1.12.0（本机代理需 GOPROXY=goproxy.io）
- 登录与会话：bcrypt 口令、POST /api/auth/login·logout、HttpOnly Cookie、
  会话中间件（401）、Repo 分层（service 不再直写 SQL）
- 登录与会话单元测试（4 项：初始化幂等 / 登录会话 / 中间件 / 参数校验）

### 骨架
- 项目初始化（v0.1.0 起点）
