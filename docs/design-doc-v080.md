# 集体台账 · 应用设计文档（v0.8.0）

| 项 | 内容 |
|----|------|
| 文档版本 | v1.0（对齐代码 **v0.8.0**，2026-09-06） |
| 项目代号 | 集体台账（jizhang） |
| 定位 | 集体经济组织内部管理台账：收支记账、往来管理、投资管理、流转管理、合同管理 |
| 关联文档 | [需求澄清](./01-requirements.md) · [PRD](./02-prd.md) · [开发手册](./dev-manual-v080.md) · [CHANGELOG](../CHANGELOG.md) |

---

## 1. 产品概述

### 1.1 要解决的问题

替代村集体经济组织「无固定格式 Excel」的内部管理方式。核心场景：

- 记账人每天随手记一笔收支（手机/电脑均可）；
- 跟踪往来单位欠款，分年度批量计提应收；
- 管理长期投资、再投资、投资收益及 532 分配；
- 分年度管理土地流转费收入与流转管理费；
- 管理合同/附件文件；
- 月底一键导出可直接使用的收支汇总表。

### 1.2 目标用户

- **记账人**：村集体经济组织的唯一系统操作者，可能不懂会计。
- 单组织单账号，一套部署多组织，各组织数据完全隔离。

### 1.3 产品形态

| 维度 | 取值 |
|------|------|
| 使用形态 | 浏览器 Web 应用，桌面适配 + 移动端响应式 |
| 部署方式 | 开发态：Vite Dev Server + Mock API Server；生产态：Windows x64 单 exe（前端已内嵌） |
| 访问方式 | 局域网内 HTTP 访问，手机/电脑均可 |
| 账号体系 | 自助注册，组织隔离 |

---

## 2. 页面结构

### 2.1 页面全景

```
登录 → 注册（首次使用）
  │
  ▼
┌─────────────────────────────────────────────────────┐
│ 桌面左侧导航（或移动端底部导航 × 5）                  │
│                                                      │
│  ◆ 收支总览（看板首页）                              │
│  ◉ 往来单位（7 子页面：概览/单位列表/应收三表/合同）   │
│  📈 投资管理（4 子 tab：长期投资/再投资/投资收益/532） │
│  🏠 流转管理（2 子 tab：土地流转费收入/流转管理费）    │
│  → 引导页面（7 步初始化向导）                        │
│  ＋ 快速记账（全局记账弹窗）                          │
│                                                      │
│  ⚙ 设置                                             │
│  🚪 退出                                             │
│  v0.8.0                                             │
└─────────────────────────────────────────────────────┘
```

### 2.2 页面详细清单

| 页面 | 路由 | 职责 | 核心元素 |
|------|------|------|----------|
| 登录 | `/login` | 账号密码登录 | 账号、密码、登录按钮、注册入口 |
| 注册 | `/register` | 组织自助注册 | 组织名、账号、密码、确认密码 |
| 收支总览 | `/` | 首页看板 | 银行/资产/待收/净资产四卡，本年收益，欠款明细，最新流水 |
| 往来概览 | `/contacts/overview` | 往来模块概览 | 欠款统计 banner，快速导航入口 |
| 单位列表 | `/contacts/parties` | 往来单位管理 | 单位表格（名称/类型/电话/面积/欠款/操作），编辑弹窗，快速记账入口 |
| 应收投资收益 | `/contacts/receivables/dividend` | 投资收益应收 | 年度筛选，应收/已收/未收统计，明细表格，导出 |
| 应收流转费 | `/contacts/receivables/rent` | 土地流转费应收 | 同上，按流转费类型 |
| 应收管理费 | `/contacts/receivables/service` | 流转管理费应收 | 同上，按管理费类型 |
| 合同管理 | `/contacts/contracts` | 合同/附件管理 | 按单位分组，上传/预览/下载/删除，文件类型筛选 |
| 长期投资 | `/investment` → tab:longterm | 长期投资管理 | 投资笔数/总额/年收益统计，明细表格，导出 |
| 再投资 | `/investment` → tab:reinvest | 再投资管理 | 同上 |
| 投资收益 | `/investment` → tab:returns | 投资收益查看 | 年度筛选，按单位统计收益 |
| 532 分配 | `/investment` → tab:dist532 | 532 分配管理 | 年度收入/分配状态，分配操作弹窗，公益支出记录 |
| 流转费收入 | `/flow` → tab:rent | 土地流转费收入 | 年度应收/已收/未收统计，农户转付支出记录 |
| 流转管理费 | `/flow` → tab:service | 流转管理费 | 年度管理费收入统计，管理费支出记录 |
| 引导页面 | `/onboarding` | 初始化向导 | 7 步引导：建流转企业→设余额→建投资公司→设余额→再投资→设余额→其他设置 |
| 流水列表 | `/transactions` | 流水查看/筛选 | 筛选栏（日期/科目/关键字），收支列表，编辑/作废 |
| 汇总 | `/summary` | 科目汇总 | 资金构成，科目余额树，区间收支，导出 |
| 科目管理 | `/categories` | 科目管理 | 科目树，增删改，科目间转账，资金划转 |
| 设置 | `/settings` | 系统设置 | 银行存款期初、科目管理入口、修改密码、备份恢复、登出 |

