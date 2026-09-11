# 往来单位（Party）设计方案与技术逻辑

> 版本：v1.0 · 覆盖往来单位从建库、联动建科目、应收关联到前端展示的完整链路。
> 关联文档：[06-linkage-rules.md](./06-linkage-rules.md)、[08-onboarding-design.md](./08-onboarding-design.md)

---

## 1. 概述与目标

往来单位（Party）是"集体台账"中与合作方（流转企业、投资公司等）打交道的核心业务对象，围绕它有两条主线：

1. **科目联动**：新建一个单位时，系统按单位类型自动在其对应"容器 L1"下创建同名二级科目，作为后续记账/入账的落点。
2. **应收关联**：单位的年度计提应收（土地流转费/投资收益/管理费）都以该单位为维度生成；收款统一走"快速记账"通道，自动入账到该单位的同名收入二级科目并核销应收。

设计约束（硬性）：
- 单位名称、类型**不可编辑**；单位**可有条件删除**：仅当「**无欠款**（名下应收单全部结清，含坏账核销）且**名下同名二级科目余额为 0**」时可删（`DELETE /api/parties/:id`）。删除时**历史流水不删除**，改挂到「历史归档 / 已删除单位」科目，原科目真删。
- 同一类型下**不允许重名**（精确校验拒绝 + 模糊相近提示），不同类型允许同名。
- 单位一经创建即联动建科目，联动在事务内完成，保证一致性。

---

## 2. 概念模型：单位类型

