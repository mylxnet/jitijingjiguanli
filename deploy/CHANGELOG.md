# Changelog

> 版本号唯一来源：`web/src/version.ts` 的 `APP_VERSION`（`README.md` 顶部「当前版本」同步）。
> ⚠️ 本文件最后一条历史记录为 `[0.6.0]`（`0.6.1` 未单独记录），`0.7.0 ~ 0.22.0` 的明细以 `git log`、GitHub Releases 与 `docs/` 各版本设计文档为准；
> 其中可核对的里程碑：v0.7 投资管理+532 分配、v0.8 全局记账弹窗、v0.12 引导页 Excel 导入建账（统一东八区/备份约定）、
> v0.13 历年欠款分年度管理与整额跨单核销、v0.14 再投资收益闭环、v0.15 引导页服务端权威标记、v0.16 往来单位数据缺失提醒、
> v0.17 移动端 tab 改「投资/流转」、v0.18 登录页去预填凭据、v0.19 坏账核销+往来单位删除+快速记账新模板、
> v0.20 合同到期管理与归档、v0.21 看板环形构成改造+全站配色归一与浅色化、v0.22 全站配色 token 化改造与对比度达标(WCAG AA)。
> 自 v0.23 起恢复逐版记录。

## [0.24.1]

### 修复
- **换镜像/升级后「点 tab 没反应」（手机端往来、合同等懒加载页）**：单二进制内嵌前端此前对**未命中的静态产物**一律回退 `index.html`，浏览器把 HTML 当 JS 模块加载会被严格 MIME 校验拒绝 → 懒加载路由的 `import()` 抛错 → Vue Router **静默中止跳转**，界面零提示、零日志。现改为：
  - `/assets/*` 未命中 → **`404 asset not found`**（不再回退 HTML）；
  - `index.html`（含 SPA 路由回退）→ `Cache-Control: no-cache`，换版本后旧 HTML 不再长期留存；
  - 带内容哈希的产物 → `Cache-Control: public, max-age=31536000, immutable`；
  - 其余非 `/api/` 路径仍回退 `index.html`（hash 路由不受影响）。
  - 回归测试 `server/cmd/server/main_test.go`（`TestStaticAsset404AndCacheHeaders`）。
- **手机端合同 PDF 打不开 / 白屏**：合同预览此前把 `data:application/pdf;base64,…` 直接塞进 `<iframe>`，而移动端内核不在 iframe 里渲染 PDF（安卓 Chrome 无内置 PDF 查看器、iOS Safari 只在顶层导航用 QuickLook、微信内直接白屏）。改为 **pdf.js 逐页画 canvas**（弹窗内可滚动、显示总页数），并给所有预览类型统一生成 **blob URL** 供 iframe 与「⬇ 下载」使用（绕开超长 data URL 被移动 WebView 拦截），渲染失败时保留下载入口而不是关掉弹窗。

### 新增
- 前端依赖 `pdfjs-dist@4.10.38`。按需**懒加载**：`pdf-*.js` 365 KB（仅打开 PDF 预览时下载）+ worker `pdf.worker.min-*.mjs` 1.37 MB；worker 经 Vite `?url` 打包为同源产物（不走 CDN），实测服务端下发 `text/javascript`，与上一条 MIME 口径配套。

### 遗留
- 真机（安卓 / 微信 WebView）渲染仍**未实机复核**：本机 QA 只有桌面 Chromium，且该窗口处于后台时 `requestAnimationFrame` 不触发、pdf.js 会无声卡死，自测需注入 rAF 垫片 —— 能证明逻辑正确，不等于实机通过。
- 「版本漂移自愈」（拉不到入口 chunk 时提示刷新并自动重载一次）本次**未做**。
- 详见 `docs/26-静态资源缓存与手机端合同PDF预览方案.md`。

## [0.24.0]

