# 合同管理模块开发与修复 — 完整会话记录

> 日期：2026-09-06
> 涉及模块：往来单位 → 合同管理
> 前序会话因上下文丢失而交接，本文档整合两段会话的全部工作

---

## 一、用户原始需求（按时间顺序）

### 阶段一：合同管理页面搭建

1. **修复合同上传功能**（全局上传按钮 + 单位级上传按钮）
2. **实现合同查看功能**，支持 PDF / 图片 / 文本 / Office 文件
3. **解决 UI 问题**：下拉框选择所属单位、预览窗口尺寸
4. **确保跨浏览器文档渲染兼容**

### 阶段二：Bug 修复与收尾

5. **关闭按钮被遮罩** — 查看合同后的关闭按钮点击无反应
6. **恢复登录状态和 mock 数据** — 回滚调试期间引入的临时改动

---

## 二、最终代码架构

### 2.1 路由结构（web/src/router/index.ts）

```
/contacts                     → ContactsLayout.vue（子 Tab 导航壳）
  ├── /contacts/overview      → 往来概览
  ├── /contacts/parties       → 单位列表
  ├── /contacts/receivables/dividend → 应收投资收益
  ├── /contacts/receivables/rent    → 应收土地流转费
  ├── /contacts/receivables/service → 应收管理费
  └── /contacts/contracts    → 合同管理 ← 本次新增
```

### 2.2 依赖（package.json 新增）

```json
{
  "mammoth": "^1.6.0",     // .docx 解析为 HTML
  "xlsx": "^0.18.5"        // .xlsx 解析为表格
}
```

### 2.3 核心文件清单

| 文件 | 作用 |
|---|---|
| `web/src/features/contacts/pages/ContractsPage.vue` | 合同管理主页面（上传、列表、分组、预览、删除） |
| `web/src/features/contacts/ContactsLayout.vue` | 往来单位模块 Tab 导航容器 |
| `web/src/router/index.ts` | 路由注册 |
| `server/mock.js` | Mock API + 种子数据 |
| `web/src/features/auth/store.ts` | 登录状态 store（恢复） |

---

## 三、阶段一：上传与预览功能实现

### 3.1 文件上传方案

**决策：用 base64 data URL，不用 FormData**

后端 mock server 不支持 multipart/form-data 解析，改用：

```typescript
function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = reject
    reader.readAsDataURL(file)
  })
}

async function submitUpload() {
  const fileData = await fileToBase64(chosenFile.value)
  await api.post('/contracts', {
    partyId: pid,
    fileName: chosenFile.value.name,
    fileSize: chosenFile.value.size,
    mimeType: chosenFile.value.type || 'application/octet-stream',
    contractTitle: chosenFile.value.name,
    fileData,            // ← 直接塞 data URL
  })
}
```

### 3.2 合同数据模型（mock.js）

```javascript
// 种子合同，fileData 是完整 base64 data URL
CONTRACTS = [{
  id: 1,
  partyId: 1,
  fileName: '土地流转合同-2025.pdf',
  fileSize: 603,
  mimeType: 'application/pdf',
  contractTitle: '土地流转合同-2025',
  contractDate: '2025-01-15',
  expiresAt: '2027-01-15',
  uploadedAt: '2025-01-15T10:00:00Z',
  fileData: 'data:application/pdf;base64,JVBERi0xLjQK...（完整合法的 PDF）',
}]
```

**注意**：之前种子 PDF 是残缺的，被 Vite 加载后 iframe 渲染空白。已替换为带完整 xref table 的合法 PDF。