单位类型定义于 [model.go#L34-L37](file:///e:/traework/jizhang/server/internal/receivable/model.go#L34-L37)：

| type 值 | 含义 | 联动容器 L1 |
|---|---|---|
| `flow` | 流转企业 | 土地流转费收入、流转管理费 |
| `invest` | 投资公司 | 长期投资 |
| `reinvest` | 再投资单位 | 再投资 |
| `other` | 其它单位 | 无联动（不自动建科目） |

- 类型映射表 `typeToL1` 见 [repo.go#L29-L33](file:///e:/traework/jizhang/server/internal/receivable/repo.go#L29-L33)。
- 历史存量单位默认 `flow`（迁移 [008_party_type.sql](file:///e:/traework/jizhang/server/internal/platform/migrations/008_party_type.sql)），即老数据按流转企业口径。
- 前端按类型展示动态列（见 §7）：`flow` 显示流转面积、隐藏投资金额；`invest/reinvest` 显示投资金额、隐藏流转面积。

---

## 3. 数据模型（表结构）

基础表 `party` 字段由多版迁移逐步累加而来：

| 迁移 | 新增列 | 说明 |
|---|---|---|
| `001_init.sql` | 基础字段 `id, org_id, name, note, created_at, updated_at` | 基础单位（早期含 `kind`） |
| `005_party_unit_only.sql` | 移除 `kind` | 需求调整：往来对象只有"单位"，去掉农户/单位歧义 |
| `008_party_type.sql` | `type` | 单位类型化，默认 `flow` |
| `009_party_contact.sql` | `contact_phone, area_mu` | 联系电话、流转面积（亩） |
| `011_party_annual.sql` | `invest_amount_cents, return_rate_bps, expected_return_cents, land_mu, land_fee_per_mu_cents, expected_land_fee_cents, mgmt_fee_per_mu_cents, expected_mgmt_fee_cents` | **年度数据落库**（此前仅声明于结构体、保存即丢，本迁移修复） |

### 3.1 年度数据字段口径

| 字段 | 单位 | 说明 |
|---|---|---|
| `invest_amount_cents` | 分 | 投资本金 |
| `return_rate_bps` | 基点 | 年收益率；500 = 5.00%（前端输入按 % 换算：输入 3 = 300bps = 3%） |
| `expected_return_cents` | 分 | 年收益（投资本金 × 收益率） |
| `land_mu` | 亩 | 流转亩数 |
| `land_fee_per_mu_cents` | 分/亩 | 每亩年流转费 |
| `expected_land_fee_cents` | 分 | 总流转费（亩 × 每亩流转费，可改） |
| `mgmt_fee_per_mu_cents` | 分/亩 | 每亩年管理费 |
| `expected_mgmt_fee_cents` | 分 | 总管理费（亩 × 每亩管理费，可改） |

其中"金额×比例"类字段为**默认值联动**，允许用户手动覆盖：
- 投资类：`expected_return = invest_amount × (rate/10000)`
- 流转类：`expected_land_fee = land_mu × land_fee_per_mu`
- 管理费：`expected_mgmt_fee = land_mu × mgmt_fee_per_mu`

> 注意：`land_mu` / `area_mu` 语义相关但字段不同——`area_mu`（009，流转面积）用于列表展示；`land_mu`（011，流转亩数）用于年度流转费计算。二者在流转类联动的"亩"含义上有重合，梳理时按各自字段名区分。

---

## 4. 后端接口

路由包：`/api/parties`（走鉴权 `authed` 分组）。处理器见 [handler.go#L86-L188](file:///e:/traework/jizhang/server/internal/receivable/handler.go#L86-L188)。

### 4.1 `POST /api/parties` — 新建单位

请求体（`CreatePartyRequest` 兼容 `type` / `types` 数组）：
```json
{ "name": "流转验证社C", "type": "flow", "types": ["flow"],
  "contactPhone": "", "areaMu": 320, "note": "...",
  "investAmountCents": 0, "returnRateBps": 0, "expectedReturnCents": 0,
  "landMu": 320, "landFeePerMuCents": 0, "expectedLandFeeCents": 0,
  "mgmtFeePerMuCents": 0, "expectedMgmtFeeCents": 0 }
```

校验流程（[handler.go#L107-L184](file:///e:/traework/jizhang/server/internal/receivable/handler.go#L107-L184)）：
1. 名称 trim 后非空，否则 `INVALID_REQUEST`。
2. 类型：`types[0]` 优先于 `type`；为空默认 `flow`；非合法类型 → `INVALID_REQUEST`。
3. `areaMu < 0` → `INVALID_REQUEST`。
4. **重名校验**（`repo.FindDuplicate`，同类型口径）：
   - 精确同名 → `409 DUPLICATE_NAME` "该类型下已存在同名单位"；
   - 模糊相近 → `409 DUPLICATE_NAME`，`Details` 携带候选，前端提示改名称。
5. 入库 + 事务内联动建同名二级科目（见 §6），成功日志 `changelog` 记 `party` create，返回 `Party` 对象。

### 4.2 `GET /api/parties?keyword=` — 单位列表

按 `org_id` 查询，`keyword` 可选名称模糊过滤。返回 `Party[]`。

### 4.3 `PUT /api/parties/:id` — 更新单位

更新逻辑（[handler.go#L188-L300](file:///e:/traework/jizhang/server/internal/receivable/handler.go#L188-L300)）：
- 先查归属（`FindPartyByID`，`org_id` 不匹配 → `404 PARTY_NOT_FOUND`）。
- **名称/类型不可编辑**：请求携带的名称/类型与现值不同 → `NAME_IMMUTABLE` / `TYPE_IMMUTABLE`（仅允许同名同值，忽略不报）。前端编辑态这两个字段置灰只读且不提交。
- 可更新：`contact_phone`、`area_mu`（<0 拒绝）、`note`（空置 NULL）、以及年度数据 8 项（任一非 nil 则覆盖）。
- 更新后同步更新 `updated_at`。

### 4.4 接口清单（鉴权分组内）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/parties` | 列表（可关键词） |
| POST | `/api/parties` | 新建（自动建二级科目） |
| PUT | `/api/parties/:id` | 更新（名称/类型不可改，可改联系方式/面积/备注/年度数据） |
| DELETE | `/api/parties/:id` | **有条件删除**（无欠款 + 科目余额 0；历史流水归档「历史归档」；否则 409 + 原因码） |

---

## 5. 错误码汇总

| Code | HTTP | 触发条件 |
|---|---|---|
| `INVALID_REQUEST` | 400 | 参数缺失/类型非法/面积负数 |
| `DUPLICATE_NAME` | 409 | 同类型精确重名；或存在相近重名（Details=候选） |
| `NAME_IMMUTABLE` | 400 | 尝试修改单位名称 |
| `TYPE_IMMUTABLE` | 400 | 尝试修改单位类型 |
| `PARTY_NOT_FOUND` | 404 | 单位不存在或非本组织 |

---

## 6. 联动规则：自动创建同名二级科目

新建单位在 `CreateParty` 的事务内联动（[repo.go#L36-L95](file:///e:/traework/jizhang/server/internal/receivable/repo.go#L36-L95)）：

```
INSERT party ...
for l1name in typeToL1[type]:            # flow→[土地流转费收入,流转管理费]；invest→[长期投资]；reinvest→[再投资]
    l1 = SELECT id FROM category
         WHERE org_id AND name=l1name AND level=1 AND status='active'
    if l1 不存在 → continue               # 容器未预置/停用则跳过
    if 已存在同名 L2 (name=party.name, parent=l1id) → continue  # 幂等
    INSERT category(name=party.name, level=2, parent=l1id, status='active', kind='equity', preset=0)
commit
```

关键点：
- **事务内完成**：建单位与建科目同生共死，避免半成品。
- **幂等**：同名 L2 已存在不重复创建（引导页重复录入、并发场景安全）。
- **按需跳过**：容器 L1 不存在时不硬报错（保护历史/停用容器）。
- **收账联动**：现金收款时若应收单无预设入账科目、也未在本次指定，按 `rec_kind → L1` 映射（见 §8）定位/再自动创建该单位同名收入二级科目后入账。与建单位联动同一套口径。

---

## 7. 前端页面与交互

### 7.1 页面结构（`web/src/features/contacts/`）

| 文件 | 职责 |
|---|---|
| `ContactsLayout.vue` | 单位模块标签页容器（单位列表 / 应收土地流转费 / 应收投资收益 / 应收管理费 / 年度计提向导等） |
| `Pages/PartiesList.vue` | 单位列表：新增、编辑、按类型筛选、动态列、名称/类型只读 |
| `Pages/ReceivablesList.vue` | 三页应收共用组件：分年度筛选、状态标签、进度条、收缴（走快速记账） |
| `AccrueWizard.vue` | 年度计提向导：土地流转费 / 投资收益 / 管理费 分类分步计提 |
| `ContractsPage.vue` | 合同管理（单位分组下合同项折叠） |

### 7.2 单位列表动态列

按当前筛选类型动态隐藏列（th/td 同时用 `v-if` 整列移除，避免表头/内容错位）：
- 筛选 `投资单位`：隐藏"流转面积"列
- 筛选 `流转企业`：隐藏"投资金额"列

### 7.3 新增/编辑表单

- 投资类企业"年投资收益率"输入语法：按 **%** 计算（输入 3 → 0.03 = 300bps）。输入框用 `type="text" + inputmode="decimal"`，保证小数（如 3.5）可输入，PC 端不受 `type=number` 过滤限制。
- 编辑态：名称/类型灰显只读、不提交；年度数据保存后从 `PUT /parties/:id` 回填。

### 7.4 「收缴」→ 统一走快速记账通道

三个应收页每条记录右侧的【收缴】不再独立 `prompt + POST /receipts`，改调全局 `openRecord({ biz, partyId, amount })` 打开快速记账弹窗，预填对应收款业务与该单位，复用统一入账逻辑（见 §8）。

### 7.5 合同管理（ContractsPage.vue）

单位分组下展示合同文件，交互约定：
- **整行点击展开/折叠**：触发区域是单位所在整行（`.cp-group-header`，含单位名/类型/份数），悬停为可点击光标；行内右侧「上传到此单位」按钮加 `@click.stop` 避免误触发展开。
- **点合同名预览**：合同文件名（`.cp-file-name`）可点击，悬停变蓝，点击打开全屏预览（图片/Word/Excel/PDF/文本，`previewVisible`），与行内「查看」按钮等效。
- **上传名称清理**：异常文件名自动改为类型泛称并保留扩展名、同单位内去重：
  - 识别：含 `&`/`=`/`,`/`?` 的网址残留、无任何中英文字符、超长（>40）无空格串。
  - 映射：`jpg/jpeg/png/gif/webp/bmp→图片合同`、`doc/docx→Word合同`、`xls/xlsx/csv→Excel合同`、`pdf→PDF合同`、`txt/md→文本合同`、其他→`合同文件`。
  - 自动补扩展名（如 `图片合同.jpg`），同单位重名追加 `(2)(3)`；上传弹窗新增「合同名称」输入框（默认填清理名，可手动改），清理结果同时写入 `fileName` 与 `contractTitle`。

### 7.6 往来单位概览统计卡（Overview.vue）

四张统计卡（单位总数 / 本年应收合计 / 本年已收 / 本年未收）：
- **淡色背景**：淡紫 / 淡黄 / 淡蓝 / 淡红，去细边框，与投资管理页风格统一。
- **图标扁平化**：去掉「彩色圆底 + emoji」，改用 **方案C**——扁平灰色 Vant 线性图标（单位总数→`shop`、应收合计→`balance-list`、已收→`passed`、未收→`clock`），置于半透明白圆角底上，与淡色卡面融合。

---

## 8. 应收与自动入账

### 8.1 应收单（receivable）与 rec_kind

| rec_kind | 中文 | 归属应收页 |
|---|---|---|
| `rent` | 土地流转费 | 应收土地流转费 |
| `dividend` | 投资收益 | 应收投资收益 |
| `reinvest_dividend` | 再投资收益 | 应收投资收益 |
| `service` | 管理费 | 应收管理费 |
| `other` | 其他 | — |

迁移 [012_receivable_kind.sql](file:///e:/traework/jizhang/server/internal/platform/migrations/012_receivable_kind.sql) 扩展了 `recv_kind` 的 CHECK 约束以纳入 `service` / `reinvest_dividend`。

### 8.2 现金收款自动入账（rec_kind → 收入容器）

无预设入账科目时按 `recvKind → L1 容器` 自动定位/创建单位同名收入二级并入账（[repo.go#L416-L487](file:///e:/traework/jizhang/server/internal/receivable/repo.go#L416-L487)，`ResolveIncomeCategory`）：

| rec_kind | 收入容器 L1 |
|---|---|
| `rent` | 土地流转费收入 |
| `service` | 流转管理费 |
| `dividend` | 投资收益 |
| `reinvest_dividend` | 再投资 |

入账科目优先级：**本次指定 → 应收单预设 `income_category_id` → 自动定位/创建单位同名收入二级**。

完整收款流程（三个应收页 + 快速记账"收到土地流转费/投资收益/管理费"统一）：
1. 银行存款 **+金额**
2. 按 `rec_kind → L1` 该单位同名二级科目 **+金额**（不存在则自动创建，与建单位联动一致）
3. 对应应收 **-金额**；全部收齐应收单置 `closed`

> 前端 `saveQuick` 已修复重复入账：当通过折收方式（receipt）核销应收时，后端已生成银行收入流水，前端不再单独创建 transaction，避免同一金额双记。

### 8.3 应收页展示（ReceivablesList.vue，三页共用）

- **年度筛选**：位于列表左上，原生下拉（全部年度 / 2026年 / 2025年 / 无年度，倒序）。
- **状态标签**：圆角胶囊，按 已收/未收 判定 —— 未收=红、部分收=橙、结清=绿。
- **进度条**：加粗（9px）圆角，颜色跟随状态；`st-open` 红 / `st-partial` 橙 / `st-closed` 绿。
- **金额区**：应收合计 / 已收 / 未收 / 收缴率。
- 前端把后端 `recvKind` 归一化为 `kind`，兼容 `data / data.items / data.Items / 直接数组` 多种响应形态。

---

## 9. 与引导页、系统重置的关系

- 引导页（`OnboardingPage.vue`）录入单位时即触发 `CreateParty` 的联动建科目（见 08-onboarding-design.md）。
- 引导页录入的年度数据（投资本金/收益、流转费、管理费等）最终落 `party` 年度字段，作为后续"年度计提向导"（AccrueWizard）的数据源。
- 系统重置（`POST /api/system/reset`）清空 `party、transaction、receivable、...` 等业务表，保留预置 L1/L2 科目结构，用户需重新注册组织并走引导流程。

---

## 10. 边界与约束清单（验收要点）

- [x] 同类型下单位名称唯一；不同类型可同名。
- [x] 新建单位自动建同名二级科目（事务内、幂等、容器缺失跳过）。
- [x] 名称/类型不可编辑；单位可**有条件删除**（无欠款 + 科目余额 0，历史流水归档「历史归档」）。
- [x] 年度数据保存不回丢（011 迁移已落库，Create/Update 全链路读写）。
- [x] 投资收益率按 % 输入并可输入小数。
- [x] 按类型动态隐藏列，th/td 整列移除不错位。
- [x] 应收页分年度筛选 + 状态标签（红/橙/绿）+ 状态色进度条。
- [x] 应收【收缴】全部走快速记账通道，自动入账单位同名收入二级并核销应收。
- [x] 现金收款无预设科目时自动定位/创建收入二级，不再报错。
- [x] 作废收款回滚应收入账（不删自动创建的科目）。