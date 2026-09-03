# 集体台账 · 开发技术文档（正式版）

| 项 | 内容 |
|---|---|
| 文档版本 | v1.1（对齐代码 **v0.5.0**，2026-09-03 核对） |
| 项目代号 | 集体台账（jititaizhang） |
| 定位 | 集体经济组织内部收支台账：多组织自注册、移动优先、局域网自托管 |
| 关联文档 | [需求澄清 01](./01-requirements.md) · [PRD 02](./02-prd.md) · [设计 03](./03-design.md) · [CHANGELOG](../CHANGELOG.md) |
| 编写原则 | 本文档按**当前代码与测试实况**撰写；「待办/未实现」均显式标注，不与设计稿混淆 |

---

## 1. 项目概述

### 1.1 要解决的问题

替代村集体经济组织「无固定格式 Excel」的内部台账。核心场景：

- 单人/少数记账人用手机随手记一笔收支；
- 月底一键导出**无需手工加工**的收支汇总表；
- 上级拨本金、对外投资、收益分配、应收欠款等集体经济特有账务要能算清「钱在哪、谁欠多少」。

### 1.2 形态与技术约束（来自需求）

| 约束 | 取值 |
|---|---|
| 使用形态 | 一套部署多组织，各组织**完全隔离**、自助注册 |
| 记账模型 | 简单流水（单式）+ 科目余额，不做完整复式（D1） |
| 金额 | 一律以「分」为整数的 `INTEGER`，禁止浮点（5.1） |
| 部署 | 局域网 / NAS 自托管，低内存，Docker（**部署包待完成，见 §9.5**） |
| 数据留存 | 只增不改优先：科目停用、流水作废、变更留痕（D0/D3/D4） |

### 1.3 已交付功能全景（按版本）

| 版本 | 内容 | 状态 |
|---|---|---|
| v0.1 | 登录/科目/记账/流水/汇总/设置 | ✅ |
| v0.2 | 余额模型 D6、科目间转账 D7、导出（CSV/xlsx/科目余额表 D8） | ✅ |
| v0.3.1 | 多组织迁移 003、自助注册 + 预置科目、org 隔离 | ✅ |
| v0.3.3 | 资产科目 + 资金划转（D10，fund_move） | ✅ |
| v0.3.4 | 应收/往来后端（D11，party/receivable/receipt，迁移 004） | ✅ |
| v0.3.5 | 前端 5 tab、注册页、往来页、预置科目徽标 | ✅ |
| v0.3.6 | 备份/恢复（F7）、修改密码、变更历史查看（F6） | ✅ |
| v0.3.7 | 往来只单位、新增二级科目单位下拉、汇总点科目看当月流水 | ✅ |
| v0.4 | 模型重构（资产/权益、到账算收益、破坏性迁移 006）、看板、批量计提+标准一键结转、双端壳 | ✅ |
| v0.4.1 | 桌面去手机感、底部弹层居中、原生下拉（NativeSelect）、快速记账（10 业务模板自动入账） | ✅ |
| — | Docker/embed 一体化部署、PWA 收尾 | ⬜ 待办骨架（见 §9.5） |

---

## 2. 需求功能（正式整理）

> 完整逐条需求见 `02-prd.md`；本节为**已实现功能**的正式口径，供验收与二次开发对照。
> ⚠️ **v0.4 覆盖声明**：余额类型（余粮/花费）、参与勾稽、科目期初、“专项资金/未分配”口径已在迁移 006 移除；
> 本节 2.2–2.5 保留 v0.3 描述供追溯，**以 §2.8 v0.4 模型总则为准**。

### 2.1 组织与账号

| 功能 | 说明 | 约束 |
|---|---|---|
| 自助注册 | 组织名 + 管理员账号 + 密码 → 单事务建组织 + 账号 + 预置科目并自动登录 | 账号全局唯一；密码 ≥ 6 位；组织名可重复 |
| 登录/登出 | HttpOnly Cookie（`jt_session`），7 天有效 | 登录错误统一 401，不区分账号是否存在（防枚举） |
| 修改密码 | 校验原口令后更新 bcrypt 哈希 | 新密码 ≥ 6 位 |
| 组织隔离 | 会话携带 orgID，服务层所有 SQL 按 org 过滤 | 跨组织访问一律 404/不可见（含测试覆盖） |

