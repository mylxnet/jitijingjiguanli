# 联动规则校验变更摘要

> 生成日期：2026-09-06（第二次校验）
> 关联文档：[06-linkage-rules.md](./06-linkage-rules.md)

---

## 一、校验范围（第二次校验）

本次校验覆盖以下所有已确认的联动规则变更，逐一检查代码实现与规则文档的一致性。

### 新增/变更的规则

| 章节 | 规则 | 变更内容 | 状态 |
|------|------|----------|------|
| 1.1 | 编辑流水 | ❌ 已禁用，只能作废 | ✅ 已实现 |
| 1.1 | 支出余额防负 | 新增校验，不足时拒绝 | ✅ 已实现 |
| 3.3 | 重名校验 | 变更为名称+类型一致才判重 | ✅ 已实现 |
| 3.3 | 编辑/删除单位 | ❌ 已禁止 | ✅ 已实现 |
| 4.1 | 532分配方案锁定 | 执行后产生流水即锁定不可编辑 | ✅ 已实现 |
| 7.1 | 引导页只出现一次 | 确定后销毁，无后续修改 | ✅ 已实现 |

### 已有规则（首次校验，保持不变）

| 章节 | 规则 | 状态 |
|------|------|------|
| 1.1 | 记一笔收入/支出联动 | ✅ 符合 |
| 1.1 | 作废流水回滚联动 | ✅ 符合 |
| 1.2 | 快速记账 10 模板联动 | ✅ 符合 |
| 2.2 | 资金划转联动 | ✅ 符合 |
| 3.2 | 收款核销/作废回滚 | ✅ 符合 |
| 3.3 | 新增单位 → 自动创建科目 | ✅ 符合 |
| 4.1 | 532 分配 → 快速记账模板 | ✅ 符合 |
| 5.1 | 流转费收入/拨付联动 | ✅ 符合 |
| 5.2 | 管理费收入/支出联动 | ✅ 符合 |
| 7.1 | 引导页余额同步 | ✅ 符合 |
| 7.2 | 设置页期初余额 | ✅ 符合 |
| 9 | 操作日志记录 | ✅ 符合 |

---

## 二、本次修改的代码变更

### 变更 1：禁止单位编辑/删除