### 3.3 Mock API 端点

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/contracts` | 列表，**不含** fileData（减小 payload） |
| GET | `/api/contracts/:id` | 详情，**含** fileData（预览/下载专用） |
| POST | `/api/contracts` | 上传，接收 base64 body |
| DELETE | `/api/contracts/:id` | 删除 |

关键实现：GET 列表时剥离 fileData，GET 详情时返回完整对象。

### 3.4 预览类型判定

```typescript
function detectPreviewKind(mime: string, fileName: string):
  'text' | 'image' | 'word' | 'excel' | 'office-download' | 'iframe' {
  const ext = fileExt(fileName)
  if (mime.startsWith('text/')) return 'text'
  if (mime.startsWith('image/')) return 'image'
  if (mime === 'application/vnd.openxmlformats-officedocument.wordprocessingml.document' || ext === 'docx') return 'word'
  if (mime === 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' || ext === 'xlsx') return 'excel'
  if (OFFICE_MIMES.includes(mime) || OFFICE_EXT.includes(ext)) return 'office-download'
  return 'iframe'  // PDF 等浏览器原生支持的
}
```

| 类型 | 渲染方式 | 限制 |
|---|---|---|
| text/* | `<pre>` 直接渲染 | — |
| image/* | `<img src="data:...">` | — |
| .docx | mammoth → HTML | 支持，效果良好 |
| .xlsx | XLSX → `<table>` | 支持，多 sheet 可切换 |
| .doc / .xls / .ppt / .pptx | 下载提示 | 浏览器无法原生渲染 |
| PDF | `<iframe src="data:...">` | 需要合法 PDF 才能渲染 |

### 3.5 预览弹窗：放弃 van-dialog，改用手写 div + Teleport

Vant 的 van-dialog 在全屏预览场景下有三大坑：
1. 强制渲染 `.van-overlay` 遮罩层，即使 `:overlay="false"` 也有残留
2. 带 `transform` 过渡动画，会干扰 iframe 的 stacking context
3. `fullscreen` prop 的尺寸计算在移动端不准

**解决方案**：`<Teleport to="body">` 包裹的手写 fixed div：

```vue
<Teleport to="body">
  <div v-if="previewVisible" class="preview-dialog" tabindex="-1">
    <div class="preview-header">
      <span class="preview-title">{{ previewFileName }}</span>
      <button class="preview-close-btn" @click="previewVisible = false">关闭</button>
    </div>
    <div class="preview-body">
      <!-- 按 previewKind 渲染具体内容 -->
    </div>
  </div>
</Teleport>
```

---

## 四、阶段二：关闭按钮被遮罩（两个叠加 Bug）

### 4.1 Bug A：`<style>` 标签不匹配 — Vite 编译直接 500

**现象**：点击"查看"按钮后 `.preview-dialog` 根本没渲染，或渲染了但样式全丢。

**根因**：`ContractsPage.vue` 里只有一个 `<style scoped>` 开标签，却有**两个** `</style>` 闭合标签。Line 528 的 `</style>` 提前关闭了 scoped 块，后面的 `.preview-dialog` CSS（Line 530+）变成了没有开标签的"孤儿 CSS"。

```
Line 401: <style scoped>          ← 唯一的开标签
...（大量 scoped 样式）...
Line 528: </style>                ← 提前关闭了 scoped
Line 530: .preview-dialog { ... } ← 这些 CSS 变成了孤儿
...
Line 571: </style>                ← 多余的闭合标签
```

**Vite 报错**：
```
Internal server error: Invalid end tag.
Plugin: vite:vue
File: ContractsPage.vue:571:1
```

**修复**：在 Line 528 的 `</style>` 后插入一个新的**非 scoped** `<style>` 块：

```vue
</style>                    <!-- 原 scoped 块正常关闭 -->

<!-- 新增：非 scoped，因为 preview-dialog Teleport 到 body，scoped 对它无效 -->
<style>
.preview-dialog { ... }
.preview-close-btn { ... }
.preview-iframe { ... }
</style>
```

### 4.2 Bug B：iframe/img/text 缺 `flex: 1` — 内容区只有 150px 高

**现象**：弹窗渲染出来了，但 iframe 高度只有 ~150px（浏览器默认），header 60px 被挤在顶部，看起来像"按钮被遮罩"。

**根因**：`.preview-body` 是 `flex-direction: column`，其直接子元素（iframe / img / pre）没有 `flex: 1`，不会自动填满剩余空间。

对比：
| 类名 | 原样式 | 修复后 |
|---|---|---|
| `.preview-text` | 有 padding/overflow，**无 flex** | `flex: 1` + 原有样式 |
| `.preview-img` | 只有 `object-fit/background` | `flex: 1; display: block` |
| `.preview-iframe` | 只有 `border/background` | `flex: 1; width: 100%; height: 100%` |
| `.preview-docx` | ✅ 已有 `flex: 1` | 无需改动 |
| `.preview-excel` | ✅ 已有 `flex: 1` | 无需改动 |

**修复后**（iframe 预览 PDF 为例）：
```
修复前: iframe rect = (0, 60, 1116, 150)   ← 只有 150px
修复后: iframe rect = (0, 60, 1116, 778)   ← 正确填满剩余空间
```

### 4.3 两个 Bug 的叠加关系

```
Bug A（编译失败）  →  Vite 500  →  浏览器拿到上一次成功版本的缓存
Bug B（flex 缺失） →  iframe 只有 150px 高  →  header 在顶部，视觉上像被遮罩
```

两个问题同时存在，让排查非常困难 —— 修了 A 才发现 B，修了 B 才看到正确的布局。

---

## 五、临时改动与恢复

为了方便浏览器自动化测试，调试期间做了两处"绕开登录"的临时改动。修复完成后已恢复：

| 文件 | 临时改动 | 恢复内容 | 恢复时间 |
|---|---|---|---|
| `web/src/features/auth/store.ts:10` | `isLoggedIn = ref(true)` | `isLoggedIn = ref(false)` | 2026-09-06 |
| `server/mock.js:223-227` | 跳过鉴权、自动补发 session cookie | 恢复 401 拒绝未登录请求 | 2026-09-06 |

Mock server 重启后生效。

---

## 六、完整修改文件清单

| # | 文件 | 改动类型 | 内容 |
|---|---|---|---|
| 1 | `web/src/features/contacts/pages/ContractsPage.vue` | **新增** | 合同管理完整页面（上传对话框、分组列表、预览弹窗、删除） |
| 2 | `web/src/features/contacts/ContactsLayout.vue` | 新增 | 往来单位子 Tab 导航壳 |
| 3 | `web/src/router/index.ts` | 修改 | 新增 `/contacts/contracts` 路由 |
| 4 | `server/mock.js` | 修改 | 新增 CONTRACTS 种子数据 + 4 个 API 端点；恢复鉴权逻辑 |
| 5 | `web/src/features/auth/store.ts` | 临时+恢复 | `isLoggedIn` 默认值先改 true 再改回 false |

---

## 七、技术决策速查

### 7.1 为什么上传用 base64 不用 FormData？

| 方案 | 优点 | 缺点 |
|---|---|---|
| FormData | 浏览器原生，内存友好 | mock server 不支持 multipart 解析，需额外依赖 |
| **base64 data URL** | 纯 JSON，无需改动 mock 解析逻辑 | 体积膨胀 ~33%，大文件慢 |

当前 mock 阶段用 base64 足够，后续接真实后端时可切换到 FormData。

### 7.2 为什么放弃 van-dialog 手写 Teleport？

| 方案 | 全屏可用性 | 遮罩控制 | iframe stacking |
|---|---|---|---|
| van-dialog + `fullscreen` | ❌ 尺寸不准 | ❌ 总有残留 overlay | ❌ transform 干扰 |
| **Teleport + 手写 fixed div** | ✅ 100vw/100vh | ✅ 完全可控 | ✅ 无 transform |

### 7.3 scoped vs 非 scoped 的选择规则

- **组件内部 DOM**（`.page-header`、`.cp-group`、`.preview-body` 等）→ `<style scoped>`
- **Teleport 到 body 的 DOM**（`.preview-dialog`）→ `<style>` 非 scoped

规则：看 CSS 作用在哪个 DOM 节点上。scoped 的哈希只会被加在 Vue 模板内渲染的元素上，Teleport 出去的不会带哈希。

### 7.4 PDF 种子数据的陷阱

之前用伪造的 PDF 头部（`%PDF-1.4` + 几个 object），浏览器 iframe 渲染空白。**浏览器 PDF 查看器要求完整合法的 PDF 结构**（catalog → pages → page tree → xref table → trailer）。修复时替换为完整的 xref table。

---

## 八、验证记录

### 阶段一验证

| 检查项 | 结果 |
|---|---|
| 上传 PDF → 列表显示 | ✅ |
| 上传后文件信息正确（名称/大小/MIME） | ✅ |
| 按单位分组显示 | ✅ |
| 全局"＋上传合同"按钮 | ✅ |
| 单位行内"上传到此单位"按钮 | ✅ |
| 所属单位下拉选择 | ✅ 原生 `<select>` |
| 查看 PDF → iframe 渲染 | ✅ |
| 查看 .docx → mammoth → HTML | ✅ |
| 查看 .xlsx → XLSX → 表格 + sheet 切换 | ✅ |
| 查看 .doc/.xls → 下载提示 | ✅ |

### 阶段二验证

| 检查项 | 结果 |
|---|---|
| Vite 编译 ContractsPage.vue | ✅ HTTP 200，无报错 |
| PDF iframe 高度 | ✅ 778px（正确填满 preview-body 剩余空间） |
| preview-dialog z-index: 99999，全屏覆盖 | ✅ `(0,0) → (1116, 838)` |
| preview-header `flex-shrink: 0` | ✅ 高度 60px，不被内容挤压 |
| 关闭按钮无遮罩覆盖 | ✅ `elementFromPoint(1064, 29)` 返回 BUTTON |
| 点击关闭按钮 | ✅ preview-dialog 正确从 body 移除 |
| ESC 键关闭 | ✅ |
| 上传对话框取消按钮 | ✅ z-index 2001，正常 |

---

## 九、后续可跟进项

- [ ] 给 Word/Excel 预览加 loading 骨架屏（mammoth/XLSX 解析有 ~200-500ms 延迟）
- [ ] 用 `vue-pdf` 或 `pdfjs-dist` 替代 iframe PDF，支持缩放 / 翻页 / 页码定位
- [ ] 上传对话框同样是 van-dialog，如果用户反馈遮罩问题，可统一迁移到 Teleport 方案
- [ ] 大文件（> 5MB）上传时 base64 转换有明显卡顿，需加进度条提示
- [ ] mock.js 的鉴权逻辑和真实后端 auth middleware 对齐，联调前先做接口契约检查
- [ ] 种子数据：当前只有 1 条 PDF 合同，可补充 .docx / .xlsx / .txt / .jpg 各一条，覆盖所有预览路径
- [ ] ESLint / stylelint 规则：`<style>` 标签匹配检查，避免类似 Bug A 再次出现