### 2.3 页面跳转关系

```
Login ──登录成功──→ Home(收支总览)
  │                    │
  │                    ├──点击快速记账 → RecordPopup
  │                    ├──导航到 Contacts/* (7 子路由)
  │                    ├──导航到 Investment (4 tab)
  │                    ├──导航到 Flow (2 tab)
  │                    ├──导航到 Onboarding
  │                    ├──导航到 Transactions
  │                    ├──导航到 Summary
  │                    └──导航到 Settings → Categories
  │
  └──注册成功──→ 自动登录 → Home
```

---

## 3. 导航设计

### 3.1 桌面端（≥992px）

- 左侧固定导航栏（196px 宽，深绿色背景）
- 顶部显示组织名 + 副标题
- 主功能区：收支总览、往来单位、投资管理、流转管理、引导页面、快速记账
- 底部：设置、退出、版本号
- 主内容区为白色卡片，圆角包裹

### 3.2 移动端（<992px）

- 底部固定导航栏，5 个 tab
- 弹窗为底部弹出（Vant 默认行为）
- 科目/单位选择使用移动端弹层

### 3.3 全局记账弹窗

- 通过 App.vue 的 `provide/inject` 提供全局 `openRecord()` 方法
- 桌面端：居中弹窗（600px 宽，`transition:""` 禁用动画）
- 移动端：底部弹出
- 默认模式：**快速记账**（10 个业务模板自动入账）
- 可选模式：普通记账（手动选科目）

---

## 4. 数据模型

### 4.1 核心实体

| 实体 | 表 | 关键字段 | 说明 |
|------|-----|----------|------|
| 组织 | `org` | id, name | 多组织隔离 |
| 用户 | `user` | id, org_id, username, password_hash | 1 组织 1 管理员 |
| 科目 | `category` | id, org_id, name, level(1/2), parent_id, status, kind(asset/equity), preset, sort_order | 两级，类型仅资产/权益 |
| 流水 | `txn` | id, org_id, txn_date, direction(income/expense), amount_cents, category_id, note, status | 收支流水 |
| 转账 | `transfer` | id, org_id, txn_date, source_category_id, source_amount_cents, note, status | 科目间转账 |
| 转入明细 | `transfer_leg` | id, transfer_id, category_id, amount_cents | 转账转入行 |
| 资金划转 | `fund_move` | id, org_id, move_date, kind(invest/recover), asset_category_id, amount_cents, note, status | 投资/收回 |
| 往来单位 | `party` | id, org_id, name, types[], contactPhone, areaMu, landFeePerMuCents, mgmtFeePerMuCents, investAmountCents, returnRateBps, expectedReturnCents, note | 含类型标签 |
| 应收单 | `receivable` | id, org_id, party_id, recv_year, recv_kind(rent/dividend/service/other), title, amount_cents, income_category_id, status(open/closed), paidCents, outstandingCents | 分年度应收 |
| 计提标准 | `recv_standard` | id, org_id, party_id, recv_kind, amount_cents, active | 年度自动结转标准 |
| 核销记录 | `receipt` | id, org_id, receivable_id, amount_cents, receipt_date, method(cash/offset), txn_id, note, status | 收款/抵销 |
| 再投资去向 | `reinvest_allocation` | id, org_id, party_id, year, amount_cents, note | 再投资分配明细 |
| 532 分配 | `distribution_532` | id, org_id, year, reinvestCents, memberCents, welfareCents, totalIncomeCents, status | 年度 532 分配方案 |
| 合同附件 | `contract` | id, org_id, party_id, partyName, fileName, fileData(base64), fileType, uploader, createdAt | 文件存储 |
| 变更日志 | `change_log` | id, org_id, entity_type, entity_id, action, field, old_value, new_value, changed_at | 操作留痕 |
| 系统配置 | `app_setting` | (org_id, key) PK, value | 银行期初等 |

### 4.2 实体关系

