# 集体台账

面向村级集体经济组织的记账管理平台：管钱、管单位、管投资，流水只作废不修改，作废自动回滚全部联动。后端是单个 Go 二进制（`go:embed` 内嵌前端产物），数据库为 SQLite 单文件，桌面与手机访问同一套页面。

![集体台账首页看板](https://github.com/user-attachments/assets/e3f776bd-16b7-4ca6-9428-5906ebfd6667)

当前版本：**v0.25.0**（版本号唯一来源 `web/src/version.ts`）。

## 功能

- **建账向导**：Excel 模板导入或手工七步录入，自动建往来单位并同步科目；首次注册按集体经济常用科目预置账套。
- **快速记账 + 普通记账**：常用业务预置模板，已配好收支方向、入账科目与默认摘要；支出提交前校验科目余额，余额不足直接拦下。
- **流水只作废、不改不删**：作废自动回滚银行存款、科目余额、应收、投资等联动。
- **往来单位与合同**：单位资料维护、应收/已收/未收概览、合同附件上传与全屏预览（PDF 用 pdf.js 逐页渲染，Word/Excel 本地解析）。
- **应收管理**：土地流转费、管理费、投资收益三类应收按单位按年份查看与核销，含计提向导与坏账清单。
- **投资与流转台账**：投资公司与再投资单位的本金、已收收益、再投资跟踪；流转企业的土地面积与流转费收缴。
- **年度 532 分配**：走分配流程，方案一旦产生流水即锁定，不可再改。
- **看板与汇总**：银行存款、净资产、收益进度看板，多维汇总与下钻，数据可导出（`GET /api/export`，CSV）。
- **数据安全**：每日 03:00 自动备份、备份保留份数可配、本地下载与恢复、系统重置；最新一份备份受保护不可删。
- **账号与权限**：自助注册组织、会话 Cookie 鉴权；三角色权限（**所有者** / **高级用户** / **一般用户（只读）**），账号可停用与启用、密码可重置；写操作按角色收口（后端 `RequireAuth` + `RequireWrite` + `RequireRole` 三级中间件，前端菜单/按钮按角色隐藏）。

## 前置要求

- 运行 Releases 产物：Linux amd64 或 Windows amd64，无额外依赖（SQLite 驱动为纯 Go，`CGO_ENABLED=0`）。
- 源码构建：Go 1.27+、Node.js 22+。
- `DATA_DIR` 必须落在宿主机本地文件系统，禁止 NFS/SMB 挂载。数据库迁移只在进程启动时执行一次，替换或删除库文件后必须重启容器/进程。

## 安装

### A. 单文件可执行（GitHub Releases）

下载内嵌前端的 Linux amd64 产物 `jitizhang`（监听 8080）：

```bash
chmod +x jitizhang
DATA_DIR=/var/lib/jitai APP_BACKUP_DIR=/var/backups/jitai ./jitizhang
```

**Windows 桌面版 `jititaizhang.exe`**：内嵌前端 + SQLite，免安装任何运行时，双击即用。

- 数据保存在 **exe 同目录** 的 `data\`，备份在 `backups\`；换版本只替换 exe，账本原地保留（新版本启动时自动迁移）。
- 8080 被占用时自动顺延到 8081~8099；服务就绪后自动打开默认浏览器。
- 关闭黑色控制台窗口即停止服务。需要静默或换端口：`set APP_OPEN_BROWSER=0`、`set APP_PORT=9090`。
- 构建（需在 WSL / Linux 下）：`bash scripts/build-windows.sh` → `dist/windows/jititaizhang.exe`。

### B. Docker 预构建镜像（阿里云 ACR）

镜像 `registry.cn-hangzhou.aliyuncs.com/mylxnet/jitijingjiguanli:vX.Y.Z`（同时维护 `latest`），容器内约定 `DATA_DIR=/data`、`APP_BACKUP_DIR=/backups`：

```bash
docker login registry.cn-hangzhou.aliyuncs.com     # 凭据不入库
cp deploy/.env.example deploy/.env                 # .env 已 gitignore，按环境修改
docker compose -f deploy/docker-compose.hub.yml up -d
```

升级到某个版本只需改 `deploy/docker-compose.hub.yml` 的 image 标签，再 `pull` + `up -d`。请用 compose 启动：裸 `docker run` 漏掉挂载会丢数据目录。

**升级不会动你的账本**：只要 `DATA_DIR` 仍指向原数据目录（同一份 `jititaizhang.db`），新镜像启动时会自动执行新增的数据库迁移（`schema_migrations` 表逐条记录、事务内执行），老数据原地升级、无需导出导入。升级前建议先在设置页做一次备份。若挂载目录被换掉或删空，程序会当作全新库并从注册页开始。

### C. 从源码构建

```bash
cd web && npm ci && npm run build                        # vue-tsc -b && vite build
rm -rf ../server/cmd/server/web && mkdir -p ../server/cmd/server/web
cp -r dist/. ../server/cmd/server/web/                   # 必须先清空，否则旧 chunk 堆积
cd ../server && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../jitizhang ./cmd/server
```

镜像构建：`deploy/build-image.sh`，或按 `docs/04-dev-manual.md` §8.5 手工执行 `docker build --provenance=false --sbom=false -f deploy/Dockerfile.local`；`deploy/Dockerfile.release` 用 Releases 已构建的二进制打薄镜像，适合在 NAS 上不编译直接部署。

⚠️ `scripts/build.sh` 仍是早期 Windows exe 脚本（产物 `server/bin/jititaizhang.exe`），不在当前发布链路上。

### 开发环境

```bash
cd server && DATA_DIR=~/jitai-debug/data APP_AUTO_BACKUP=0 APP_PORT=8080 go run ./cmd/server
cd web && npm ci && npm run dev
```

改前端要让 8080 的单二进制看到，需按 C 的三步重建并同步内嵌目录，再重启后端。

## 使用

首次启动打开 `http://<host>:8080`。登录页点「注册组织」（仅当系统里还没有任何账号时出现）→ 填组织名、管理员账号、密码 → 自动登录并进入建账向导，二选一：下载模板填好单位后上传，或手工逐条录入。

日常记账走左侧「+」快速记账，选业务、填金额提交即可；记错了进流水页找到那一笔点「作废」。

```bash
curl http://127.0.0.1:8080/api/health
```

忘密码点登录页「忘记密码？」，系统里最早注册的账号会被重置为默认口令 `admin888`，登录后立即到「设置 → 修改密码」换掉。

## 配置

全部来自环境变量，默认值见 `server/internal/platform/config.go`。

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `APP_PORT` | `8080` | HTTP 监听端口 |
| `DATA_DIR` | `./data` | SQLite 数据目录，库文件名固定为 `jititaizhang.db` |
| `APP_BACKUP_DIR` | `./backups` | 备份目录，建议与数据不同盘 |
| `APP_KEEP_BACKUP` | `30` | 自动备份保留份数 |
| `APP_AUTO_BACKUP` | `1` | `1` 启用每日 03:00 自动备份，`0` 关闭 |
| `TZ` | `Asia/Shanghai` | 时区，固定东八区；决定自动备份 03:00 的触发时刻与日志时间 |
| `APP_SECURE_COOKIE` | 未设置 | 置 `1` 时会话 Cookie 加 `Secure`，仅在 HTTPS 前置之后使用 |

## 文档

| 文档 | 内容 |
| --- | --- |
| `docs/19-新用户快速入门.md` | 三分钟建账、日常记账、页面速览、常见业务该记哪 |
| `docs/05-user-manual.md` | 记账员操作手册与 FAQ |
| `docs/01-requirements.md` / `02-prd.md` / `03-design.md` | 需求、PRD、设计与各版本变更记录 |
| `docs/04-dev-manual.md` | 开发手册：模块划分、构建、部署链路、遗留项 |
| `docs/06-linkage-rules.md` | 流水作废的联动规则与验证口径 |
| `docs/20-备份设计.md` | 备份与恢复设计 |
| `docs/27-多用户与权限控制方案.md` | 多用户与权限控制：三角色模型、权限矩阵、路由分级与前端收口 |
| `deploy/CHANGELOG.md` | 版本变更日志 |

## 贡献

从 `main` 开分支，改完提 PR 回合 `main`。提交前跑 `cd web && npm run build` 与 `cd server && go test ./...`。

发布由维护者执行：更新 `web/src/version.ts`、`README.md` 的版本号、`deploy/CHANGELOG.md` 与 `deploy/docker-compose.hub.yml` 的镜像标签，推送 `main` 后打 `vX.Y.Z` 标签触发 `.github/workflows/release.yml`，产出 GitHub Releases 二进制；ACR 镜像另行在 WSL 构建推送。Gitee 远端只收 `main`，不打标签。

## 许可证

Distributed under the MIT License. 仓库当前未附带 `LICENSE` 文件，如需改用其他许可证请替换本节并补上文件。
