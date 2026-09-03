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
- 科目管理（category）：树形查询+实时余额、创建/更新/删除保护校验、更新留痕（change_log）、
  Register 路由；一级科目限制为分组容器（禁期初/勾稽），二级父科目校验
- 科目管理单元测试（5 组：建树余额 / 创建校验 / 留痕 / 删除保护 / D6-D7 余额公式）
- 流水管理（transaction）：记一笔/列表筛选/编辑/作废·撤销；Update 补齐校验（金额/日期/
  方向/状态/科目活性）避免裸撞 DB CHECK；GetSummary 与列表 includeVoided 口径一致；
  void/unvoid 留痕动作区分；NewHandler(db) 统一构造 + Register
- 流水单元测试（4 组：建单汇总搜索 / 创建校验 / 作废撤销与留痕 / 更新校验）
- 汇总查询（summary）：D6 资金构成（含停用勾稽科目）、科目余额树（一级=子项之和、
  区间发生额）、区间收支小计；余额口径统一复用 category.Repo.CalcBalance（消除重复实现）
- 系统配置（settings）：银行存款期初余额读写；Get 清理为单次带转换查询
- 汇总与配置单元测试（summary 3 组：D6 恒等式/负未分配警告/区间口径；settings 1 组：读写覆盖与负值拒绝）
- 科目间转账（transfer，D7）：1 转出 → N 转入单事务原子创建；R2 Σ转入=转出 / R3 转出≤余额 /
  R4 花费型禁作转入方（允许作转出方结转清零）校验；作废·撤销（余额实时聚合自动回滚）、
  列表两段式加载 legs（消除 N+1 与静默吞错）；余额口径复用 category.CalcBalance；
  NewHandler(db) + Register
- 转账单元测试（4 组：D7 五步验算 / 创建校验矩阵 12 反例 / 作废撤销原子回滚+留痕 / 列表 legs+排序+过滤）
- HTTP 全量接线（main.go）：category/transaction/summary/transfer/settings/changelog 六包
  Register 挂入鉴权组（空 base group + RequireAuth，保留各包 /api 绝对路径）；changelog 补 Register
  （GET /api/changelog）与 NewHandler(db) 统一构造
- 冒烟修复：空科目树返回 [] 而非 null（buildTree 归一），补空态回归测试
- 端到端冒烟通过：health → 401 → 登录 → 建科目 → summary（临时实例 18080）

### 前端（web，Vue 3 + TS + Vite）
- 页面 6 个（设计 P1-P6 对齐）：Login / Home 记账 / List 流水 / Summary 汇总 /
  Category 科目管理（含 D7 转账表单）/ Settings 设置；底部导航 4 tab + /categories 高亮归属设置
- 基建：hash 路由 + 守卫、Pinia auth store、Vant 全量、http 封装（Cookie 认证/401 拦截/统一错误）
- types/api.ts 前端契约（Category/Transaction/ApiResponse + 金额分↔元工具）
- tsconfig 治理：根配置 noEmit + node 配置去 composite（消除 tsc 副产物污染 src）
- 验证：vue-tsc 类型检查通过 + vite build 成功（5.4s，页面级分包）
- ⚠️ 版本偏离记录：package.json 实装 vite ^6.3.0→6.4.3（设计定为 8.2.1，v0.1.0 首验未受阻即未升级，待用户拍板）

### 骨架
- 项目初始化（v0.1.0 起点）