### 变更（功能口径）
- **投资收益拆成两类**：投资收益 = 长期投资收益 + 再投资收益，此前页面只有长期一项。
  - 新增预置一级科目「再投资收益」（迁移 `020_reinvest_income_cat.sql`，`preset=1`、`sort=6`），预置科目由 **8 个一级 → 9 个一级**、共 **18 项**；注册即生成。
  - `reinvest_dividend`（再投资分红）的现金入账容器由「再投资」改指「再投资收益」，**再投资容器从此只承载本金**，不再被收益虚增。
  - 投资收益页签改为 5 张卡（已收收益合计 / 已收长期投资收益 / 已收再投资收益 / 已分配 / 未分配余额）+ 按往来单位聚合的 4 列明细 + 对账提示行；CSV 同步为 4 列；表头「年收益」正名「年预期收益」。
  - **532 分配基数换口径**：由「只算长期投资已收」改为「长期 + 再投资两类已收合计」。`已收` 是展示与分配的唯一权威口径，收益科目余额只作对账（差额提示"直接入账、未挂应收单的收益不计入可分配"）。
  - 快速记账「收到投资收益/分红」按所选单位的应收类型自动落到 `投资收益/<单位>` 或 `再投资收益/<单位>`；往来单位删除的科目作用域（`partyCategoryL1`）同步纳入新容器。
  - ⚠️ **迁移会回填**：把历史上误挂在 `再投资/<单位>` 下的收益二级迁到 `再投资收益/<单位>`，**已生成分配方案不回改**（其为 `total_income_cents` 快照），但换口径后**往年「未分配余额」数字会上升**。属纠错，上线前请先只读核查并备份库文件。
  - 详见 `docs/25-投资收益两类收益口径方案.md`。

### 新增
- 看板「银行存款流水」弹窗新增**摘要**列（取 `txn.note`，空显示 `—`，悬停看全文）；窄屏改为表格横向滚动（最小宽 560px），摘要列允许换行。

### 修复
- **可支出构成 · 公益支出出现负数**（如 `¥-1.00`）：分配额此前只取「最近一个年度」的 532 `welfare_cents`，而已支出是历年累计，跨年发放/改过同年方案即把该项算成负。改为**历年累计** `SUM(welfare_cents)`，两侧同区间。回归用例 `TestCompositionWelfareCrossYear`（旧实现实测得 `-20000`，新实现 `80000`）。
- **环形构成图渲染失效**：分项含负数时占比分母用净值，导致正项 `frac > 1`、`stroke-dasharray` 出现负值（SVG 规范判为非法，浏览器忽略整条）→ 环形画成一整圈纯色。改为扇区只由正分项构成、分母取正分项之和；负分项不画扇区但保留在图例与合计中。
- **环形中心合计与图例对不上**：`formatShort` 的「千」档把 `¥5,999.00` 四舍五入显示成 `6.0千`。改为 1 万元以内一律显示准确数（≥1 万仍用「x.x万」），并给中心数字加 `title` 显示精确金额。

### 影响面
- 后端：迁移 1 个、`receivable`（入账容器映射与删除作用域）、`summary`（公益额度口径）；前端：`InvestmentsPage.vue`、`RecordPopup.vue`、`Home.vue`、`types/api.ts`。
- 预置科目数变化会同时影响 mock 与 JS 契约测试（`server/mock.js` 加 `backfillPresetL1()` 以兼容已持久化的旧快照）。
- ⚠️ 迁移只在**进程启动时**执行一次：换镜像后必须重启容器才会跑 020。

### 已知未修（本版本不含）
- 「银行存款流水」缺资金划转（对外投资/收回）行，且未过滤 `voided` 流水 → 表末余额与首页银行存款卡可能对不上。
- 再投资页签金额仍按**同名挂接**取科目余额，且对负本金取 `Math.abs`（本金被绝对值洗白）；根治需给 `category` 加 `party_id`。
- 快速记账「收到投资收益/分红」在该单位**无未结清应收单**时会静默回落成普通收入记账（不进「已收」、不进可分配池），界面无区分提示。

## [0.23.1]