**预置科目（注册即生成 11 项，preset=1，可改名/增删）**：

| 一级 | 预设二级 | 类型 | 勾稽 |
|---|---|---|---|
| 本金 | —（按拨款项目自建） | 余粮 | 开 |
| 对外投资 | —（按投资项目自建，资产型二级） | 余粮(存量口径) | — |
| 经营收入 | 土地流转费收入 / 投资分红收益 / 其他收入 | 余粮 | 关 |
| 收益分配 | 收益分红发放 / 流转费分发 / 福利发放 | 花费 | 关 |
| 公益支出 | —（按用途自建） | 花费 | 关 |

### 2.2 科目与余额

- 严格两级：一级科目 = 分组容器（禁期初/禁勾稽，余额 = 子项之和）；二级科目承载业务。
- 二级可设置：余额类型（余粮型收增支减 / 花费型支增收减）、期初余额、参与勾稽（仅余粮型）。
- 资产型二级（`kind=asset`）：固定余粮型、无期初、不参与勾稽、**只能走资金划转**，禁止记收支/科目间转账。
- 出口机制：未被引用科目可物理删除；已引用科目只能停用（停用后不再出现在录入下拉，历史归属不变）。

**科目余额公式（实时聚合，不落库）**：

```
普通科目余额 = 期初 + Σ收支(按类型方向) + Σ转入(转账 leg) − Σ转出(转账源)
资产科目余额 = Σ投资 − Σ收回（fund_move，normal）
```

### 2.3 记账 / 转账 / 资金划转

| 操作 | 规则要点 |
|---|---|
| 记一笔（收支） | 方向+金额+二级科目必选；金额 > 0；未来日期允许；连点由按钮禁用防重 |
| 科目间转账（D7） | 1 转出 → N 转入；Σ转入 = 转出额；转出 ≤ 科目余额；**花费型禁作转入方**；不碰银行 |
| 资金划转（D10） | 投资：银行 −N、资产 +N，且 N ≤ 当前银行存款；收回：银行 +N、资产 −N，且 N ≤ 该资产在外金额 |

三种操作均可**作废/撤销**，余额实时聚合自动回滚；每次创建/作废写 change_log。

### 2.4 汇总与资金构成（D6/D10 恒等式）

```
银行存款   = 银行期初 + Σ(收入−支出, 全年 normal) + Σ收回 − Σ投资
可用资金   = 银行存款 + Σ资产科目余额
专项资金   = Σ(参与勾稽的普通科目余额，含停用)
未分配资金 = 可用资金 − 专项资金
```

汇总页四卡：银行存款 / 投资资产 / 专项资金 / 未分配资金；未分配为负给出橙色警告。

### 2.5 应收 / 欠款（D11）

| 环节 | 说明 |
|---|---|
| 往来对象 | 农户/单位，含欠款合计（各 open 应收单未收之和），可增改 |
| 应收单 | 类别：流转费/投资收益/其他；事由、金额、可预设「收款入账科目」 |
| 现金收款（cash） | 单事务写核销 + **自动生成银行收入流水**（入账科目 = 本次指定 → 预设 → 否则报错） |
| 抵销（offset） | 关联一条**发放支出流水**（须 expense+normal+本组织），不产生现金流水，支出流水保留 |
| 结清 | Σ核销 == 应收额 → 自动 closed；closed 后禁止再核销（超收拒绝） |
| 作废核销 | cash 核销**连带作废**其收入流水；应收单未结清自动退回 open；抵销作废不动支出流水 |

### 2.6 备份 / 恢复 / 留痕（F6/F7）

| 能力 | 实现 |
|---|---|
| 手动备份 | `VACUUM INTO` 一致性快照；立即生成 |
| 自动备份 | 每日本机时区 03:00，保留最近 30 份（超量删除最旧） |
| 一键恢复 | 整库热恢复：旧库暂存 → 备份覆盖 → 重开迁移 → **进程内重建路由**，无需重启；失败自动回滚 |
| 恢复后行为 | 清除会话 Cookie，强制重新登录 |
| 变更留痕 | 实体的 create/update/void/unvoid 写入 change_log；前端「留痕」弹窗查看 |

