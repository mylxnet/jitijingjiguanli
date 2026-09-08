# 532 分配（投资管理）设计方案与技术逻辑

> 适用范围：投资管理页的「532 分配」板块。目标是把集体年净收益按 **5 : 3 : 2** 在「再投资 / 分红福利 / 管理（公益）」三档之间分配，并支持年度切换、执行记账与方案锁定。
> 本模块最初仅存在于 mock（`server/mock.js`，内存假数据），真实 Go 后端缺失。v0.7+ 已补齐真实后端（迁移 `013`），本文为补齐后的设计。

---

## 1. 概述与目标

- 按年度存储一份 **532 分配方案**（一年一条，upsert）。
- 支持跨年度切换，查看/编辑不同年度的分配数据。
- 方案执行后只要产生了对应支出流水，即 **锁定不可编辑**（硬约束，见 §5）。
- 三个类目的记账统一走**快速记账模板**（与全站记账打通，见 §6）。

---

## 2. 数据模型

迁移 [013_distribution_532.sql](file:///e:/traework/jizhang/server/internal/platform/migrations/013_distribution_532.sql) 新建表：

```sql
CREATE TABLE distribution_532 (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id             INTEGER NOT NULL REFERENCES org(id),
    year               INTEGER NOT NULL,
    total_income_cents INTEGER NOT NULL DEFAULT 0,   -- 年度净收益快照
    reinvest_cents     INTEGER NOT NULL DEFAULT 0,   -- 50% 再投资
    dividend_cents     INTEGER NOT NULL DEFAULT 0,   -- 30% 分红福利
    welfare_cents      INTEGER NOT NULL DEFAULT 0,   -- 20% 管理（公益）
    created_at         DATETIME NOT NULL,
    updated_at         DATETIME NOT NULL,
    UNIQUE (org_id, year)
);
```

- **org + year 唯一**：一年一份方案。
- 三档金额合计应与 `total_income_cents` 一致（前端按比率自动分摊，后端做非负校验）。

### 2.1 字段口径

| 字段 | 备注 |
|---|---|
| `total_income_cents` | 本年度可用于分配的总收益快照（前端展示为「年度总收益」） |
| `reinvest_cents` | 50% 再投资 |
| `dividend_cents` | 30% 分红福利 |
| `welfare_cents` | 20% 管理（公益） |

---

## 3. 后端接口

均挂在鉴权分组内，按当前登录用户 `orgId` 限定数据域。

### 3.1 `GET /api/distributions-532`

- 参数：`?year=N`（可选）。带 `year` 返回单年方案；否则返回全量年度数组。
- 返回：`data` 为记录数组（或单条），含 `reinvestCents/dividendCents/welfareCents/totalIncomeCents` 等驼峰字段（见 [model.go](file:///e:/traework/jizhang/server/internal/receivable/model.go) `Distribution532`）。

### 3.2 `POST /api/distributions-532`

- 请求：`{ year, totalIncomeCents, reinvestCents, dividendCents, welfareCents }`。
- 行为：按 `org_id + year` **upsert**——存在则更新金额与 `updatedAt`、保留 `createdAt`；不存在则新建。
- 校验：各档金额 ≥ 0；非法返回参数错误。
- 成功记录操作日志（`changelog`）。

### 3.3 相关能力

- `transaction` 列表接口支持 `direction` 过滤与「L1 传一级自动含全部二级子科目」（见 [handler.go](file:///e:/traework/jizhang/server/internal/transaction/handler.go)），用于 §5 锁定判定与 §6 已支统计。

---

## 4. 前端页面与交互

页面：`web/src/features/investment/pages/InvestmentsPage.vue`（532 板块）。

### 4.1 年度总收益卡

- 展示 `totalIncomeCents`（对应年度）。

### 4.2 三个类目统计卡（50% 再投资 / 30% 分红福利 / 20% 管理公益）

每张卡含主金额 + 一行副信息（本月近期视觉约定）：
- **总计** = 本年度方案分配金额
- **已支** = 该类目已记账的支出流水合计
  - 再投资 →「再投资」L1 下支出；分红 →「成员分红」科目支出；公益 →「公益支出」科目支出
- **余** = `总计 − 已支`（不小于 0），**「余」字与金额用红字**（`.rd`）；总计、已支保持默认灰字。
- 年度切换时自动重新统计「已支 / 余」。

### 4.3 分类分布引导

按土地流转费 / 投资收益 / 管理费分类分步录入（与年度计提向导呼应），确认后保存。

---

## 5. 年度锁定判定（硬约束）

> 规则：**方案执行后只要产生一条支出流水即锁定，不可编辑**。

- 锁定判断：查询该年度「公益支出」是否存在 `direction=expense` 的流水；只要存在一条即视为已执行，锁定编辑。
- **动态解析科目 id**：不再硬编码 `categoryId=84`，前端用已加载的 `categories` 按名称解析出「分配与支出」下名为「公益支出」的科目 id（`findEquityCat`），再拼 `direction=expense` 查询；未找到时退化为仅按方向过滤。
- 后端 `transaction` 列表已支持 `direction` 过滤（mock 曾忽略该参数，真实后端原不解析，已补齐），并支持传一级科目自动含其全部二级子科目，确保"该科目下任意支出流水"均能命中。

---

## 6. 记账与锁定的一致性

- 三个类目的【记账】按钮统一走**快速记账模板**（`RecordPopup.vue` `openRecord`），保证支出流水落到正确的支出科目（再投资 / 成员分红 / 公益支出）。
- 一旦通过快速记账产生任何支出流水，即触发 §5 锁定，方案不可再编辑，避免"先分配、再改比例、已支出对不上"。

---

## 7. 边界与约束清单（验收要点）

- [x] 一年一份方案，按年 upsert、按年切换。
- [x] 真实后端存在 `distribution_532` 表与 GET/POST 接口（迁移 013）。
- [x] 前端不再硬编码科目 id，公益支出科目按名动态解析。
- [x] 后端 `transaction` 列表支持 `direction` 过滤与 L1 含子科目过滤。
- [x] 类目卡展示 总计 / 已支 / 余，其中「余」与金额红字。
- [x] 产生支出流水后方案锁定不可编辑。
- [x] 记账统一走快速记账模板。