### 修复
- **汇总页下钻跳转失败**：`web/src/features/summary/SummaryPage.vue` 的 `goListWithFilter()` 把 query 写成
  `from: range.from, to: range.to`，而该组件不存在 `range` 变量 —— 点任意一条流水即在 `router.push` 前抛
  `ReferenceError: range is not defined`，跳转与筛选全部失效。改为使用函数内已算好的整年局部常量
  `from`/`to`（`${年}-01-01` ~ `${年}-12-31`），与函数注释「带该科目 + 该年筛选」的原意一致。
  ⚠️ 该 bug 已随 0.23.0 进入 Releases 产物与 ACR 镜像，**线上必须升到本版本才算修掉**。
- 前端类型检查 8 处报错全部清零（`npx vue-tsc --noEmit` → 0），涉及 5 个文件：
  `ContractsPage.vue`（mammoth `ArrayBufferLike` 入参、`expiresAt` 可空性）、`ReceivablesList.vue`
  （`van-tag` 的 `type` 收紧为 `TagProps['type']`）、`InvestmentsPage.vue`（本地 `Category` 缺 `kind`、
  `/categories` 响应泛型）、`OperationLogDialog.vue`（`/operation-logs` 响应泛型）。均为类型层收口，运行时行为不变。

### 工具链
- `cd web && npm run build`（= `vue-tsc -b && vite build`）此前因这 8 处错误长期必失败，**现恢复可用**，
  本地验收改回以它为准；CI 的 `npx vite build`（跳过类型检查）保留为遗留收紧项。

### 影响面
- 5 个前端文件，+14 / −8；接口契约、数据模型、迁移文件均未变（`schema_migrations` 仍 19 个迁移）。
- 逐条根因、修法与实机验收证据见 `docs/22-前端待修类型错误清单.md` §8。

## [0.23.0]

### 修复
- 密码输错时前端显示「请先登录」：`web/src/lib/http.ts` 的 401 拦截不再无差别当作会话失效——
  `/auth/login`、`/auth/register`、`/auth/reset-password` 列入公开认证路径，其 401/400 按服务端
  `{error:{code,message}}` 原文显示；需要登录的接口（如 `/api/me`）仍走「请先登录 + 跳登录页」。
- 登录/注册页错误提示撑高卡片导致按钮上下跳（实测登录页 96px）、整页可滚 96px（`.login-page`/`.register-page`
  与 `App.vue` 的 `.auth-wrap` 重复声明 `min-height:100vh` + `padding:24px`）。

### 优化
- 认证页错误提示改为与输入框**同一行右侧**（Vant `van-field` 的 `#extra` 插槽 + `:show-error-message="false"`
  + `van-form @failed` 取失败字段名），行高继承 `.van-cell`，出现/消失零位移；提示色用 token `--danger-deep`。
- 注册页提示按语义落点：`USERNAME_TAKEN` → 账号行；`REGISTRATION_CLOSED` 及其他异常 → 组织名称行。
- 输入即清提示（`watch(..., {flush:'sync'})`，同步刷新以免「先清空密码再写提示」被异步队列抹掉）。

### 影响面
- 纯前端显示层 + http 封装判定，接口契约、数据模型、迁移文件均未变（`schema_migrations` 仍 19 个迁移）。
- 详细方案与实测数据见 `docs/23-v0.23-认证页错误提示与会话拦截修复.md`。

## [0.6.0]

### 新增
- 科目期初录入（迁移 007）：二级科目可填期初余额（权益=期初+收入-支出+转入-转出；
  资产=期初+投资-收回），一级分组不可填，负数拒绝
- 快速记账业务模板改造：投资给公司=支出记入「长期投资/公司」并自动建同名往来单位（投资公司）；
  收回投资=收入记入「上级补助」；新增「支出管理费」（科目：管理费支出）