```
org 1 ──< N user
org 1 ──< N category
  category 自关联（一级 → 二级）
org 1 ──< N txn (category_id → category L2)
org 1 ──< N transfer (source_category_id → category L2)
  transfer 1 ──< N transfer_leg (category_id → category L2)
org 1 ──< N fund_move (asset_category_id → category L2 asset)
org 1 ──< N party
org 1 ──< N receivable (party_id → party)
org 1 ──< N recv_standard (party_id → party)
org 1 ──< N receipt (receivable_id → receivable)
org 1 ──< N reinvest_allocation (party_id → party)
org 1 ──< N contract (party_id → party)
```

### 4.3 金额口径

```
银行存款 = 银行期初 + Σ(收入 − 支出, 全年 normal) + Σ收回 − Σ投资
资产科目余额 = Σ(该科目投资) − Σ(该科目收回)
可用资金 = 银行存款 + Σ资产科目余额
专项资金 = Σ(参与勾稽的普通科目余额)
未分配资金 = 可用资金 − 专项资金
```

**v0.4+ 简化模型**：仅资产/权益类型，无勾稽/期初。到账才算收益。

---

## 5. API 接口

### 5.1 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/auth/login` | 登录 |
| POST | `/api/auth/register` | 注册组织 |
| POST | `/api/auth/logout` | 登出 |
| GET | `/api/me` | 当前用户信息 |

### 5.2 科目

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/categories` | 科目树 |
| POST | `/api/categories` | 新建科目 |
| PUT | `/api/categories/:id` | 修改科目 |
| DELETE | `/api/categories/:id` | 删除科目 |

### 5.3 流水

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/transactions` | 流水列表 + 筛选 |
| POST | `/api/transactions` | 记一笔 |
| PUT | `/api/transactions/:id` | 编辑/作废/撤销 |
| GET | `/api/transactions/:id/changes` | 变更历史 |

### 5.4 转账

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/transfers` | 转账列表 |
| POST | `/api/transfers` | 记一笔转账 |
| PUT | `/api/transfers/:id` | 作废/撤销 |

### 5.5 资金划转

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/fund-moves` | 资金划转列表 |
| POST | `/api/fund-moves` | 投资/收回 |
| PUT | `/api/fund-moves/:id` | 作废/撤销 |

### 5.6 汇总

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/summary` | 汇总（资金构成/科目余额/区间收支） |

### 5.7 往来单位

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/parties` | 单位列表（含欠款合计） |
| POST | `/api/parties` | 新建单位 |
| PUT | `/api/parties/:id` | 修改单位 |
| GET | `/api/parties/:id` | 单位详情 |

### 5.8 应收/核销

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/receivables` | 应收单列表 |
| POST | `/api/receivables` | 登记应收 |
| GET | `/api/receivables/:id` | 应收单详情（含核销记录） |
| POST | `/api/receivables/batch` | 批量计提应收 |
| POST | `/api/receivables/:id/receipts` | 收款核销（cash/offset） |
| PUT | `/api/receipts/:id` | 作废核销 |

### 5.9 计提标准

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/recv-standards` | 计提标准列表 |
| POST | `/api/recv-standards` | 保存标准 |
| PUT | `/api/recv-standards/:id` | 启停标准 |
| GET | `/api/recv-standards/preview` | 预览年度结转 |
| POST | `/api/recv-standards/accrue` | 一键结转年度应收 |

### 5.10 投资管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/investments/longterm` | 长期投资列表 |
| GET | `/api/investments/reinvest` | 再投资列表 |
| GET | `/api/investments/returns` | 投资收益列表 |
| GET | `/api/investments/dist532` | 532 分配数据 |
| POST | `/api/investments/dist532` | 保存分配数据 |
| POST | `/api/investments/dist532/allocate` | 执行分配 |

### 5.11 流转管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/flow/rent-summary` | 流转费收入汇总 |
| GET | `/api/flow/service-summary` | 管理费收入汇总 |
| GET | `/api/flow/farmer-expenses` | 转付农户支出记录 |
| GET | `/api/flow/mgmt-expenses` | 管理费支出记录 |

### 5.12 合同管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/contracts` | 合同列表 |
| POST | `/api/contracts` | 上传合同 |
| DELETE | `/api/contracts/:id` | 删除合同 |

