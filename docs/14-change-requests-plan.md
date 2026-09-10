# 14 · 修改意见处理方案（v0.13.x 待审阅）

> 本文档针对 7 条修改意见，逐条给出**现状定位 / 根因 / 方案 / 影响面 / 待确认点**。审阅确认后再实施。
> 结论先行：**第 5 条是根因级问题**（再投资收益整条链路断裂，连带影响第 4 条）；**第 1 条涉及数据模型决策**（是否给流水补单位外键）；其余 5 条为局部修复。

---

## 一、快速记账建投资公司后数据不一致（往来无投资记录、投资页负数）

### 现状
- "长期投资给公司"模板：`invest: true`，**不选往来单位**（[RecordPopup.vue:152](file:///e:/traework/jizhang/web/src/features/transaction/RecordPopup.vue#L152)）。
- 新增公司时只做三件事：建「长期投资」同名二级科目、建 `type='invest'` 往来单位、**随后保存一笔 `expense` 流水**（[handleCreateCompany L276-L315](file:///e:/traework/jizhang/web/src/features/transaction/RecordPopup.vue#L276-L315)、[saveQuick L391-L410](file:///e:/traework/jizhang/web/src/features/transaction/RecordPopup.vue#L391-L410)）。
- 新建单位**未写任何金额**：`invest_amount_cents / return_rate_bps / expected_return_cents` 全为 0；也不产生计提标准。

### 根因（三条各自独立的断链）
1. **科目余额为负**：投资被实现为"长期投资二级科目的一笔 expense"，而该科目按 equity 口径计算 `余额 = 期初 + Σ收入 − Σ支出`（[CalcBalance](file:///e:/traework/jizhang/server/internal/category/repo.go#L199-L224)）→ 恒为负。
2. **展示未统一方向**：记账弹窗显示时取反（`balanceCents<0 ? -balanceCents : 0`，[RecordPopup.vue:256](file:///e:/traework/jizhang/web/src/features/transaction/RecordPopup.vue#L256)），投资页原样显示负数（[InvestmentsPage.vue:41](file:///e:/traework/jizhang/web/src/features/investment/pages/InvestmentsPage.vue#L41)）。
3. **投资与往来/应收解耦**：`txn` 表**没有 party_id**（[001_init.sql L47-L57](file:///e:/traework/jizhang/server/internal/platform/migrations/001_init.sql#L47-L57)、[transaction/model.go](file:///e:/traework/jizhang/server/internal/transaction/model.go)），前端即使传 `partyId` 也被静默丢弃；投资本身也**不产生应收单**，而"应收投资收益"页只读 receivable，故为空。三处（科目余额 / party 字段 / receivable）互不相通。

### 方案（分两级，建议先 A 后 B）
**A. 一致性修复（低风险，本次做）**
1. 投资页"长期投资/再投资"金额**按投出口径取绝对值展示**（或标注"投出"），与记账弹窗口径一致；
2. 保存"长期投资给公司"时，**同步累计该单位 `investAmountCents`**（新建公司场景此时已有 party id）：`PUT /parties/:id { investAmountCents: 原值+本次 }`；
3. 新建公司时若确定金额，建单位即带 `investAmountCents`；
4. 明确"应收投资收益"= **收益应收**（dividend / reinvest_dividend），不含投资本金；若要看到本金往来，在单位详情新增"投资流水"区（读该单位同名科目下的 expense 流水）。

**B. 根本修复（需拍板，改动较大）**
`txn` 增加 `party_id`（迁移）+ `POST /transactions` 支持 partyId，使流水与单位真正关联。收益：投资/收款/应收可对账、单位可反查流水、为"投资记录"提供数据基础。代价：迁移 + 事务接口校验 + 前端补传。

### 其他模块同类问题排查（同源）
| 入口 | 问题 |
|---|---|
| RecordPopup 收到投资收益/管理费 | `kindMap` **缺 `reinvest_dividend`**，选再投资单位时按 `dividend` 匹配 → 核销匹配不到再投资收益应收单（[RecordPopup.vue L341-L354](file:///e:/traework/jizhang/web/src/features/transaction/RecordPopup.vue#L341-L354)） |
| 各单位"自动建单位" | 建 L2 与建 party 两处都会建同名科目（靠 `SELECT EXISTS` 去重），**靠名字关联**，改名/重名即断链 |
| 532 记账 | 前端传 `partyId` 同样被后端丢弃（[InvestmentsPage.vue L696-L704](file:///e:/traework/jizhang/web/src/features/investment/pages/InvestmentsPage.vue#L696-L704)），仅落在 note 文本 |

---

## 二、看板"银行存款"卡点击查看明细流水

### 现状
看板统计卡为纯展示，无点击行为（[Home.vue](file:///e:/traework/jizhang/web/src/features/transaction/Home.vue)）。系统已有流水页 `/transactions`（[router/index.ts:54](file:///e:/traework/jizhang/web/src/router/index.ts#L54)），后端 `GET /api/transactions` 已支持 `from/to/direction/categoryId/keyword/page` 过滤。

### 方案（二选一，请拍板）
- **方案 1（推荐，改动小）**：点击"银行存款"卡 → 跳转流水页 `/transactions`。因所有收支都过银行，等同于"银行存款流水"；可在流水页标题带一句"银行存款流水"。
- **方案 2**：卡上弹出弹窗，按日期倒序展示全部流水（含收/支/余额小计），不离开看板。

### 影响面
仅前端：`Home.vue` 卡片加点击与样式；方案 1 无需改后端。

---

## 三、提交数据时偶尔跳转引导页

### 根因（已定位，非随机）
引导跳转判定在路由守卫（[router/index.ts:96-L100](file:///e:/traework/jizhang/web/src/router/index.ts#L96-L100)）：
`requiresAuth && 非白名单 && auth.orgId && !localStorage['jt_onboarding_done_'+orgId]`。
问题出在两侧状态不稳定：
1. **`auth.orgId` 只在"硬刷新且未登录"时被填充**（[store.ts checkLogin L19-L37](file:///e:/traework/jizhang/web/src/features/auth/store.ts#L19-L37) 仅由守卫在 `!isLoggedIn` 时调用）；`login()/markLoggedIn()/logout()` 都不维护它。→ 用户**刷新过**才有值，此时一旦标记缺失，**任意后续导航（提交成功后的跳转）都会被拦到引导页**。
2. **标记键不一致**：读用 `orgID`（取不到为 `null`），写在引导页用 `orgID ?? 'default'`（[OnboardingPage.vue L255-L258](file:///e:/traework/jizhang/web/src/features/onboarding/OnboardingPage.vue#L255-L258)），字段异常时写成 `_default`，永远命中不了。
3. **写标记被业务失败连累**：`onConfirm` 把"业务写入 + 写标记"放同一 try，任一失败则不写标记；`skipToHome` 的 `/auth/me` 失败被静默吞掉却仍跳首页（[OnboardingPage.vue L204-L275](file:///e:/traework/jizhang/web/src/features/onboarding/OnboardingPage.vue#L204-L275)）→ 造成"假完成"，之后每次导航都被打回引导页。
4. 系统重置会清空所有标记（[SettingsPage.vue L357-L365](file:///e:/traework/jizhang/web/src/features/settings/SettingsPage.vue#L357-L365)），属预期，但会与上面问题叠加。

### 首次修复（v0.14.0，仍不足）
1. **统一引导完成标记的读写**（抽 `onboardingKey(orgId)` helper，去掉 `'default'` 兜底；取不到 orgId 就不写并给出明确提示）；
2. **修正 orgId 生命周期**：`login()`/`markLoggedIn()`/注册成功后统一拉一次 `/api/me` 填充 `orgId`（或注册接口直接返回 orgID）；`logout()`/401 时清空，避免跨组织串用；
3. **写标记与业务解耦**：`skipToHome` 必须写标记（失败要显式提示而非静默）；`onConfirm` 业务失败仍留在引导页，但提供"跳过引导"出口必须可靠；
4. **老组织兼容**：对"已有业务数据但无标记"的组织，登录后不应被强制引导。方案：前端在判定前先看该组织是否已有数据（如 `/parties` 非空视为已建账），有则自动补写标记；或后端在组织上存 `onboarded` 标记（改动稍大）。

### 最终修复（服务端权威标记）
v0.14.0 的前端修复后仍偶发，因为判定仍依赖两个不稳定来源：
- **localStorage 标记**按「浏览器 + 站点来源」存储 → 清缓存 / 换浏览器 / 换访问地址即丢失；
- 兜底探测 `GET /api/parties` 的**失败被静默吞掉** → 标记丢失后那次恰好失败，就被误判为"未建账"。

已实施：
1. **迁移 017**：`org` 加 `onboarded`（默认 0），并把"已有业务数据"的既有组织回填为 1（party / txn / receivable / contract / fund_move / 银行期初 / 科目期初 任一非空）；
2. **`/api/me` 返回 `onboarded`**；新增 **`POST /api/onboarding/complete`** 置 1；
3. **前端**：store 以 `/api/me().onboarded` 为准，移除 localStorage 标记与 `/api/parties` 探测；守卫仅在 `orgId` 有值且未完成时跳转 → 请求异常不再误跳。

### 影响面
后端：迁移 017、`/api/me`、`onboarding` 包新增接口；前端：auth store、路由守卫、引导页、设置重置。

---

## 四、往来单位页"应收投资"应含长期投资 + 再投资收益

### 现状
路由已配置 `kind: ['dividend','reinvest_dividend']`（[router/index.ts:30](file:///e:/traework/jizhang/web/src/router/index.ts#L30)），**过滤范围本身是对的**。

### 根因
"看不到再投资收益"是**数据没被生产**：整条 `reinvest_dividend` 链路断裂（见第五条）。属第 5 条的连带表现，修好第 5 条即自然包含。

### 方案
无需为第 4 条单独改前端；实施第 5 条并回归验证该页两类收益都能出现即可。

---

## 五、再投资收益未列入年度计提范围（根因级，建议优先）

### 现状（链路逐层断裂）
| 环节 | 问题位置 |
|---|---|
| 标准落库 | 迁移 014 重建 `recv_standard` 时 CHECK **不含 `reinvest_dividend`**，物理上存不进去（[014 L11](file:///e:/traework/jizhang/server/internal/platform/migrations/014_recv_standard_service.sql#L11)） |
| 写标准 | `applyFeeStandards` 把 `invest` 与 `reinvest` **都写成 `dividend`**（[handler.go L233-L238](file:///e:/traework/jizhang/server/internal/receivable/handler.go#L233-L238)） |
| 预览 | 预览 SQL 无 `reinvest_dividend` 分支，且 `dividend` 只匹配 `p.type='invest'` → reinvest 单位双重排除（[PreviewAccrueAuto L980-L988](file:///e:/traework/jizhang/server/internal/receivable/repo.go#L980-L988)） |
| 结转 | `AccrueFromStandards` 的 partyType map 无该键，直接返回空（[repo.go:936](file:///e:/traework/jizhang/server/internal/receivable/repo.go#L936)）；接口 oneof 也拒收（[handler.go:726](file:///e:/traework/jizhang/server/internal/receivable/handler.go#L726)） |
| 向导 | `AccrueWizard` 只有 3 组（无再投资收益），`filter(it=>it.kind===g.key)` 会静默丢弃未知 kind（[AccrueWizard.vue L99-L119](file:///e:/traework/jizhang/web/src/features/contacts/pages/AccrueWizard.vue#L99-L119)） |
| 其他 | `ListReceivables` kind 白名单漏 service/reinvest_dividend（[repo.go:288](file:///e:/traework/jizhang/server/internal/receivable/repo.go#L288)）；导出 kindLabel 缺两类（[export.go:328](file:///e:/traework/jizhang/server/internal/export/export.go#L328)）；收款的 `kindMap` 缺 reinvest（[RecordPopup.vue L341-L354](file:///e:/traework/jizhang/web/src/features/transaction/RecordPopup.vue#L341-L354)） |

### 方案
1. **迁移 016**：
   - 重建 `recv_standard`，CHECK 加入 `reinvest_dividend`；
   - 把 `reinvest` 单位的既有 `dividend` 标准**改为 `reinvest_dividend`**（金额沿用）；已生成的历史应收单**保持原样**（历史事实不改）；
   - 迁移 015 未部署到的环境由 016 兜底。
2. **后端映射统一**：`applyFeeStandards`（reinvest→reinvest_dividend）、`PreviewAccrueAuto` SQL（新增 reinvest 分支，`dividend` 仅匹配 invest）、标题 switch、`AccrueFromStandards` map、`AccrueByStandards` oneof、`ListReceivables` kind 过滤、导出 label。
3. **前端**：`AccrueWizard` 增加"再投资收益"分组（步骤变为 5 步：土地流转费 / 投资收益 / 再投资收益 / 管理费 / 确认）；`RecordPopup.kindMap` 按单位类型区分 `dividend` / `reinvest_dividend`；`ReceivablesList` 金额带出对两类共用 `expectedReturnCents`。
4. 回归：再投资单位建单位→填年收益→计提标准→预览→结转→应收页→收款核销 全链打通。

### 待确认
- 再投资单位的收益应收类别统一为 `reinvest_dividend`（与长期投资 `dividend` 区分）——是否符合你的业务口径？还是两者合并为 `dividend`（则只需放行 service 那类修补）？**建议区分**，便于"再投资收益"独立统计。

---

## 六、532 分配：已支出完的项目禁用"记账"

### 现状
分配明细三行按钮（再投资/成员分红/公益）**无任何禁用条件**（[InvestmentsPage.vue L206-L228](file:///e:/traework/jizhang/web/src/features/investment/pages/InvestmentsPage.vue#L206-L228)）。页面已有"总计/已支/余"数据（`reinvestStat/dividendStat/welfareStat`，[L508-L522](file:///e:/traework/jizhang/web/src/features/investment/pages/InvestmentsPage.vue#L508-L522)）。

### 方案
每行按钮绑定 `:disabled="xxxStat.remain <= 0"`（三类各自判断），并加 title/提示"该项目已全部支出完毕"；同时记账弹窗确认时也校验一次（防绕过）。无需改后端。

---

## 七、532 年度支出记录：显示全部 532 支出 + 时间列提前

### 现状
- 数据来自 `GET /api/transactions?categoryId={公益支出}&direction=expense`（[loadDist L524-L549](file:///e:/traework/jizhang/web/src/features/investment/pages/InvestmentsPage.vue#L524-L549)）：
  - **只查"公益支出"一个科目**，不含再投资、成员分红；
  - **没有年度过滤**（后端支持 from/to 却未使用），切换年份数据不变；
  - 未传 `pageSize`（默认 50 条，可能截断）；
  - 列顺序为 支出项目 / 类别 / 金额 / **日期（末列）**。

### 方案
1. 数据源改为**三类支出**并行查询：再投资（L1 含子科目）、成员分红、公益支出；均带 `from=YYYY-01-01&to=YYYY-12-31&direction=expense&pageSize=10000`；
2. 行内含**类别**（三类之一）与**时间列移至首列**：`时间 / 支出项目 / 类别 / 金额`；
3. 金额列右对齐（现状保持）。

### 待确认
- "532 支出"如何界定？
  - **方案 a（按科目）**：三个对应科目下的全部支出（实现简单，但可能含非 532 场景的同类支出）；
  - **方案 b（按科目 + note 标记）**：只取 `note` 以 `532-` 开头（现有记账默认备注均为此格式），更精准但用户改备注会漏；
  - **方案 c（严格）**：需要给流水加"532 归属"标记/字段（改动最大，最准确）。

---

## 八、待确认问题清单（请逐条回复）

| 编号 | 问题 | 建议 |
|---|---|---|
| Q1 | 第 1 条做到什么程度：仅"显示与字段一致"(A)，还是同时给流水补 `party_id` 做根本关联(B)？ | 先 A，B 列入下阶段 |
| Q2 | 第 2 条用"跳转流水页"还是"看板弹窗"？ | 跳转流水页 |
| Q3 | 第 5 条：再投资收益类别与长期投资收益**区分**（reinvest_dividend）还是**合并**？历史应收单是否保留原类别？ | 区分；历史保留 |
| Q4 | 第 7 条"532 支出"范围按 a/b/c 哪种？ | 方案 a 起步，可加 note 过滤(b) |
| Q5 | 本批改动版本号：v0.13.1（修缺陷）还是 v0.14.0（含 reinvest 类别扩展）？ | v0.14.0 |

---

## 九、建议实施顺序与验证

1. **第一批（根因 + 高收益）**：第五条全链、第三条引导页、第六条按钮禁用；
2. **第二批（展示一致性）**：第一条 A 级、第二条、第七条；
3. 每批：`go test ./...`（新增 reinvest 计提链路、按钮禁用、532 支出过滤用例）→ 本地真实环境重启验收 → 通过后统一构建镜像。

---

## 十、实施状态（2026-09-10）

按建议（Q1=A 级、Q2=跳转流水页、Q3=区分 `reinvest_dividend` 且历史数据保留、Q4=按科目口径、Q5=v0.14.0）已全部实施：

| 意见 | 状态 |
|---|---|
| 一（投资一致性，A 级） | ✅ 投资页金额按投出口径展示；保存投资累计单位投资额 |
| 二（看板银行流水入口） | ✅ 银行存款卡点击打开弹窗，展示 日期/收入/支出/余额（余额 = 【设置】的银行存款期初 + 逐笔收/支累计）；桌面端为居中弹窗，移动端为底部弹层 |
| 三（引导页偶发跳转） | ✅ 改为服务端权威标记（迁移 017 `org.onboarded` + `/api/me` 返回 + `POST /api/onboarding/complete`），移除 localStorage 标记与 `/api/parties` 探测兜底 |
| 四（应收投资含两类收益） | ✅ 由第五条链路打通后自然包含 |
| 五（再投资收益计提） | ✅ 迁移 016 + 后端全链 + 向导新增步骤 |
| 六（532 已支完禁用记账） | ✅ 按钮禁用 + 二次校验 |
| 七（532 年度支出记录） | ✅ 三类支出 + 年度过滤 + 时间首列 |

后端 `go test`（receivable/onboarding/export）全绿，前端 `vite build` 通过；版本号 v0.14.0。
