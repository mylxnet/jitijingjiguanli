# 26 · 静态资源缓存口径与手机端合同 PDF 预览（v0.24.1）

日期：2026-09-21　状态：已实现并本机取证，**未实机复核**

## 1 两条现象，两个根因

| 现象 | 根因 | 结论 |
|---|---|---|
| 手机端点「往来」没反应、某页永远打不开 | **不是代码 bug**。客户端留着升级前的旧 `index.html`，其引用的旧哈希 chunk 在新镜像里已不存在；而内嵌前端对未命中路径一律回退 `index.html` → 请求 JS 拿到 `200 + text/html` → 严格 MIME 校验拒绝 → 懒加载 `import()` 抛错 → Vue Router 静默中止跳转 | 服务端补 404 与缓存头（甲） |
| 手机端「合同管理」里 PDF 打不开、弹窗空白 | 预览把 `data:application/pdf;base64,…` 直接塞 `<iframe>`。移动端内核不在 iframe 里渲染 PDF：安卓 Chrome 无内置 PDF 查看器、iOS Safari 只在顶层导航用 QuickLook、微信内直接白屏。桌面 Chromium 能显示，所以本机看是"好的" | pdf.js 自渲染 + blob 下载兜底（乙） |

线上取证（`http://192.168.31.19:1133`，v0.24.0）：请求一个不存在的 chunk 得 `200 text/html 742B`，即上表第一行；本机与线上用 390×844 手机壳点「往来」都能正常出数，反证不是路由/接口问题。

## 2 改动清单

### 2.1 后端 `server/cmd/server/main.go` · `registerStatic`（+11 行）

| 请求 | 改前 | 改后 |
|---|---|---|
| `/assets/<缺失>.js` | 200 + `text/html`（index.html） | **404** `asset not found`（`text/plain`） |
| `/`、`/index.html` | 200，无缓存头 | 200 + `Cache-Control: no-cache` |
| `/assets/<哈希>.js\|.css\|.mjs` | 200，无缓存头 | 200 + `Cache-Control: public, max-age=31536000, immutable` |
| 其余非 `/api/` 路径 | 回退 index.html | 不变（hash 路由不受影响） |

回归测试：`server/cmd/server/main_test.go` → `TestStaticAsset404AndCacheHeaders`（用 `httptest` 起真服务，断言四类响应；反向取证：临时关掉 404 分支即 `FAIL main_test.go:57`）。

### 2.2 前端 `web/src/features/contacts/pages/ContractsPage.vue`（+118 / −9）

- `detectPreviewKind` 新增 `'pdf'` 分支（`application/pdf` 或 `.pdf`），不再落到 iframe。
- 新增 `renderPdf()`：`await import('pdfjs-dist')` 懒加载；worker 用 `pdfjs-dist/build/pdf.worker.min.mjs?url` 由 Vite 打包成**同源产物**（不走 CDN）；按容器宽度 × `min(devicePixelRatio, 2)` 逐页画 `<canvas class="pdf-page">`，弹窗内滚动。
- 新增 blob URL 链路：`makePreviewBlobUrl / revokePreviewBlobUrl / closePreview`。iframe 的 `src`、下载链接的 `href` 一律优先用 blob（`previewBlobUrl || previewFileData`），关闭按钮 / Esc / 解析失败 / 组件卸载都会 `revokeObjectURL` 并清空 canvas。
- PDF 渲染失败**不关弹窗**：顶部提示「无法在线渲染，请下载后查看」，下载按钮仍在。
- ⚠️ 关键实现细节：PDF 分支**不能**走 `previewLoading`，那条分支会整块替换掉 `.preview-body`，canvas 容器 `pdfPagesRef` 就不存在了；渲染中状态由 `pdfBarText`（正在渲染… / 共 N 页 / 失败提示）承担。

### 2.3 体积代价

`pdf-*.js` 365 KB（懒加载 chunk，只有打开 PDF 才下载）+ `pdf.worker.min-*.mjs` 1.37 MB + `ContractsPage-*.js` 861 KB。换来的是移动端可读 PDF；不接受体积则回退为「只做下载兜底」。

## 3 验证结果（本机 :8080，WSL 构建）

| 项 | 方式 | 结果 |
|---|---|---|
| 前端编译 | `npm run build`（`vue-tsc -b && vite build`） | 通过 |
| 产物 MIME/缓存 | curl 逐条 | `.mjs`→`text/javascript`+immutable；`.js`/`.css` 正常；缺失 chunk→`404 text/plain`；`/`→`no-cache` |
| Go 回归 | `go test ./cmd/server -run TestStatic` | PASS |
| 手机端合同 PDF | 390×844 同源 iframe，`#/` → 往来 → 合同管理 → 点 `使用手册.pdf`（669 KB / 8 页） | 弹窗全屏、`共 8 页`、8 张 canvas 均有实际像素（非白像素 2.7 万–6.9 万/页）、容器可滚动（scrollHeight 4172 / clientHeight 697） |
| 下载兜底 | fetch blob 链接 | 669,153 B（与库内 `file_size` 一致）、`%PDF-1.7`、文件名正确、`application/pdf` |
| 关闭清理 | 关闭后 fetch 旧 blob | `TypeError`（已 revoke）、canvas 归零、弹窗卸载 |

## 4 已知限制

1. **真机未验证**：本机只有桌面 Chromium。QA 期间发现该窗口 `document.hidden=true` 时 `requestAnimationFrame` 完全不触发（`setTimeout` 正常），pdf.js 的 `render()` 会**无声卡死**（不报错、console 无输出）。给手机壳 iframe 注入 rAF→setTimeout 垫片后跑通 —— 这只证明**代码逻辑正确**，安卓/微信前台真机行为仍需用户复核。
2. **非 PDF 类型未验证**：调试库只有 1 份 PDF 合同，图片/Word/Excel/老式 Office 走 blob 后的表现未实测。
3. **版本漂移自愈未做**：本次只堵住「HTML 被当 JS 加载」这一条。旧页面在**新镜像上线前**已打开、且用户不刷新仍会看到旧界面（现在至少会在控制台/网络里显式 404，不再静默）。
4. `server/cmd/server/web/assets.old-0920-235013/` 仍在内嵌目录里被 `//go:embed all:web` 打进二进制（旧产物可按 URL 命中），待清理授权。

## 5 运维注意

- 换镜像后**首次访问**才会拿到新 `index.html`（`no-cache` 保证不再被旧副本挡住）；已打开的旧标签页仍需刷新一次。
- 若将来把前端换成非哈希产物名，`immutable` 会把错误版本钉死一年 —— 保持 Vite 默认 `assets/[name]-[hash]` 命名。