### 5.13 设置/导出/备份

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/settings` | 读取系统配置 |
| PUT | `/api/settings` | 修改系统配置 |
| GET | `/api/export` | 导出（流水/汇总/科目余额表 × csv/xlsx） |
| GET | `/api/backups` | 备份列表 |
| POST | `/api/backups` | 手动备份 |
| POST | `/api/backups/:id/restore` | 恢复备份 |

---

## 6. 功能设计详细说明

### 6.1 快速记账（10 个业务模板）

| 业务 | 模板 | 入账规则 |
|------|------|----------|
| 收上级财政补助 | 收上级补助 | 记入「本金/上级补助」收入 |
| 收到投资收益/分红 | 投资收益 | 记入「投资收益」收入 |
| 收到土地流转费 | 土地流转费 | 选单位 → 自动冲该单位欠款，剩余记收入 |
| 土地流转服务费收入 | 服务费收入 | 记入「流转管理费」收入 |
| 拨付土地流转费给农户 | 转付农户 | 记入「分配与支出/土地流转费-转付农户」支出 |
| 银行存款利息 | 利息收入 | 记入「经营收入/其他收入」收入 |
| 532-成员分配发放 | 532成员分配 | 记入「分配与支出/成员分红」支出 |
| 532-公益支出 | 532公益支出 | 记入「分配与支出/公益支出」支出 |
| 投资给公司 | 投资公司 | 资金划转：银行→长期投资/公司名 |
| 收回投资 | 收回投资 | 资金划转：长期投资/公司名→银行 |
| 支出管理费 | 管理费支出 | 记入「分配与支出/管理费支出」支出 |

### 6.2 往来单位类型

| 类型 | 标识 | 用途 | 特有字段 |
|------|------|------|----------|
| 投资公司 | `invest` | 长期投资对象 | investAmountCents, returnRateBps, expectedReturnCents |
| 流转企业 | `flow` | 土地流转费对象 | landMu, landFeePerMuCents, mgmtFeePerMuCents |
| 再投资 | `reinvest` | 再投资去向 | — |
| 其它单位 | `other` | 通用往来单位 | — |
| 长期投资 | `longterm` | 投资公司（自动生成） | 关联投资金额自动同步 |

### 6.3 532 分配

- 按年度分配投资收益
- 分配比例：50% 再投资、30% 成员分配、20% 公益支出
- 分配弹窗：默认按剩余可分配金额生成 50/30/20 方案
- 校验：合计不得超过剩余可分配金额
- 已分配年份显示「已分配 ¥X.XX · 剩余 ¥Y.YY」
- 未分配年份显示「未分配」

### 6.4 合同管理

- 按往来单位分组展示
- 支持文件上传：PDF、Word、Excel、图片、文本文件
- 文件预览：PDF 内嵌 iframe 展示，图片直接展示，Office 文件提示下载
- 文件类型筛选（全部/合同/附件/凭证）
- 上传自动关联当前单位

### 6.5 引导页面（Onboarding）

- 7 步初始化向导，按单位类型分步
- 步骤结构：流转企业 → 流转企业余额 → 投资公司 → 投资公司余额 → 再投资 → 再投资余额 → 其他设置
- 完成后写 `localStorage` 标记，不再重复引导

---

## 7. 非功能需求

| 维度 | 要求 |
|------|------|
| 兼容性 | 桌面 Chrome ≥ 992px，移动端 Chrome/Safari |
| 金额精度 | 全程整数分存储，展示时分/100 并格式化 ¥ 千分位 |
| 数据隔离 | 组织间完全隔离，接口层强制 org 过滤 |
| 数据留存 | 只增不改，流水可作废不可删除，变更留痕 |
| 部署 | 开发态双进程，生产态单 exe（Windows x64） |

---

## 8. 版本迭代记录

| 版本 | 主要内容 |
|------|----------|
| v0.1 | 登录/科目/记账/流水/汇总/设置骨架 |
| v0.2 | 余额模型、科目间转账、导出（CSV/xlsx/科目余额表） |
| v0.3.1 | 多组织迁移、自助注册、预置科目、org 隔离 |
| v0.3.3 | 资产科目 + 资金划转 |
| v0.3.4 | 应收/往来后端 |
| v0.3.5 | 前端 5 tab、注册页、往来页 |
| v0.3.6 | 备份/恢复、修改密码、变更历史 |
| v0.3.7 | 往来只单位、新增二级科目单位下拉 |
| v0.4 | 模型重构（资产/权益）、看板、批量计提+标准一键结转 |
| v0.4.1 | 桌面去手机感、快速记账（10 业务模板） |
| v0.5.0 | 单 exe 打包、start.bat 启动脚本、Windows 部署 |
| v0.6.0 | 往来重构（单位类型化）、年度结转预览、科目期初、导出增强 |
| v0.7.0 | 投资管理（4 tab）、532 分配、全局记账弹窗、Mock API |
| v0.8.0 | 流转管理（土地流转费收入+流转管理费）、引导页面、合同管理 |

---

## 9. 变更记录

| 日期 | 变更内容 |
|------|----------|
| 2026-09-06 | 初稿，对齐 v0.8.0 代码实况，反映完整功能全景 |