# 集体台账 · 开发技术文档（v0.8.0）

| 项 | 内容 |
|----|------|
| 文档版本 | v1.0（对齐代码 **v0.8.0**，2026-09-06） |
| 项目代号 | 集体台账（jizhang） |
| 定位 | 集体经济组织内部管理台账：Web SPA，Mock API 开发，Windows 单 exe 部署 |
| 关联文档 | [设计文档](./design-doc-v080.md) · [需求澄清](./01-requirements.md) · [PRD](./02-prd.md) · [CHANGELOG](../CHANGELOG.md) |

---

## 1. 项目概述

### 1.1 技术栈

| 层 | 技术 | 版本 | 备注 |
|---|---|---|---|
| 前端框架 | Vue 3 | 3.5.41 | Composition API + `<script setup>` |
| 构建工具 | Vite | 6.4.3 | Rolldown 引擎，快速 HMR |
| 类型 | TypeScript | 5.9.x | — |
| UI 组件 | Vant | 4.9.x | 移动端优先，桌面端适配 |
| 状态管理 | Pinia | 4.5.x | 配合 Vue Router |
| 路由 | Vue Router | 5.2.0 | hash 模式 |
| HTTP 客户端 | 原生 fetch 封装 | — | 封装在 `src/lib/http.ts` |
| 后端（开发态） | Node.js Mock API | — | 纯 http 模块，无依赖 |
| 后端（生产态） | Go + Gin | 1.27.1 / v1.12.0 | 单 exe 内嵌前端 |
| 数据库 | SQLite（modernc.org/sqlite） | v1.57.0 | 纯 Go，无 CGO |
| 金额精度 | 整数分（INTEGER） | — | 全程整数，展示时 ÷100 格式化 |

### 1.2 项目目录结构