- 登录/品牌：登录页大字=本机记住的组织名、小字=集体经济管理系统（/api/me 返回 orgName）
- 桌面端左侧导航：新增「记账」（跳看板开记一笔）、「退出」；删除顶部品牌文字；设置/退出带图标
- 看板新增「最新流水」卡片
- 往来重构（大模块）：
  - 单位类型化（迁移 008）：流转企业 flow / 投资公司 invest / 其它单位 other；
    投资公司「投资金额」只读=长期投资/公司同名累计投出（不可手改）
  - 单位资料扩展（迁移 009）：联系电话、流转面积（亩，流转企业）
  - 新增/编辑单位弹窗：类型下拉、按类型输入年度标准/分红、电话、面积
  - 单位详情改底部弹窗（蓝色主色），展示全部资料 + 应收记录 + 就地修改年度标准
  - 年度结转 = 预览再确认：/api/recv-standards/preview 从单位标准带数据（流转费/投资收益），
    标注「将新增/已存在跳过」，确认才生成分年度应收
  - 应收作废（未收款可作废重结，PUT /api/receivables/:id/void）
  - 往来页三 banner：欠款总计 / 流转费欠款 / 投资收益欠款（点击按类型筛选全部企业）
  - 往来导出：单位基本情况、欠款明细（/api/export?content=parties|receivables）
  - 登记应收不再预设「收款入账科目」，现金收款时再选
- 预置科目树定稿：本金=上级补助；长期投资（空）；经营收入=投资收益|土地流转费收入|流转管理费|其他收入；
  分配与支出=土地流转费-转付农户|成员分红|福利发放|公益支出|管理费支出（迁移同步 008/009 应用）

### 调整
- 「资金划转/资产类资金池」弃用（前端入口移除；后端 fund_move 保留兼容旧数据）
- 原「土地流转服务费收入」预置改名「流转管理费」；「对外投资」组改名「长期投资」
- 站点视觉：往来页蓝色主色；单位详情弹窗分区卡片布局

### 迁移
- 007 科目期初 / 008 单位类型 / 009 联系电话+流转面积

## [0.5.0]

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
- 导出（export，F5/D5/D8 落点）：GET /api/export?content=transactions|summary|balance_sheet&format=csv|xlsx，
  复用 summary.GetSummary 与 transaction.List 口径；CSV 带 UTF-8 BOM；
  xlsx 免加工格式（标题合并、表头冻结、列宽、金额右对齐 + #,##0.00、一级行加粗、合计上边框）、表尾资金构成三行
- 导出单元测试（6 组：流水 CSV/含作废/汇总 CSV/科目余额表 CSV（D8）/xlsx 重开+格式与右对齐/参数校验）
- summary 修复：一级科目行补汇总子项期初余额（此前恒 0，致 D8 余额表一级合计错）

### v0.4.1 桌面端适配 + 快速记账（web）
- 双端壳修复：仅带导航页面用 .app-shell 左右布局；登录/注册全宽居中
- 桌面去手机感：按钮/单元格/字号放大；底部弹层统一居中弹窗；全部 is-link 选择改原生下拉（新增 NativeSelect，手机保留原弹层）
- 流水页头/行按钮加大；编辑与筛选科目、Home 记一笔科目原生下拉
- 记一笔：新增「普通记账/快速记账」切换（按钮各半全宽）；快速记账 10 业务模板自动入账
  （补助/收益/流转费冲欠款/服务费/利息/转付农户/532成员/532公益/投资/收回投资）
- 提交 3a7e05c

### v0.4 模型重构：资产/权益 + 到账算收益 + 看板 + 批量计提（用户逐案确认后定稿）
- **迁移 006（破坏性重置）**：科目二级类型 = 资产 asset / 权益 equity；删除余额类型(余粮/花费)、参与勾稽、科目期初、
  “专项资金/未分配”口径；新增 `recv_standard`（计提标准，单位+类别唯一）；receivable 增 recv_year
- **记账口径**：到账才算收益；收入=银行+权益科目+、支出=银行+权益科目−、资金划转=银行↔资产；
  往来应收欠款单列“待收”（不计收益/资产）；532 仅按比例记账（无方案台账）
