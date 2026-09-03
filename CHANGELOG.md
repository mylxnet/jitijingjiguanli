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
- 导出（export，F5/D5/D8 落点）：GET /api/export?content=transactions|summary|balance_sheet&format=csv|xlsx，
  复用 summary.GetSummary 与 transaction.List 口径；CSV 带 UTF-8 BOM；
  xlsx 免加工格式（标题合并、表头冻结、列宽、金额右对齐 + #,##0.00、一级行加粗、合计上边框）、表尾资金构成三行
- 导出单元测试（6 组：流水 CSV/含作废/汇总 CSV/科目余额表 CSV（D8）/xlsx 重开+格式与右对齐/参数校验）
- summary 修复：一级科目行补汇总子项期初余额（此前恒 0，致 D8 余额表一级合计错）

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