```
e:\traework\jizhang\
├── docs/                          文档
│   ├── 01-requirements.md         需求澄清
│   ├── 02-prd.md                  产品需求文档
│   ├── 03-design.md               设计方案（v0.3 基线）
│   ├── 04-dev-manual.md           开发手册（v0.5 基线）
│   ├── 05-user-manual.md          操作手册（v0.5 基线）
│   ├── design-doc-v080.md         应用设计文档（当前）
│   ├── dev-manual-v080.md         开发技术文档（当前）
│   ├── release-v0.5.0.md          v0.5.0 发布说明
│   ├── release-v0.6.0.md          v0.6.0 发布说明
│   ├── prototype.html             页面原型
│   └── HOME-CONTINUE.md           回家继续开发指引
│
├── web/                           前端（Vue 3 + TS + Vite）
│   ├── src/
│   │   ├── features/              按特性组织
│   │   │   ├── auth/              登录、注册、会话状态（store.ts）
│   │   │   ├── transaction/       记账（Home.vue）、流水列表（List.vue）、
│   │   │   │                       记账弹窗（RecordPopup.vue）
│   │   │   ├── summary/           汇总页（SummaryPage.vue）
│   │   │   ├── category/          科目管理（CategoryPage.vue）
│   │   │   ├── contacts/          往来模块
│   │   │   │   ├── ContactsLayout.vue     子 tab 导航
│   │   │   │   ├── ContactsPage.vue       旧版（保留）
│   │   │   │   └── pages/
│   │   │   │       ├── Overview.vue       概览页
│   │   │   │       ├── PartiesList.vue    单位列表
│   │   │   │       ├── ReceivablesList.vue 应收三表（按 kind 复用）
│   │   │   │       └── ContractsPage.vue  合同管理
│   │   │   ├── investment/        投资管理模块
│   │   │   │   ├── InvestmentLayout.vue   4 tab 导航
│   │   │   │   └── pages/
│   │   │   │       └── InvestmentsPage.vue 四个 tab 全部内容
│   │   │   ├── flow/              流转管理模块
│   │   │   │   ├── FlowLayout.vue         2 tab 导航
│   │   │   │   └── pages/
│   │   │   │       └── FlowPage.vue       两个 tab 全部内容
│   │   │   ├── onboarding/        引导页面（OnboardingPage.vue）
│   │   │   └── settings/          设置页（SettingsPage.vue）
│   │   ├── components/            跨特性通用组件
│   │   │   ├── BottomNav.vue      移动端底部导航
│   │   │   ├── SideNav.vue        桌面端左侧导航
│   │   │   └── ChangeLogDialog.vue 变更留痕弹窗
│   │   ├── composables/           通用组合式函数
│   │   │   └── useScreen.ts       屏幕尺寸检测（桌面/移动）
│   │   ├── lib/                   工具库
│   │   │   ├── http.ts            HTTP 客户端封装
│   │   │   └── download.ts        下载工具
│   │   ├── types/api.ts           前后端共享类型定义
│   │   ├── styles/theme.css       全局主题样式
│   │   ├── router/index.ts        路由配置
│   │   ├── App.vue                根组件（布局 + 全局记账弹窗）
│   │   └── main.ts               入口
│   ├── index.html
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── _patch.cjs                 兼容补丁
│   └── package.json
│
├── server/                        后端（Go 源码 + Mock API）
│   ├── mock.js                    Node.js Mock API 服务（开发用）
│   ├── cmd/server/main.go         Go 入口
│   ├── internal/                  Go 业务模块
│   │   ├── auth/                  登录、会话、注册
│   │   ├── transaction/           收支流水
│   │   ├── category/              科目管理
│   │   ├── transfer/              科目间转账
│   │   ├── fundmove/              资金划转
│   │   ├── receivable/            应收/核销
│   │   ├── summary/               汇总
│   │   ├── export/                导出（CSV/xlsx）
│   │   ├── changelog/             变更留痕
│   │   ├── backup/                备份/恢复
│   │   ├── settings/              系统配置
│   │   └── platform/              平台层（DB/迁移/错误）
│   ├── tests/                     Node.js 测试脚本
│   └── go.mod + go.sum
│
├── data/                          数据库目录（gitignored）
├── backups/                       备份目录（gitignored）
├── deploy/                        Docker 骨架（待完成）
├── scripts/                       构建脚本
├── jititaizhang.exe               生产态单 exe（v0.5.0 产物）
├── start.bat                      Windows 启动脚本
├── VERSION                        版本号
├── CHANGELOG.md                   变更日志
└── .gitignore
```

---

## 2. 前端架构

### 2.1 路由架构

使用 Vue Router **hash 模式**（`createWebHashHistory`），路由配置见 `src/router/index.ts`。

**路由表**：

| 路径 | 组件 | 权限 | 说明 |
|------|------|------|------|
| `/login` | Login.vue | 无 | 登录页 |
| `/register` | Register.vue | 无 | 注册页 |
| `/` | Home.vue | 需登录 | 首页看板 |
| `/transactions` | List.vue | 需登录 | 流水列表 |
| `/summary` | SummaryPage.vue | 需登录 | 汇总 |
| `/categories` | CategoryPage.vue | 需登录 | 科目管理 |
| `/settings` | SettingsPage.vue | 需登录 | 设置 |
| `/contacts` | ContactsLayout.vue | 需登录 | 往来模块（含 7 子路由） |
| `/contacts/overview` | Overview.vue | 需登录 | 往来概览 |
| `/contacts/parties` | PartiesList.vue | 需登录 | 单位列表 |
| `/contacts/receivables/dividend` | ReceivablesList.vue | 需登录 | 应收投资收益 |
| `/contacts/receivables/rent` | ReceivablesList.vue | 需登录 | 应收流转费 |
| `/contacts/receivables/service` | ReceivablesList.vue | 需登录 | 应收管理费 |
| `/contacts/contracts` | ContractsPage.vue | 需登录 | 合同管理 |
| `/investment` | InvestmentLayout.vue | 需登录 | 投资管理（4 tab） |
| `/flow` | FlowLayout.vue | 需登录 | 流转管理（2 tab） |
| `/onboarding` | OnboardingPage.vue | 需登录 | 引导页面 |

