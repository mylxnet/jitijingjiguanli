# 集体台账

> 集体经济组织内部收支台账。单人、移动优先、局域网自托管。

## 本地启动

（待 v0.1.0 补充）

## 目录说明

| 目录 | 放什么 | 不放什么 |
|------|--------|----------|
| web/src/features/ | 按业务特性组织的前端代码 | 跨特性复用的通用组件 |
| web/src/components/ | 无业务含义的通用 UI 组件 | 含业务逻辑的组件 |
| server/internal/ | 后端 handler / service / repo | 配置与迁移 |
| server/migrations/ | 版本化 SQL 迁移 | 手工 SQL |
| deploy/ | Dockerfile / compose / env 样例 | 真实密钥 |