- **后端**：category/summary/transfer/export/auth 预置科目(4一级+9二级) 全面适配 asset/equity；
  汇总资金构成 = 银行存款 / 资产类合计 / 权益类合计(净资产)；预置示例：本金-上级补助、经营收入-投资收益/流转费/服务费/其他、
  分配与支出-转付农户/分红/福利/公益
- **批量计提（D11 升级）**：POST /api/receivables/batch（多单位列表一次生成，同年同类防重，year 过滤）；
  GET/POST /api/recv-standards、PUT /api/recv-standards/:id、POST /api/recv-standards/accrue（标准一键结转年度）
- **首页看板化**：第一 tab“记账”=看板（银行/资产类/待收欠款/净资产四卡、本年收益到账、欠款明细Top、
  底部悬浮“记一笔”弹层）；5 tab 不变；汇总页资金四卡改 银行/资产类/净资产/总资产
- **双端适配**：<992px 底部 5 tab；≥992px 左侧 SideNav 侧栏 + 内容自适应
- 验证：后端 12 包测试全绿（新增 accrue 测试）；vue-tsc + vite build；真实浏览器冒烟——
  注册演示村 → 预置 v0.4 科目 → 记投资收益1000 → 看板银行/净资产/本年收益=1000 → 甲公司流转费标准500 →
  一键结转2026 → 看板待收500、欠款明细“甲公司·流转费·2026年度土地流转费”且不计收益
- ⚠️ 破坏性升级：旧库业务数据作废；正式使用前重建空库注册

### v0.3.7 需求调整：往来单位化 + 科目单位快速命名 + 汇总点科目看流水
- **往来只单位（去农户）**：迁移 005 移除 party.kind（历史行按单位）；party model/repo/handler/测试全部去 kind；
  往来页不再有农户/单位筛选与类型选择，文案统一「往来单位」；测试库旧数据（张三等）自动按单位口径继续可用
- **新增二级科目选往来单位**：名称输入框右侧「选择往来单位 ▾」下拉（数据取 GET /parties，仅单位），
  点选后自动把单位名填入科目名称，仍可手动修改；无单位时提示先到往来页新增
- **汇总页点二级科目看流水**：二级行可点 → 底部弹层，默认当前月、可用左右箭头切换年/月；
  列出该科目当月流水（日期/收支/金额/摘要，作废置灰），底部当月收/支/结余（仅 normal 计入）；
  点某行跳转流水页并自动带「该科目+该月」筛选（List 支持路由 query 初始化筛选）
- 验证：全量 go vet+test 12 包绿、vue-tsc + vite build 通过；真实浏览器冒烟——
  往来只新增单位祥云合作社 → 新增二级科目用下拉自动填入「祥云合作社」→ 汇总点土地流转费收入弹层显示
  2026-09 的 +300 收款与合计 → 点行跳流水页自动筛选

### v0.3.6 数据守护：备份恢复（F7）+ 修改密码 + 变更历史查看（F6）
- **配置**：新增 APP_BACKUP_DIR（默认 ./backups）、APP_KEEP_BACKUP（默认 30）、APP_AUTO_BACKUP（默认 1）
- **备份后端**（internal/backup 新包）：GET /api/backups、POST /api/backups（VACUUM INTO 一致性快照 + 保留策略清理）、
  POST /api/backups/:id/restore（热恢复：独占锁 → 旧库暂存 → 备份覆盖 → 重开迁移 → 重建 router；失败自动回滚旧库）；
  每日 03:00 自动备份 goroutine（本机时区）；恢复后强制清 Cookie 重新登录
- **修改密码**：PUT /api/auth/password（鉴权）校验原口令 + bcrypt 更新；错误码 OLD_PASSWORD_WRONG / INVALID_PASSWORD
- **设置页**：新增「备份与恢复」区（立即备份/列表带时间大小/恢复带确认与全屏遮罩，恢复成功后登出回登录页）、
  「修改密码」弹层（原/新/确认，服务端错误红字保留输入）