**路由守卫**：`beforeEach` 检查 `auth.isLoggedIn`，未登录则跳转 `/login`。

### 2.2 状态管理

使用 Pinia，存放于 `src/features/auth/store.ts`：

```typescript
// store.ts 核心状态
export const useAuthStore = defineStore('auth', () => {
  const isLoggedIn = ref(false)
  const user = ref<User | null>(null)
  const orgName = ref('')

  async function checkLogin() { /* 调用 /api/me 检查会话 */ }
  async function login(username: string, password: string) { /* 登录 */ }
  async function logout() { /* 登出 */ }
  // ...
})
```

### 2.3 全局记账弹窗

通过 **provide/inject** 模式实现全局可用的记账弹窗：

```typescript
// App.vue
const recordPopupRef = ref<InstanceType<typeof RecordPopup> | null>(null)
provide('openRecord', () => {
  recordPopupRef.value?.openRecord()
})
```

组件内注入使用：
```typescript
const openRecord = inject<() => void>('openRecord', () => {})
```

**RecordPopup.vue** 核心逻辑：
- 桌面端居中弹窗（600px 宽），禁用 Vant 过渡动画（`transition=""`）
- 默认「快速记账」模式，可切换到「普通记账」
- 快速记账：10 个业务模板，自动选科目/冲欠款
- 普通记账：手动选方向/日期/摘要/科目/金额

### 2.4 响应式布局

屏幕尺寸检测使用 `useScreen.ts` composable：

```typescript
export function useScreen() {
  const isDesktop = ref(window.innerWidth >= 992)
  // 监听 resize 事件
  return { isDesktop }
}
```

- **桌面端（≥992px）**：左侧 SideNav + 主内容卡片区，弹窗居中
- **移动端（<992px）**：底部 BottomNav，弹窗从底部弹出

### 2.5 主题样式

全局 CSS 变量定义在 `src/styles/theme.css`：

```css
:root {
  --jade: #07c160;          /* 主色 - 翡翠绿 */
  --jade-deep: #0b4226;     /* 深绿 - 导航背景 */
  --paper: #f5f6f8;         /* 页面背景 */
  --ink-900: #1f2329;       /* 文字主色 */
  --ink-500: #646566;       /* 文字次要色 */
  --line-soft: #eaeaea;     /* 分割线 */
  --r-sm: 6px;              /* 圆角 */
  /* ... */
}
```

### 2.6 金额格式化

