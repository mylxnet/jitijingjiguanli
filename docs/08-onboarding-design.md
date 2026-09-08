# 08. 引导页（Onboarding）设计方案与详细逻辑

> 适用范围：首次注册新组织后的建账引导流程。
> 版本：v0.10.0（2026-09-08 补充按单位分组、流转企业费用去向、onConfirm 提交逻辑与缺陷 ⑥⑦ 后更新）
> 相关文档：`02-prd.md`、`06-linkage-rules.md`

---

## 1. 目标

引导页用于新组织完成**建账初始化**，一次走完就能正常记账。设计目标：

1. **按业务类型分步录入**：流转企业 → 投资公司 → 再投资，每类独立成步。
2. **自动联动建科目**：录入一个往来单位，系统自动在同一组织下按类型创建同名二级科目，无需用户手动建。
3. **期初余额可填**：为自动创建及预置科目设置建账时点的存量。
4. **只出现一次**：正常完成或跳过即写入完成标记，后续登录不再进入（系统重置后清除标记，重新注册再次出现）。
5. **重名有提示**：录入与已有单位相近的名称时给出候选，避免数据混乱。

---

## 2. 流程总览（7 步）

| 序号 | 步骤 | 类型 | 说明 |
|------|------|------|------|
| 1 | 流转企业 | add-party | 录入流转企业单位，联动建「土地流转费收入」「流转管理费」同名 L2 |
| 2 | 流转企业余额 | fill-balance | 按单位录入流转费/管理费（**存入基本信息，不入科目余额**，见 4.2） |
| 3 | 投资公司 | add-party | 录入投资公司，联动建「长期投资」同名 L2 |
| 4 | 投资公司余额 | fill-balance | 设置投资公司同名 L2 的期初 |
| 5 | 再投资 | add-party | 录入再投资去向单位，联动建「再投资」同名 L2 |
| 6 | 再投资余额 | fill-balance | 设置再投资同名 L2 的期初 |
| 7 | 其他设置 | final | 银行存款期初 + 预置科目期初 + 预览 + 确认提交 |

步骤定义（`web/src/features/onboarding/OnboardingPage.vue` 顶部 `STEPS`）：

```ts
{ label: '流转企业',   type: 'add-party',    partyType: 'flow' },
{ label: '流转企业余额', type: 'fill-balance', l1Names: ['土地流转费收入', '流转管理费'] },
{ label: '投资公司',   type: 'add-party',    partyType: 'invest' },
{ label: '投资公司余额', type: 'fill-balance', l1Names: ['长期投资'] },
{ label: '再投资',     type: 'add-party',    partyType: 'reinvest' },
{ label: '再投资余额',  type: 'fill-balance', l1Names: ['再投资'] },
{ label: '其他设置',   type: 'final' },
```

---

## 3. 前置条件与入口判定

### 3.1 谁有资格进入

- 引导页路由 `/onboarding` 需要登录（`meta.requiresAuth`）。
- 正常入口：注册成功 `Register.vue` 后跳转 `/onboarding`。

### 3.2 “只出现一次”判定（入口守卫）

`web/src/router/index.ts` 的 `beforeEach`：

```ts
const noOnboardPaths = ['/login', '/register', '/reset-password', '/onboarding']
if (to.meta.requiresAuth && !noOnboardPaths.includes(to.path) && auth.orgId &&
    !localStorage.getItem(`jt_onboarding_done_${auth.orgId}`)) {
  next('/onboarding')
  return
}
```

- 已登录用户**首次进入任何主流程页**（`/`、`/summary`、`/contacts`…），若当前组织 `jt_onboarding_done_{orgId}` 标记不存在 → 强制转引导页。
- `orgId` 来自 `auth store`（`checkLogin` 时调用 `/api/me` 缓存，取 `data.orgID`）。

### 3.3 完成标记

- `localStorage` 键：`jt_onboarding_done_{orgId}`。
- 写入时机：步骤 7 点「确认并完成」（`onConfirm`）或第 1 步点「跳过引导」（`skipToHome`）。
- 取值：`/auth/me` 返回的 `orgID`（大写 D），兼容旧字段 `orgId`：

```ts
const orgId = me.data?.orgID ?? me.data?.orgId ?? 'default'
localStorage.setItem(`jt_onboarding_done_${orgId}`, '1')
```

> ⚠️ 历史缺陷修复：后端 `/api/me` 返回 `"orgID"`（大写 D），早期前端用 `orgId`（小写）导致标记写成 `jt_onboarding_done_default`，与实际组织对不上。

### 3.4 标记的清除

- 系统重置时 `SettingsPage.vue` 遍历 `localStorage`，删除所有 `jt_onboarding_done_` 前缀键，使重新注册后引导页再次出现。

---

## 4. 各步骤详细逻辑

### 4.0 页面加载

`onMounted` 并发拉取：

```ts
cats.value = await api.get('/categories')   // 科目树（L1/L2，含 balanceCents）
parties.value = await api.get('/parties')   // 往来单位列表
```