- **变更历史查看（F6）**：通用 ChangeLogDialog 组件（create/update/void/unvoid + 字段中文映射）；
  接入科目页资金划转记录「留痕」（流水/转账查看此前已在流水页编辑弹层提供）
- **测试**：backup 2 组（备份→改数据→恢复→数据回退；保留策略只留 N 份）、auth 增 TestChangePassword
- 验证：全量 go vet+test 12 包绿；live API 冒烟（备份→建对象→恢复→对象回退/旧会话 401/重登正常；改密旧密码失效新密码可用）；
  真实浏览器冒烟（设置页备份列表/立即备份刷新、修改密码错误提示）
- ⚠️ 恢复语义：恢复的是整库一致性快照（含全部组织），不是单组织；恢复后所有会话失效需重新登录

### v0.3.5 前端：5 tab 导航 + 注册页 + 往来页（D11/F8/F9 界面化）
- **底部导航 4 → 5 tab**：记账 / 流水 / 汇总 / **往来** / 设置（BottomNav 新增 /contacts 高亮映射，科目管理仍归设置）
- **注册页**（/register）：组织名 + 账号 + 密码 + 确认，注册成功即自动登录回首页（auth store 增 markLoggedIn）；
  登录页底部加「注册组织」入口；App 导航对 /register 隐藏
- **往来页**（/contacts，新 ContactsPage）：对象列表（名称/类别/欠款合计/全部·农户·单位筛选 + 新增/编辑对象）→
  对象详情（合计欠款、应收单：类别标签、应收/已收/未收、未结清/已结清）；
  操作：登记应收（事由/金额/类别/可选收款入账科目）、收款/抵销弹层
  （现金入账自动记银行收入、预设科目自动带出；抵销需选一笔分红支出流水）、核销记录展开查看 + 作废
- **预置科目展示**：科目管理一/二级行加「预置」徽标（注册即生成的五件套一目了然）
- **修复**：往来详情页合计欠款未随应收/核销刷新（openDetail 未用最新对象数据回填）——浏览器冒烟发现并修复
- 验证：vue-tsc + vite build 通过；真实浏览器全链路冒烟——
  注册页注册「新庄」自动登录 → 5 tab 可见 → 往来新增张三(农户) → 登记应收 500（预设土地流转费收入科目）→
  现金收款 300 → 未收 200/欠款 200 → 核销记录 +300 → 汇总页银行存款 300、经营收入 300（自动入账闭环）
- ⚠️ data/ 现含三个冒烟组织（甲村/丙村/新庄），正式使用前建议删除该目录重建空库再注册

### v0.3.4 应收/往来后端（D11 落地；迁移 004 补 receipt.status）
- **迁移 004**：003 建表遗漏 receipt.status（设计 §9.1 有、建表 SQL 漏）→ ADD COLUMN status DEFAULT 'normal'（历史行视为正常）
- **往来对象 party**（internal/receivable 新包）：GET/POST /api/parties、PUT /api/parties/:id；列表带**欠款合计**
  （Σ 各 open 应收单未核销余额，SQL 相关子查询）；kind=household|unit + keyword 过滤；更新留痕
- **应收单 receivable**：POST /api/receivables（对象/类别 rent|dividend|other/事由/金额/可选预设入账科目，入账科目须为启用中普通二级）、
  GET /api/receivables（partyId/kind/status 过滤 + 已收/未收统计）、GET /api/receivables/:id 详情（含核销记录）
- **核销 receipt**：POST /api/receivables/:id/receipts
  - cash：单事务写 receipt + 自动生成银行**收入**流水（入账科目 = 本次指定优先 → 应收单预设 → 400 报错）；
    txn 备注「核销应收 #id 事由」
  - offset：单事务写 receipt 关联一条**发放支出**流水（须 expense+normal+本组织），不产生现金流水，支出流水保留
  - 超收拒绝（事务内兜底）、结清自动置 closed、closed 后禁止再核销；创建留痕（receipt/txn/receivable 状态）