> ⚠️ 恢复是**整库**一致性快照（含全部组织），非单组织粒度。

### 2.8 v0.4 模型总则（以本节为准）

- **科目**：二级类型只有 `asset 资产` / `equity 权益`；一级为分组容器。无期初、无勾稽、无余额类型。
- **银行存款** = 期初（设置页） + Σ到账收入 − Σ支出 ± 资金划转。
- **到账才算收益**：收入=银行 +、权益科目 +；支出=银行 −、权益科目 −；资金划转=银行 ↔ 资产科目。
- **往来应收欠款**单列“待收”，不计收益/资产；收到款自动冲欠款并计收入。
- **532**：仅按比例记对应账（再投资走资金划转、成员/公益走支出），无方案台账。
- **预置科目**（注册生成，4 一级 + 9 权益二级）：本金-上级补助；对外投资(空，自建资产二级)；经营收入-投资收益/土地流转费收入/流转服务费收入/其他收入；分配与支出-转付农户/成员分红/福利发放/公益支出。
- **快速记账**：10 个业务模板自动入账（见 §7.7）。
- **看板**：首页=银行/资产/待收/净资产 + 本年收益到账 + 欠款明细。

---

## 3. 系统架构

### 3.1 总体结构

```
浏览器（手机/桌面，Vue SPA）
   │ HTTPS / HTTP（局域网）
   ▼
Go 后端（Gin）
   ├─ /api/*       业务接口（JSON）
   ├─ 中间件       会话鉴权 → userID/orgID 注入
   ├─ handler 层   请求解析/参数校验/响应
   ├─ repo 层      SQL（全部带 org 过滤）
   └─ SQLite（WAL，单写者，foreign_keys=ON）
```

- 当前开发态 = 前端 Vite dev（:5173，proxy `/api` → :8080）+ 后端独立进程。
- 目标生产态 = 前端构建产物 embed 进 Go 二进制、单容器运行（**待完成**，§9.5）。

### 3.2 后端目录（`server/`，go.mod 模块 `jititaizhang/server`）

| 路径 | 职责 |
|---|---|
| `cmd/server/main.go` | 装配：配置→开库→迁移→路由；备份/恢复热切换；每日自动备份 goroutine |
| `internal/platform/` | 配置、DB(PRAGMA)、迁移、统一响应/错误 |
| `internal/platform/migrations/` | 版本化迁移 001–004（**实际位置**；根目录 `server/migrations/` 为旧占位） |
| `internal/auth/` | 注册/登录/会话/改密 |
| `internal/category/` | 科目树、余额聚合（CalcBalance/AssetBalance/BankDelta）、删除保护 |
| `internal/transaction/` | 收支流水 |
| `internal/transfer/` | 科目间转账（1→N） |
| `internal/fundmove/` | 资金划转（投资/收回） |
| `internal/receivable/` | 往来对象/应收单/核销（cash/offset） |
| `internal/summary/` | 资金构成、科目余额树、区间收支 |
| `internal/export/` | CSV/xlsx（流式）导出 |
| `internal/changelog/` | change_log 查询（被各模块写入方调用） |
| `internal/backup/` | 快照生成、列表、保留清理、文件级恢复 |
| `internal/settings/` | app_setting（银行存款期初） |

约定：Go 特性目录固定 `model.go / repo.go / handler.go`；handler 只做解析与校验，SQL 全部在 repo；service 集中在 auth（其余模块业务校验在 handler 内联，与既有代码一致）。

### 3.3 前端目录（`web/`）

| 路径 | 内容 |
|---|---|
| `src/features/auth/` | Login / Register / Pinia store |
| `src/features/transaction/` | Home 记账、List 流水（含筛选/编辑/作废/留痕/导出） |
| `src/features/summary/` | 汇总页（四卡资金构成、科目余额、区间收支、导出） |
| `src/features/category/` | 科目管理（含科目间转账、资金划转、资产二级、留痕） |
| `src/features/contacts/` | 往来页（对象列表/详情、登记应收、收款/抵销、核销记录） |
| `src/features/settings/` | 设置页（银行期初、科目入口、改密、备份恢复、登出） |
| `src/components/` | 跨特性通用组件（BottomNav、ChangeLogDialog 等） |
| `src/lib/` | http 封装（同源 /api、401 拦截）、download |
| `src/types/api.ts` | 前后端契约 + 金额/日期工具 |