```typescript
// 分 → 元 显示，统一 ¥ 千分位格式
function fmt(cents: number): string {
  return `¥ ${(cents / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}
```

---

## 3. 前端模块详解

### 3.1 往来模块（Contacts）

**文件结构**：
- `ContactsLayout.vue`：子 tab 导航（7 个 tab），包裹 `<router-view>`
- `pages/Overview.vue`：概览页，欠款统计 banner
- `pages/PartiesList.vue`：单位列表，表格展示，编辑弹窗，快速记账入口
- `pages/ReceivablesList.vue`：应收三表，通过 `props.kind` 区分类型（dividend/rent/service），复用同一组件
- `pages/ContractsPage.vue`：合同管理，按单位分组，上传/预览/删除

**单位类型**：`PARTY_TYPES` 定义为 `['invest', 'flow', 'reinvest', 'longterm', 'other']`，通过 `types` 数组字段标记，支持多类型交集。

### 3.2 投资管理模块（Investment）

**文件结构**：
- `InvestmentLayout.vue`：4 tab 导航，包裹 `<InvestmentsPage>`
- `pages/InvestmentsPage.vue`：按 `activeTab` 切换四个 tab 内容

**四个 tab**：
1. **长期投资**：投资公司列表，展示投资金额、年收益率、年收益
2. **再投资**：再投资去向列表，同上
3. **投资收益**：按年度/单位查看投资收益，含应收/已收/未收统计
4. **532 分配**：按年度查看分配方案，可执行分配操作

**532 分配逻辑**：
- 年度总收入来自「投资收益」科目余额
- 分配弹窗展示总余额、已分配、剩余可分配
- 默认按 50/30/20 比例生成方案
- 校验：合计不得超过剩余可分配金额
- 已分配年份显示「已分配 ¥X.XX · 剩余 ¥Y.YY」
- 分配生成的记账：再投资不走资金划转，直接记入科目

### 3.3 流转管理模块（Flow）

**文件结构**：
- `FlowLayout.vue`：2 tab 导航
- `pages/FlowPage.vue`：按 `activeTab` 切换

**两个 tab**：
1. **土地流转费收入**：年度应收/已收/未收统计，明细表格（可折叠），转付农户支出记录
2. **流转管理费**：年度管理费收入统计，管理费支出记录

**表格折叠**：表头可点击展开/收起详情行，默认收起，使用 `v-show` 控制。

### 3.4 引导页面（Onboarding）

7 步初始化向导，使用 `localStorage` 标记完成状态：

```typescript
// 步骤结构
const STEPS = [
  { label: '流转企业',     type: 'add-party',    partyType: 'flow' },
  { label: '流转企业余额', type: 'fill-balance',  l1Names: ['土地流转费收入', '流转管理费'] },
  { label: '投资公司',     type: 'add-party',    partyType: 'invest' },
  { label: '投资公司余额', type: 'fill-balance',  l1Names: ['长期投资'] },
  { label: '再投资',       type: 'add-party',    partyType: 'reinvest' },
  { label: '再投资余额',   type: 'fill-balance',  l1Names: ['再投资'] },
  { label: '其他设置',     type: 'final' },
]
```

---

## 4. Mock API 服务器

### 4.1 概述

开发态使用 `server/mock.js` 替代 Go 后端，提供完整 API 模拟。

- 纯 Node.js http 模块，无外部依赖
- 端口 8080
- 测试账号：`admin / admin888`
- 内存数据存储（重启丢失）
- 种子数据：预置科目、往来单位、应收数据、流水、532 分配等

### 4.2 数据模型（Mock.js 实现）

内存数据数组：

| 变量 | 类型 | 说明 |
|------|------|------|
| `CATEGORIES` | Array | 科目树，含 L1/L2 层级 |
| `PARTIES` | Array | 往来单位，含类型、面积、费率等 |
| `TRANSACTIONS` | Array | 收支流水 |
| `TRANSFERS` | Array | 科目间转账 |
| `RECEIVABLES` | Array | 应收单（含种子数据） |
| `RECEIVABLES_532` | Array | 532 分配应收 |
| `CONTRACTS` | Array | 合同附件（base64 存储） |
| `DISTRIBUTIONS_532` | Array | 532 分配记录 |
| `REINVEST_ALLOCATIONS` | Array | 再投资去向明细 |
| `SETTINGS` | Object | 系统配置 |

### 4.3 路由实现

```javascript
// 统一响应格式
function sendJSON(res, status, data) {
  res.writeHead(status, { 'Content-Type': 'application/json; charset=utf-8' })
  res.end(JSON.stringify({ data }))
}

function sendFail(res, status, code, msg) {
  res.writeHead(status, { 'Content-Type': 'application/json; charset=utf-8' })
  res.end(JSON.stringify({ error: { code, message: msg } }))
}
```

**鉴权机制**：Cookie-based session，`session=mock_session`。

### 4.4 种子数据

初始化时自动生成：
- 8 个一级科目（本金/长期投资/再投资/经营收入/投资收益/土地流转费收入/流转管理费/分配与支出）
- 3 个流转类型单位（绿野种植合作社/丰源农业公司/金穗家庭农场）
- 3 个投资类型单位
- 2026 年度应收种子数据（流转费 + 管理费）
- 转付农户支出流水 + 管理费支出流水

### 4.5 启动方式

```bash
node server/mock.js
# 或使用 start.bat 自动启动（生产态）
```

---

## 5. 启动与开发

### 5.1 开发态启动

```bash
# 1. 启动 Mock API（端口 8080）
cd e:\traework\jizhang
node server/mock.js

# 2. 启动前端 Vite Dev Server（端口 5173）
cd e:\traework\jizhang\web
npm install
npm run dev

# 3. 浏览器访问 http://localhost:5173
# 登录 admin / admin888
```

### 5.2 生产态构建

```bash
# 前端构建
cd web && npm run build

# Go 后端编译（需 Go 1.27 工具链）
cd server && go build -o ../jititaizhang.exe ./cmd/server
```

### 5.3 质量关卡

```bash
# 前端类型检查 + 构建
cd web && npx vue-tsc -b && npx vite build

# Go 后端检查
cd server && go vet ./... && go test ./...
```

### 5.4 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `APP_PORT` | 8080 | HTTP 端口 |
| `DATA_DIR` | ./data | 数据目录（本地盘，禁网络盘） |
| `APP_BACKUP_DIR` | ./backups | 备份目录 |
| `APP_KEEP_BACKUP` | 30 | 备份保留份数 |
| `APP_AUTO_BACKUP` | 1 | 每日 03:00 自动备份 |
| `APP_SECURE_COOKIE` | 空 | HTTPS 时设 1 |

---

## 6. 开发约定

### 6.1 代码组织

- **按特性分层**：每个业务模块放在 `src/features/<name>/` 下
- **Vue 组件**：PascalCase 命名，`.vue` 后缀
- **TS 模块**：camelCase 命名
- **单文件上限**：Vue 组件 ≤ 500 行，超出则拆分

### 6.2 HTTP 客户端

封装在 `src/lib/http.ts`：

```typescript
// 自动处理 401 跳转
// 统一响应解析 { data: ... } | { error: { code, message } }
export async function api<T>(path: string, options?: RequestInit): Promise<T>
```

### 6.3 类型定义

前后端共享类型在 `src/types/api.ts`：

```typescript
export interface Category { id: number; name: string; level: number; /* ... */ }
export interface Party { id: number; name: string; types: string[]; /* ... */ }
export interface Receivable { id: number; partyId: number; /* ... */ }
export interface Transaction { id: number; txnDate: string; direction: string; /* ... */ }
// ...
```

### 6.4 金额处理

- 存储：`amount_cents: number`（整数分）
- 显示：`fmt(cents)` 函数 → `¥ 1,234.00`
- 输入：用户输入元，提交前 `Math.round(parseFloat(val) * 100)` 转为分

---

## 7. 已知偏离与待办

| 项 | 状态 |
|------|------|
| Vite 实装 6.4.3 vs 设计 8.2.1 | 待用户拍板（可暂不处理） |
| Docker/embed 一体化部署 | 骨架占位，未完成 |
| 生产静态资源托管（单二进制） | v0.5.0 已实现（Go 内嵌），后续版本开发态使用 Mock API |
| PWA 收尾、启动自检（网络盘检测） | 设计有、未实现 |
| 变更历史查看覆盖范围 | 流水/转账/资金划转已可用；应收核销/科目自身留痕未全部做成界面 |
| Mock API 与 Go 后端功能差异 | 快速记账/投资管理/流转管理/合同管理仅在 Mock 中实现，Go 后端未同步 |

---

## 8. 变更记录

| 日期 | 说明 |
|------|------|
| 2026-09-06 | 初稿，对齐 v0.8.0 代码实况，反映完整技术栈与模块结构 |