**后端**：[mock.js:699-703](file:///e:/traework/jizhang/server/mock.js#L699-L703) `PUT /api/parties/:id` 返回 403 `PARTY_READONLY`。

**前端**：[ContactsPage.vue:73-75](file:///e:/traework/jizhang/web/src/features/contacts/ContactsPage.vue#L73-L75) 移除单位详情弹窗中的"编辑"按钮。

**前端**：[OnboardingPage.vue:253-254](file:///e:/traework/jizhang/web/src/features/onboarding/OnboardingPage.vue#L253-L254) 移除引导页单位列表中的"编辑"/"删除"按钮，移除 `editParty`/`deleteParty` 函数及 `editingParty` ref。

### 变更 2：流水不可编辑，只能作废

**后端**：[mock.js:574-595](file:///e:/traework/jizhang/server/mock.js#L574-L595) `PUT /api/transactions/:id` 仅允许 `status: 'voided'` 操作，其他修改返回 403 `TXN_READONLY`。

**前端**：[transaction/List.vue:162-250](file:///e:/traework/jizhang/web/src/features/transaction/List.vue#L162-L250) 编辑弹窗改为只读详情视图，移除编辑表单（日期/摘要/科目/金额/方向选择器），仅保留作废/撤销作废按钮。清理 `editForm`、`categoryOptions`、`handleSaveEdit` 等已废弃代码。

### 变更 3：532 分配方案锁定

**后端**：[mock.js:1143-1153](file:///e:/traework/jizhang/server/mock.js#L1143-L1153) `POST /api/distributions-532` 检查是否存在该年度相关流水，有则返回 403 `DIST_LOCKED`。

**前端**：[InvestmentsPage.vue:505-527](file:///e:/traework/jizhang/web/src/features/investment/pages/InvestmentsPage.vue#L505-L527) `openDistDialog()` 根据 `distExpenses` 长度判断是否锁定，锁定时代入现有方案数据以只读展示。弹窗显示红色锁定提示，输入框 readonly，确认按钮变为"关闭"。

### 变更 4：支出余额防负校验

**后端**：[mock.js:550-558](file:///e:/traework/jizhang/server/mock.js#L550-L558) `POST /api/transactions` 在支出方向时计算当前银行存款余额（`openingBalance + 收入合计 - 支出合计`），若 `amountCents > currentBalance` 返回 400 `INSUFFICIENT_BALANCE`。

### 变更 5：重名校验改为名称+类型一致才判重

**后端**：[mock.js:133-143](file:///e:/traework/jizhang/server/mock.js#L133-L143) `findDuplicateParties()` 新增 `type` 参数，过滤时比较单位类型，仅名称和类型同时匹配才算重复。

---

## 三、首次校验发现的问题（已修复）

### 问题 1：引导页科目期初余额未同步到当前余额

**现象**：引导页 OnboardingPage.vue 的 `onConfirm()` 调用 `PUT /api/categories/:id` 设置 `openingBalanceCents`，但服务器端仅存储了该字段，未同步更新 `balanceCents`，导致科目余额一直为 0。

**修复**：`mock.js` 中 `PUT /api/categories/:id` 处理函数新增 `cat.balanceCents = body.openingBalanceCents`。

### 问题 2：引导页银行期初字段名错误

**现象**：引导页向 `/api/settings` 发送 `bankBalanceCents`，但服务器端期望的字段名是 `bankOpeningBalanceCents`。

**修复**：OnboardingPage.vue 中 `bankBalanceCents` → `bankOpeningBalanceCents`。

### 问题 3：HTTP 客户端错误信息丢失

**现象**：后端返回 409 DUPLICATE_NAME 时，前端 HTTP 客户端仅提取错误消息字符串，未保留响应状态码和数据结构。

**修复**：`http.ts` 增强错误对象，附加 `err.status` 和 `err.response` 属性。

### 问题 4：引导页重名提示不友好

**现象**：引导页创建单位遇重名时，仅显示 `alert('保存失败：请求失败 (409)')`。

**修复**：OnboardingPage.vue 增加 409 错误分支，显示具体重复单位名称。

### 问题 5：往来单位页面重名提示不友好

**现象**：ContactsPage.vue 的 `saveParty()` 遇重名时仅显示通用弹窗"保存失败"。

**修复**：增加 409 错误分支，使用 `showDialog` 显示具体重复单位名称。

### 问题 6：引导页查找/创建 L2 科目函数未 await

**现象**：OnboardingPage.vue 的 `findOrCreateL2()` 函数内部调用未被 await，导致返回 `null`。

**修复**：改为 `async function` 并 await 内部调用。后因服务器端已自动创建 L2 科目，移除了引导页的 `findOrCreateL2` 调用。

---

## 四、修改文件清单

| 文件 | 修改类型 | 说明 |
|------|----------|------|
| `server/mock.js` | 变更 | 禁止单位编辑、流水仅允许作废、532 分配锁定、余额防负校验、重名校验改为名称+类型一致 |
| `web/src/features/contacts/ContactsPage.vue` | 变更 | 隐藏单位编辑按钮 |
| `web/src/features/onboarding/OnboardingPage.vue` | 优化 | 隐藏编辑/删除按钮，清理已废弃代码 |
| `web/src/features/transaction/List.vue` | 变更 | 编辑弹窗改为只读详情，仅保留作废按钮，清理废弃代码 |
| `web/src/features/investment/pages/InvestmentsPage.vue` | 变更 | 532 分配方案锁定（只读视图 + 锁定提示） |
| `web/src/lib/http.ts` | 修复 | 保留错误响应的状态码和数据结构 |
| `docs/06-linkage-rules.md` | 更新 | 文档版本 v1.1 → v1.2 |

---

## 五、剩余风险与建议

| 风险点 | 说明 | 建议 |
|--------|------|------|
| 转账（transfer）仍可编辑 | 本次未处理转账流水编辑限制 | 后续可参照 transaction 逻辑，将转账编辑弹窗也改为只读 |
| 532 分配锁定仅按年度判断 | 后端锁定判断基于 `txnDate` 年份 + note 包含关键字，粒度较粗 | 当前方案按用户确认的"方案级判断"实现，若后续需要更精确的关联可优化 |
| 余额防负仅覆盖直接记支出 | 核销、转账等操作也可能导致余额为负 | 后续可在这些操作入口也增加余额校验 |

---

## 六、回归测试要点

1. **引导页流程**：创建单位 → 自动创建 L2 科目 → 设置余额 → 确认后看板余额正确
2. **重名校验**：输入已存在的同名同类型单位 → 弹窗提示；同名不同类型 → 允许创建
3. **单位不可编辑**：点击单位详情 → 无"编辑"按钮；引导页单位列表 → 无"编辑"/"删除"按钮
4. **流水详情**：点击流水行 → 打开只读详情视图，仅可作废；确认作废后回滚正常
5. **532 分配锁定**：执行分配产生流水后 → 再次打开"分配数据"弹窗显示只读
6. **余额防负**：银行存款不足时记支出 → 拒绝并提示