---

## 4. 技术栈（实装版本）

| 层 | 技术 | 实装 | 备注 |
|---|---|---|---|
| 后端 | Go | 1.27.1（本机托管于 `E:\work\jizhang\.tools\go`，gitignored） | go.mod 要求 1.27 |
| HTTP | gin | v1.12.0 | |
| SQLite 驱动 | modernc.org/sqlite | v1.57.0（纯 Go，无 CGO） | |
| xlsx | xuri/excelize/v2 | v2.10.1 | 流式写入 |
| 口令 | golang.org/x/crypto/bcrypt | 最新 | |
| 前端 | Vue 3 + TS + Vite | 3.5.41 / 5.9.x / **6.4.3** | ⚠️ Vite 实装 6.4.3，设计稿 8.2.1，待用户拍板（可暂不处理） |
| UI | Vant | 4.9.x | |
| 状态/路由 | Pinia / Vue Router | 4.5.x | hash 路由 |

金额精度：全程整数分；展示层 `cents/100` 格式化（`formatFen/formatYuan`）。

---

## 5. 数据模型

迁移文件（`server/internal/platform/migrations/`，按文件名升序执行、只增不改、事务内执行）：

| 文件 | 内容 |
|---|---|
| `001_init.sql` | 首期 user/session/category/txn 骨架 |
| `002_balance_and_transfer.sql` | D6 字段、transfer/transfer_leg/change_log/app_setting |
| `003_multi_org.sql` | v0.3 全表重建带 org_id；category.kind/preset；org/fund_move/party/receivable/receipt |
| `004_receipt_status.sql` | 补 `receipt.status`（003 建表遗漏，历史行默认 normal） |
| `005_party_unit_only.sql` | 往来只单位：移除 party.kind（农户类型删除） |
| `006_v04_asset_equity.sql` | v0.4 破坏性重置：科目 kind=asset/equity，删余额类型/勾稽/期初；receivable 增 recv_year；新增 recv_standard |

### 5.1 表清单与关键字段

| 表 | 关键字段 | 说明/约束 |
|---|---|---|
| `org` | name | 组织 |
| `user` | org_id, username UNIQUE, password_hash | 1 组织 1 管理员账号 |
| `session` | id, user_id, expires_at | 服务端会话 |
| `category` | org_id, name, level(1/2), parent_id, status, kind(asset/equity), preset, sort_order | v0.4：仅 资产/权益；无期初/勾稽/余额类型；同级重名唯一 |
| `txn` | org_id, txn_date, direction(income/expense), amount_cents>0, category_id, note, status(normal/voided) | 流水（只挂 equity 二级） |
| `transfer` | org_id, txn_date, source_category_id, source_amount_cents, note, status | 转账（equity 之间） |
| `transfer_leg` | org_id, transfer_id, category_id, amount_cents | 转入明细 |
| `fund_move` | org_id, move_date, kind(invest/recover), asset_category_id, amount_cents, note, status | 资金划转（asset） |
| `party` | org_id, name, note | 往来单位（无类型） |
| `receivable` | org_id, party_id, recv_year, recv_kind(rent/dividend/other), title, amount_cents, income_category_id, status(open/closed), note | 应收单（recv_year 供批量计提防重） |
| `recv_standard` | org_id, party_id, recv_kind, amount_cents, active | 年度计提标准（单位+类别唯一） |
| `receipt` | org_id, receivable_id, amount_cents, receipt_date, method(cash/offset), txn_id, note, status(normal/voided) | 核销记录 |
| `change_log` | org_id, entity_type, entity_id, action(create/update/void/unvoid), field, old_value, new_value, changed_at | 留痕（无外键，实体删除仍保留） |
| `app_setting` | (org_id,key) PK, value | 如 `bank_opening_balance_cents` |
| `schema_migrations` | version, applied_at | 迁移账本 |

### 5.2 关系要点