### 4.1 add-party 步骤（录入单位）

**交互**
- 点「+ 新增」弹出简化弹窗（仅名称，类型由当前步骤决定）。
- 列表展示当前类型已有的单位（`getPartiesByType(partyType)`，兼容 `types[]` 与单值 `type`）。

**保存（`saveParty`）**

请求体（两端兼容）：

```ts
await api.post('/parties', { name, type: t, types: [t] })
```

后端处理顺序（`receivable/handler.go CreateParty`）：
1. **名称非空校验**
2. **类型解析**：`types` 数组优先于单值 `type`；取 `types[0]`；再否则默认 `flow`；ctype 必须合法（`flow/invest/reinvest`）。
3. **重名校验**（`repo.FindDuplicate`）：
   - 同组织 + 同名 + 同类型 → `409 DUPLICATE_NAME`「该类型下已存在同名单位」；
   - 同组织 + 名称模糊命中其它单位 → `409 DUPLICATE_NAME`，`Details` 携带候选单位名数组（`["甲社","甲合作社"]`）。
   - 前端提示“以下近似重名单位与您输入高度相近：…请修改名称”。
4. **事务内创建单位 + 联动建同名 L2**（详见第 5 节）。
5. 写操作日志 `LogCreate(orgID, 'party', id)`。
6. 前端刷新 `parties` 与 `cats`。

**canNext**：当前类型单位数 > 0 或勾选“跳过本步”，否则“下一步”禁用。

### 4.2 fill-balance 步骤（期初余额）

- **按单位分组渲染**（方案 A）：本步 (`balancePartyType`) 自动绑定其前一个 add-party 步骤的类型，按单位分组，每个单位一张卡片。卡片内列出该单位对应的**同名科目**，行标签用 L1 名区分（如流转企业卡片含「土地流转费收入」「流转管理费」两行）；系统预置/普通科目**已从本步剔除**（预置科目在第 7 步 final 处理）。
- 计算当前步涉及的同名 L2：`stepL2s` 仅遍历 `l1Names` 的 L1，收集其 children 中 name 等于当前类型单位名的 L2（剔除预置/普通科目）。
- 每行一个输入框，填入正数才提交；余额可选，不填默认 0。
- **流转企业费用去向（重点）**：流转企业填写的「土地流转费收入」「流转管理费」金额，**不写入同名科目期初**，而是保存到该单位**基本信息**（`expectedLandFeeCents` / `expectedMgmtFeeCents`）。同名科目照建、期初保持 0，余额由后续收到流水产生。投资/再投资不受影响（仍走「基本信息 investAmount + 同名科目期初」双写）。
- **导航**：可与上一步互跳；无“跳过本步”禁用限制（余额随时可跳过）。
- 空态：`unitBalances.length === 0` 提示“暂无可填余额的单位（可能上一步没有录入单位）”。

### 4.3 final 步骤（其他设置 + 预览）

- **银行存款期初**：单一输入。
- **预置科目期初**：列出所有 `l2.preset === true` 的二级科目（本金/经营收入等）。
- **预览**：银行存款、往来单位数量、非零科目余额汇总清单。
- **提交（`onConfirm`）**按实际代码顺序执行（代码位于 `OnboardingPage.vue`）：
  1. **投资/再投资双写**：对 `invest`/`reinvest` 单位，按其容器 L1（`长期投资`/`再投资`）查同名 L2，读取余额 → `PUT /parties/{id} { investAmountCents }`。
  2. **银行期初**：`bankOpening > 0` → `PUT /settings { bankOpeningBalanceCents }`。
  3. **流转企业费用 → 基本信息**：对 `flow` 单位，按其两个容器 L1（`土地流转费收入`/`流转管理费`）查同名 L2，读取金额 → `PUT /parties/{id} { expectedLandFeeCents, expectedMgmtFeeCents }`，并把这些科目的 id 记入 `skipCat`（不再写科目期初）。
  4. **其余科目期初**：遍历 `openingInputs`，跳过 `skipCat` 中的科目，`yuan > 0` → `PUT /categories/{id} { openingBalanceCents }`（投资/再投资同名科目与预置科目走此）。
  5. 写完成标记（第 3.3 节）。
  6. `router.push('/')`。

  > ⚠️ **关键坑**：`onConfirm` 在 final 步骤执行，此时 `stepL2s` computed 只在 `fill-balance` 步骤有值（final 时为空数组）。因此步骤 1（投资/再投资双写）与步骤 3（流转费用）均**不能依赖 `stepL2s`**，须像「按类型单位 → 按其容器 L1 查同名 L2」的方式直接解析（与后端 `CreateParty` 的容器 L1 映射一致），否则取不到科目、`skipCat` 为空，流转金额会被错误写入科目期初。
  > 预览（`previewBalances`）同样需排除 `flow` 单位的同名科目（其费用已存基本信息，不再列示为科目余额）。