- **作废核销**：PUT /api/receipts/:id → 状态 voided；cash 核销**连带作废**其收入流水（留痕 void）；
  应收单若因此未结清自动退回 open；offset 核销作废不影响支出流水
- **org 隔离**：所有查询/更新按会话组织过滤，跨组织 404/不可见
- **测试**（5 组）：D11 验收（欠 500 → 收 300 → 收 200 结清 → 银行两笔收入流水 → 再收拒绝 → 对象欠款 0）、
  抵销流程（分红支出保留、无现金流水、结清）、核销校验矩阵 9 反例、作废（cash 连带作废收入流水/offset 保留支出流水/应收单退回 open）、
  列表与详情（欠款聚合、kind/status 过滤、已收未收、跨组织不可见）
- 全量 go vet + go test 11 包绿

### v0.3.3 资产科目 + 资金划转（D10 落地，迁移 003 表已就绪）
- **资金划转后端**（internal/fundmove 新包）：POST /api/fund-moves（投资 invest=银行→资产 / 收回 recover=资产→银行，单行原子写 + 留痕 create）、
  GET /api/fund-moves（from/to/kind 过滤 + 分页）、PUT /api/fund-moves/:id（作废/撤销 + void/unvoid 留痕）；已挂入 main.go 鉴权组
- **校验**：金额>0、日期合法、资产科目须为本组织启用中的资产型二级（普通科目/一级/停用均拒）；
  投资 ≤ 当前银行存款（银行期初 + Σ收支 ± 已生效划转）；收回 ≤ 该资产科目在外金额（防资产余额为负）
- **口径对齐**：银行净额复用 category.Repo.BankDelta、资产余额 AssetBalance（迁移 003 已建聚合），summary 资金构成四卡恒等式自动吸收
- **删除保护**：category.Repo 增 CountFundMoves，资产科目已有资金划转记录时 DELETE 拒绝 409（D0/D10）
- **测试**（fundmove 包 5 组）：D10 场景验算（拨款→投资→收益→收回→作废回滚，资金构成逐项断言）、
  创建校验矩阵 10 反例、作废/撤销原子回滚 + 留痕计数、列表（排序/类型/日期/跨组织不可见）、
  科目删除保护；全量 go vet + go test 10 包绿
- **前端**：类型契约补 kind/preset/FundMove；记账（Home）、流水筛选/编辑（List）、科目间转账（Category）
  的科目下拉一律只列普通二级（R12 资产科目不进收支/转账）；科目页二级对话框新增「资产」类型开关
  （资产固定余粮型、无期初、不勾稽，编辑时仅可改名）；二级行资产显示「资产」标签与「在外」余额
- **科目页资金划转弹层**：投资/收回单选、日期、资产科目下拉（仅资产二级，带一级前缀与在外余额）、金额、摘要、
  保存；下方「划转记录」列表（方向/科目/±金额/日期/状态）支持作废；保存/作废后科目树余额实时刷新
- **汇总页**：资金构成由三卡改四卡（银行存款 / 投资资产 / 专项资金 / 未分配资金），科目余额树资产二级带「资产」标签
- 验证：vue-tsc + vite build 通过；真实浏览器（Edge CDP）冒烟通过——
  登录 → 科目页建资产二级 → 资金划转投资 500 → 树「在外」与记录即时更新 → 汇总四卡恒等式成立
- ⚠️ 冒烟数据说明：git-bash curl 直接发中文会按 GBK 编码入库产生乱码（仅本机冒烟库，非产品缺陷；
  浏览器/表单均以 UTF-8 提交，正常）。含乱码的旧冒烟库已移出工作区（见 v0.3.5 注，data/ 已重建为 UTF-8 测试库）