- `category` 自关联（一级→二级）；收支/转账/应收入账科目只允许**普通启用二级**；资金划转只允许**资产启用二级**。
- SQLite PRAGMA：`journal_mode=WAL`、`foreign_keys=ON`、`busy_timeout=5000`、`SetMaxOpenConns(1)`（单写者）。

---

## 6. 接口清单（实际实现）

统一响应：成功 `{"data":...}`；失败 `{"error":{"code","message","details?"}}`。
公共：401 未登录；400 参数/业务拒绝（多数带中文 message）；404 资源不存在或**跨组织不可见**；409 冲突；500 服务异常。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/health` | 健康检查 |
| POST | `/api/auth/register` | 注册组织（自动登录） |
| POST | `/api/auth/login` / `logout` | 登录/登出 |
| PUT | `/api/auth/password` | 修改密码（需登录） |
| GET/POST/PUT/DELETE | `/api/categories` `/api/categories/:id` | 科目树/建/改/删 |
| GET/POST/PUT | `/api/transactions` `/api/transactions/:id` | 流水列表/记一笔/编辑作废 |
| GET/POST/PUT | `/api/transfers` `/api/transfers/:id` | 转账列表/创建/作废撤销 |
| GET/POST/PUT | `/api/fund-moves` `/api/fund-moves/:id` | 资金划转 |
| POST | `/api/receivables/batch` | 批量计提应收（多单位/年度防重，返回 created/skipped） |
| GET/POST | `/api/recv-standards` | 计提标准列表/保存（单位+类别唯一） |
| PUT | `/api/recv-standards/:id` | 启停计提标准 |
| POST | `/api/recv-standards/accrue` | 按启用标准一键结转年度应收 |
| GET | `/api/summary?from=&to=` | 汇总（收支/资金构成/科目余额树） |
| GET/POST/PUT | `/api/parties` `/api/parties/:id` | 往来对象 |
| GET/POST | `/api/receivables` | 应收单列表/登记 |
| GET | `/api/receivables/:id` | 应收单详情（含核销记录） |
| POST | `/api/receivables/:id/receipts` | 收款核销（cash/offset） |
| PUT | `/api/receipts/:id` | 作废核销 |
| GET/PUT | `/api/settings` | 银行期初余额 |
| GET | `/api/changelog?entityType=&entityId=` | 按实体查留痕 |
| GET | `/api/export?content=&format=&筛选` | 导出（transactions/summary/balance_sheet × csv/xlsx） |
| GET/POST | `/api/backups` | 备份列表/手动备份 |
| POST | `/api/backups/:id/restore` | 一键恢复（整库热恢复） |

主要错误码（节选）：`UNAUTHORIZED` `USERNAME_TAKEN` `OLD_PASSWORD_WRONG` `INVALID_PASSWORD` `CATEGORY_IN_USE` `CATEGORY_NAME_DUP` `CATEGORY_NOT_FOUND` `TRANSFER_AMOUNT_MISMATCH` `TRANSFER_INSUFFICIENT_BALANCE` `TRANSFER_SPENDING_LEG` `FUND_MOVE_INSUFFICIENT_BANK` `FUND_MOVE_INSUFFICIENT_ASSET` `RECEIPT_OVER_RECEIVABLE` `RECEIVABLE_CLOSED` `INCOME_CATEGORY_REQUIRED` `OFFSET_TXN_REQUIRED` `BACKUP_*` `RESTORE_FAILED` 等。

---

## 7. 操作说明（面向记账员）

> 账号口令见登录页注册入口；本文以角色「村集体记账员」口吻。

### 7.1 首次使用

1. 打开系统网址（开发态 http://localhost:5173 ；部署态见 §9）。
2. 登录页点「注册组织」→ 填组织名（如 XX村）+ 账号 + 密码（≥6 位）→ 注册即自动登录。
3. 进入「设置」→ 资金账户：填**银行存款期初余额**并保存（若从某时点建账，这里放时点的银行余额）。

### 7.2 日常记账（底部「记账」）

- 选 收入/支出 → 日期（默认今天，未来日期允许）→ 摘要 → 科目（只列普通启用二级）→ 金额 → 保存，可连续录入。
- 若列表没有合适科目：去「设置 → 科目管理」先建一级与二级。

### 7.3 科目管理（设置 → 科目管理）

- 「+ 新增一级」：分组容器，只需名称。
- 某一级下「+二级」：名称 + 类型（余粮/花费）+ 期初 + 勾稽；也可建**资产型二级**（对外投资等，无期初、不勾稽、只能资金划转进出）。
- 「转账」：科目间专款调剂/期末结转（1 转出 → 多条转入）。
- 「资金划转」：**投资**（银行→资产）或**收回**（资产→银行），底部是该资产「在外」金额提示，下方为划转记录（可作废/看留痕）。

### 7.4 查账与导出

- 底部「流水」：按日期/科目/关键字/金额筛选；转账行有标记；可编辑/作废/撤销/看变更历史。
- 底部「汇总」：四卡资金构成（银行/投资资产/专项/未分配）+ 科目余额树 + 区间收支持平；「导出」可选收支汇总或科目余额表（xlsx/csv），**导出即格式齐全免加工**。

### 7.5 往来与欠款（底部「往来」）

1. 「新增对象」（农户/单位）→ 点对象进入详情。
2. 「登记应收」：类别（流转费/投资收益/其他）+ 事由 + 金额；可预设收款入账科目（如土地流转费收入）。
3. 对未结清应收单「收款/抵销」：
   - **现金入账**：填金额/日期，自动记一笔银行收入（入账科目自动带出预设，未预设需选）；
   - **抵销**：选一笔已发生的分红支出流水，用其抵应收欠款（不产生现金流水）。
4. 展开核销记录可**作废**某次收款（现金核销作废会连带作废对应银行收入流水，欠款自动恢复）。

### 7.6 数据守护（设置）

- **立即备份**：生成当前一致性快照；列表按时间倒序显示，可随时**恢复**到任意备份点。
- 自动备份：每日 03:00（服务器本机时区）自动执行，保留最近 30 份。
- 恢复 = 整库回到快照时刻（所有组织都会回退），完成后强制重新登录；恢复前请先「立即备份」以免丢失新数据。
- **修改密码**：填原密码 + 两次新密码。

### 7.7 快速记账（看板 → 记一笔 → 快速记账）

按业务模板自动入账，减少选科目：

| 业务 | 操作提示 |
|---|---|
| 收上级财政补助 / 投资收益 / 服务费 / 利息 | 只填金额 → 自动记对应收入科目 |
| 拨付农户流转费 / 532成员分配 / 532公益 | 只填金额 → 自动记对应支出科目 |
| 收到某单位土地流转费 | 选往来单位 → 自动冲该单位欠款；无欠款则直记收入 |
| 投资给公司 / 收回投资 | 选公司（资产科目）→ 自动走资金划转 |

> 依赖预置科目名定位；若科目被改名会提示“缺少科目：XX”。

### 7.8 桌面端说明

- ≥992px：左侧导航 + 主内容铺满；表单/弹层居中；下拉为原生选择（鼠标友好）
- <992px：底部 5 tab + 底部弹层；科目/单位选择仍为移动端弹层

---

## 8. 部署与运维（当前状态）

### 8.1 环境变量（后端）

| 变量 | 默认 | 说明 |
|---|---|---|
| `APP_PORT` | 8080 | HTTP 监听 |
| `DATA_DIR` | ./data | SQLite 数据目录（必须本地盘，禁 NFS/SMB） |
| `APP_BACKUP_DIR` | ./backups | 备份目录（**建议与数据不同盘**） |
| `APP_KEEP_BACKUP` | 30 | 备份保留份数 |
| `APP_AUTO_BACKUP` | 1 | 每日 03:00 自动备份开关（0 关闭） |
| `APP_SECURE_COOKIE` | 空 | =1 时 Cookie 加 Secure（HTTPS 场景） |

### 8.2 本地开发启动

```bash
# 后端（Windows 本机 Go 1.27 或 .tools/go；模块走 goproxy.io）
cd server
APP_PORT=8080 DATA_DIR=../data go run ./cmd/server
# 前端
cd web && npm install && npm run dev   # http://localhost:5173，/api 代理到 :8080
```

### 8.3 质量关卡

```bash
cd server && go vet ./... && go test ./...   # 12 个包
cd web && npx vue-tsc -b && npx vite build
```

### 8.4 数据安全注意

- 数据目录与备份目录都要有异地/异盘副本（需求 R3：备份不与数据同盘）。
- 禁止把 `data/` 放到 NFS/SMB 网络盘（SQLite 锁不可靠）。
- `data/`、`backups/`、`web/node_modules`、`.tools/` 均不入库（`.gitignore`）。

### 8.5 ⬜ 待办：Docker / 一体化部署

- `deploy/Dockerfile`、`docker-compose.yml`、`scripts/build.sh` 目前为**骨架占位**：
  - 后端尚未 embed 前端产物（main.go 无静态资源托管）；
  - Dockerfile 仅有 `FROM scratch`，未完成多阶段构建。
- 到达成设计稿的「单容器 + 内嵌静态资源 + scratch」前，请以 §8.2 开发态运行。
- README「本地启动」小节亦待补（由部署收尾一并完成）。

---

## 9. 开发指南（给二次开发者）

### 9.1 环境与约定

- Go：`go.mod` 声明 go 1.27；本机无全局 Go 时用托管工具链 `E:\work\jizhang\.tools\go\bin\go.exe`，模块下载 `GOPROXY=https://goproxy.io,direct`。
- 新增后端特性包步骤：`model.go → repo.go → handler.go`；handler 提供 `NewHandler(db) *Handler` 与 `Register(r gin.IRouter)`；在 `cmd/server/main.go buildRouter` 里 `x.NewHandler(a.db).Register(authed)` 接线。
- 每个列表/写接口都带 org 过滤；跨组织返回 404；新模块补「跨组织不可见」测试。
- 写操作（建/改/作废）调用 `changelog.LogCreate/LogChangeVoid/LogUpdateField` 留痕。
- 数据库结构变更：在 `internal/platform/migrations/` 新增 `NNN_*.sql`（只增不改），同步 `internal/platform/migrate_test.go` 的迁移计数。