> 说明：`opening_balance_cents` 是 category 表的**存储列**；`balanceCents` 是 API 端**计算值**（含 opening 汇总）。故改开头即驱动余额联动，无需额外同步。步骤 4.2/4.3 填的正是 opening。

---

## 5. 后端配套接口

### 5.1 `POST /api/parties`（`receivable`）

```http
POST /api/parties
{ "name": "某合作社", "type": "reinvest", "types": ["reinvest"] }
```

- 类型映射容器 L1（`repo.go` `typeToL1`）：

```go
var typeToL1 = map[string][]string{
	"flow":    {"土地流转费收入", "流转管理费"},
	"invest":  {"长期投资"},
	"reinvest": {"再投资"},
}
```

- 事务内（`repo.CreateParty`）：
  1. `INSERT party`（org_id, name, type, contact_phone, area_mu, note）；
  2. 对每个容器 L1：查 `level=1 AND status='active'` 的 L1 id；不存在则跳过；
  3. 若该 L1 下已存在同名 L2 则跳过，否则 `INSERT category(level=2, kind='equity', preset=0, status='active')`；
  4. `COMMIT`。

- 重名：`repo.FindDuplicate(orgID, name, type)` 返回 `(exact, dupes)`。

### 5.2 `PUT /api/categories/:id`

仅更新 `opening_balance_cents`（非负校验），并写操作日志字段 `opening_balance_cents`。balance 由 API 端汇总得出。

### 5.3 `PUT /api/settings`

更新 `bank_opening_balance_cents`（非负校验）。

### 5.4 `GET /api/me`（入口守卫与标记依赖）

返回 `{ userID, orgID, orgName }`。**注意 orgID 大小写**。

---

## 6. 数据模型与联动规则

- 单位：`party(id, org_id, name, type, contact_phone, area_mu, note, …)`，单值 `type` 落库；`types` 仅为前端多选兼容层（后端取 `types[0]` 作为 type）。
- 科目：`category(id, org_id, name, level, parent_id, status, kind, opening_balance_cents, preset, sort_order, …)`。
- 联动：**录入单位 → 自动按类型建同名二级科目**（容器 L1 下）；单位重名校验与科目自动建均收敛在后端 `CreateParty` 事务，全入口一致。

---

## 7. 边界与既有问题修复清单（2026-09-08）

| # | 缺陷 | 根因 | 修复 |
|---|------|------|------|
| ① | 单位类型全存成 flow | 引导页传 `types:[t]`，后端只读单值 `type` | 后端优先解析 `types[0]`；前端双传 `type+types` |
| ② | add-party 后余额步无科目可填 | “自动建同名 L2”只在注释声明，后端未实现 | `repo.CreateParty` 事务内联动建同名 L2 |
| ③ | 重名提示永不触发 | 后端无查重 | `FindDuplicate` + `409 DUPLICATE_NAME`/`details` |
| ④ | 引导页反复出现 | `/me` 返回 `orgID`(大写)，前端读 `orgId`(小写) → 标记 key 错 | 兼容读取 `orgID ?? orgId` |
| ⑤ | “只出现一次”无统一判定 | 只有“注册→引导”路径 | `router.beforeEach` 增加引导入口守卫 |
| ⑥ | fill-balance 平铺所有 L1 子科目，预置科目混入、看不出归属单位 | 按 L1 平铺，未按单位组织 | **按单位分组渲染**（每单位卡片），预置科目剔除（4.2） |
| ⑦ | `onConfirm` 阶段 `stepL2s` 为空，流转企业金额被误写入同名科目期初 | `stepL2s` 只在 `fill-balance` 步骤有值；final 步引用它拿不到科目、`skipCat` 空 | onConfirm 改为按类型单位+容器 L1 直接查同名 L2；流转费用改存单位基本信息 `expectedLandFeeCents`/`expectedMgmtFeeCents`，不写科目期初；`ListParties` 对 `invest` 单位不再用累计流水覆盖 `InvestAmountCents`（改用基本信息存储值） |

---

## 8. 验证要点（回归清单）

1. 各类型步骤新增单位后，`/categories` 对应容器 L1 下出现同名 L2。
2. 余额步骤**按单位分组**渲染，能看到上一步生成的同名 L2，可填期初且同步到 `balanceCents`。
3. 流转企业费用：余额步骤填「土地流转费收入/流转管理费」→ 确认后该单位**基本信息** `expectedLandFeeCents`/`expectedMgmtFeeCents` 正确、同名科目**期初保持 0**（不入科目余额）。
4. 投资/再投资费用：填余额 → 确认后该单位基本信息 `investAmountCents` 与同名科目期初一致（双写）。
5. 录入近似重名单位 → 弹出候选提示（`details` 中文名列表）；精确重名 → “已存在同名单位”。
6. 完成/跳过引导后，刷新进入 `/` 不再弹引导（`jt_onboarding_done_{orgID}` 存在）。
7. 系统重置 → 标记清除 → 重新注册再次出现引导页。