### v0.3 多组织改造（迁移 003 + 全业务 org 隔离）
- **迁移 003**：全业务表重建带 org_id（user/category/txn/transfer/leg/change_log/app_setting），
  category 新增 kind(normal/asset) 与 preset，新建 org/fund_move/party/receivable/receipt；
  app_setting 主键改 (org_id,key)；旧单组织数据作废（破坏性升级，需求决策）
- **注册制**（F8）：POST /api/auth/register（组织名+账号+密码）单事务建组织+账号+**预置科目**
  （本金/对外投资/经营收入[土地流转费收入·投资分红收益·其他收入]/收益分配[收益分红发放·流转费分发·
  福利发放]/公益支出，共 11 项）；废除环境变量初始账号引导
- **org 会话与隔离**（F9）：登录会话 Resolve 返回 user+org，RequireAuth 注入 orgID；category/txn/
  transfer/settings/changelog/summary/export 全部按会话组织过滤；Update/Delete 带 org 防御；
  git 中新增跨组织隔离测试（不可见/不可改/不可删 404）
- **资产科目打底**（D10 前置）：category.kind 校验（资产仅二级/余粮/禁勾稽/无期初）、txn/transfer
  拒绝资产科目；repo 增 AssetBalance/BankDelta；summary 资金构成口径升级为
  (银行存款+资产合计)−专项资金，Capital 增 assetTotalCents
- 测试：注册（预置 11 项/重名/短密码/双组织独立）、隔离、全量 seed 适配 org；
  全库 9 包 vet+test 绿；冒烟：甲乙两村注册各自得到预置科目树

### 前端（web，Vue 3 + TS + Vite）
- 页面 6 个（设计 P1-P6 对齐）：Login / Home 记账 / List 流水 / Summary 汇总 /
  Category 科目管理（含 D7 转账表单）/ Settings 设置；底部导航 4 tab + /categories 高亮归属设置
- 基建：hash 路由 + 守卫、Pinia auth store、Vant 全量、http 封装（Cookie 认证/401 拦截/统一错误）
- types/api.ts 前端契约（Category/Transaction/ApiResponse + 金额分↔元工具）
- tsconfig 治理：根配置 noEmit + node 配置去 composite（消除 tsc 副产物污染 src）
- 验证：vue-tsc 类型检查通过 + vite build 成功（5.4s，页面级分包）
- ⚠️ 版本偏离记录：package.json 实装 vite ^6.3.0→6.4.3（设计定为 8.2.1，v0.1.0 首验未受阻即未升级，待用户拍板）
- 导出入口接入后端 /api/export：新增 lib/download.ts（fetch blob + Content-Disposition 文件名 + 401 跳登录）；
  流水页导出改为后端生成（原前端组表逻辑移除，xlsx 库不再打进包——List chunk 301KB→16.9KB）；
  汇总页新增「导出」按钮（收支汇总×当前月 / 科目余额表，Excel+CSV）；转账视图下导出给提示
- 前端验证：vue-tsc + vite build 通过；端到端 curl 导出的 xlsx 经 zip 校验结构完整
- 登录链路两处修复（浏览器实测通过）：
  ① http.ts：dev 模式 BASE_URL 原为绝对 http://localhost:8080/api（跨端口=跨源，后端无 CORS →
     浏览器一律 Failed to fetch）；改统一同源相对 /api，dev 经 vite proxy 转发
  ② auth store：login 检查响应 res.data.ok，但后端登录返回 {data:{user,expiresAt}} 无 ok 字段 →
     登录实际成功但判定失败、路由守卫弹回登录页；改为判断 res.data.user
- 科目创建体验修复：新增一级科目对话框原显示「期初余额/余额类型」输入，但一级科目是分组容器
  （后端拒绝一级设期初）→ 填了就 400 且 toast 一闪而过像「没保存」；现一级对话框只留名称并
  提示「期初在二级上设置」，保存失败改持久对话框（不再错过原因）
- 浏览器实测（Edge headless CDP）：建一级 → 建二级（余粮/勾稽/期初30000）→ 列表显示余额正确

### 骨架
- 项目初始化（v0.1.0 起点）