### 9.2 测试现状

后端含测试包：auth、backup、category、export、fundmove、platform、receivable、settings、summary、transaction、transfer（changelog 仅查询无测试）。重要回归：

- `TestD6*` / `TestD7FiveStep`（五步验算）、fundmove `TestD10InvestRecover`、receivable `TestCashReceiptFlow / TestOffsetReceiptFlow`、backup `TestCreateListRestore`、auth `TestChangePassword`。

### 9.3 冒烟与验收清单（建议发布前走一遍）

1. 注册两个组织 → 各得预置 11 科目，互不可见。
2. D6：银行期初 + 收支 → 汇总四卡恒等。
3. D10：本金拨款 → 投资/收回 → 资产科目余额与银行正确。
4. D11：登记欠款 → 现金收一部分 → 抵销 → 结清 → 作废核销回滚。
5. F7：立即备份 → 改动数据 → 恢复 → 数据回退、强制重登。
6. F5：导出收支汇总/科目余额表 xlsx 直接可打印。

---

## 10. 已知偏离与待办（透明清单）

| 项 | 状态 |
|---|---|
| Vite 实装 6.4.3 vs 设计 8.2.1 | 待用户拍板（可暂不处理） |
| Docker/embed 一体化部署 | 骨架占位，见 §8.5 |
| 生产静态资源托管（单二进制） | 未实现（开发态 = Vite + Go 双进程） |
| PWA 收尾、启动自检（网络盘检测） | 设计有、未实现 |
| 变更历史查看覆盖范围 | 流水/转账（流水页编辑弹层）、资金划转（科目页留痕）已可用；应收核销/科目自身留痕存于库中，暂未全部做成界面 |
| 本机测试数据 | `data/` 含冒烟组织（迁移 006 后仅 demo/test1234），正式使用建议删除重建 |

---

## 11. 变更记录

| 日期 | 说明 |
|---|---|
| 2026-09-03 | v1.0：按 v0.3.6 代码实况撰写（需求/架构/数据/接口/操作/部署/开发指南），待办显式标注 |
| 2026-09-03 | v1.1：同步 v0.3.7 / v0.4 / v0.4.1 —— 迁移 005/006、资产权益模型与看板/批量计提/标准结转、双端适配与快速记账、接口与操作补充 |
