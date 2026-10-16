// Command server 是集体台账后端入口：加载配置、打开数据库、执行迁移、启动 HTTP 服务。
package main

import (
	"database/sql"
	"embed"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/auth"
	"jititaizhang/server/internal/backup"
	"jititaizhang/server/internal/category"
	"jititaizhang/server/internal/changelog"
	"jititaizhang/server/internal/contract"
	"jititaizhang/server/internal/export"
	"jititaizhang/server/internal/fundmove"
	"jititaizhang/server/internal/onboarding"
	"jititaizhang/server/internal/platform"
	"jititaizhang/server/internal/receivable"
	"jititaizhang/server/internal/settings"
	"jititaizhang/server/internal/summary"
	"jititaizhang/server/internal/transaction"
	
)

// webFS 内嵌前端构建产物（scripts/build.sh 先把 web/dist 复制到 ./web）。
//
//go:embed all:web
var webFS embed.FS

// app 持有随恢复热替换的可变状态（数据库、会话服务、路由）。
type app struct {
	mu      sync.Mutex
	cfg     platform.Config
	db      *sql.DB
	authSvc *auth.Service
	router  *gin.Engine
	srv     *http.Server
}

func main() {
	cfg := platform.Load()
	a := &app{cfg: cfg}

	db, err := openDB(cfg)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	if err := platform.Migrate(db); err != nil {
		db.Close()
		log.Fatalf("执行迁移失败: %v", err)
	}

	a.db = db
	a.authSvc = auth.NewService(db)
	a.router = buildRouter(a)
	a.srv = &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           a.router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	if cfg.AutoBackup {
		repo := backup.New(db, cfg.DBPath(), cfg.BackupDir, cfg.KeepBackup)
		go runDailyBackup(repo, a)
	}

	log.Printf("集体台账后端启动，监听 :%s（数据目录 %s，备份目录 %s）", cfg.Port, cfg.DataDir, cfg.BackupDir)
	if err := a.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("服务异常退出: %v", err)
	}
}

// buildRouter 依据当前 db/authSvc 组装路由（恢复后调用以切换新连接）。
//
// 路由按权限分四组（依据 docs/27 §5.2）：
//   - public：登录/注册/登出/忘记密码，无需登录；
//   - read  ：已登录即可（含 viewer），仅 GET：/api/me、导出、修改自己的密码；
//   - biz   ：业务读写，viewer 的非 GET 请求由 RequireWrite 统一拦截；
//   - admin ：owner/admin 专属（备份管理、操作日志查看、账号管理、读设置）；
//   - owner ：owner 专属最高危操作（恢复备份、清空日志、系统重置）。
func buildRouter(a *app) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/api/health", func(c *gin.Context) {
		platform.OK(c, gin.H{"status": "ok"})
	})
	authHandler := auth.NewHandler(a.authSvc)
	authHandler.Register(r) // 公开：login/register/logout/reset-password/registration-status

	// 只读组：所有已登录账号（含 viewer）均可访问。
	read := r.Group("", auth.RequireAuth(a.authSvc))
	meHandler := func(c *gin.Context) {
		id, _ := auth.CurrentUserID(c)
		orgID, ok := auth.CurrentOrgID(c)
		role, _ := auth.CurrentRole(c)
		var orgName string
		var onboarded int
		if ok {
			_ = a.db.QueryRow(`SELECT name, onboarded FROM org WHERE id = ?`, orgID).Scan(&orgName, &onboarded)
		}
		platform.OK(c, gin.H{
			"userID": id, "orgID": orgID, "orgName": orgName,
			"role": role, "onboarded": onboarded == 1,
		})
	}
	read.GET("/api/me", meHandler)
	read.GET("/api/auth/me", meHandler)
	authHandler.RegisterAuthed(read)       // PUT /api/auth/password（自服务，viewer 可用）
	export.NewHandler(a.db).Register(read) // GET /api/export（只读，viewer 可导出）

	// 业务读写组。
	biz := r.Group("", auth.RequireAuth(a.authSvc), auth.RequireWrite())
	category.NewHandler(a.db).Register(biz)
	transaction.NewHandler(a.db).Register(biz)
	summary.NewHandler(a.db).Register(biz)
	fundmove.NewHandler(a.db).Register(biz)
	contract.NewHandler(a.db).Register(biz)
	receivable.NewHandler(a.db).Register(biz)
	onboarding.NewHandler(a.db).Register(biz)
	settings.NewHandler(a.db).RegisterWrite(biz)

	// 管理组：owner / admin。
	admin := r.Group("", auth.RequireAuth(a.authSvc), auth.RequireRole(auth.RoleAdmin, auth.RoleOwner))
	changelog.NewHandler(a.db).RegisterRead(admin)
	settings.NewHandler(a.db).RegisterRead(admin)
	authHandler.RegisterUsers(admin)

	// 所有者专属组：最高危操作。
	owner := r.Group("", auth.RequireAuth(a.authSvc), auth.RequireRole(auth.RoleOwner))

	// 系统重置：清空所有业务数据与账号，保留预置科目结构，退出到注册页。
	owner.POST("/api/system/reset", func(c *gin.Context) {
		var tables = []string{
			"transaction", "receivable", "receivable_detail",
			"distribution_532", "reinvest_allocation", "contract",
			"operation_log", "changelog",
		}
		for _, t := range tables {
			_, _ = a.db.Exec("DELETE FROM " + t)
		}
		// 重置科目余额
		_, _ = a.db.Exec("UPDATE category SET balance_cents = 0, opening_balance_cents = 0 WHERE level = 2")
		// 重置设置
		_, _ = a.db.Exec("UPDATE settings SET bank_opening_balance_cents = 0, reinvest_ratio_bps = 0")
		// 删除所有非预置单位（自动创建的 L2 科目由单位删除联动清除）
		_, _ = a.db.Exec("DELETE FROM party WHERE preset = 0")
		// 清除会话
		_, _ = a.db.Exec("DELETE FROM session")
		// 删除所有用户（包括 owner），强制重新注册
		_, _ = a.db.Exec("DELETE FROM user")
		_, _ = a.db.Exec("DELETE FROM org")
		platform.OK(c, gin.H{"ok": true, "message": "系统已重置，请重新注册"})
	})

	changelog.NewHandler(a.db).RegisterClear(owner)

	registerBackupRoutes(admin, owner, a)
	registerStatic(r)
	return r
}

// registerStatic 托管内嵌前端：非 /api 请求返回静态文件，未命中回退 index.html（SPA hash 路由）。
// assets/ 下的产物一律带内容哈希，未命中必须 404：回退 index.html 会让浏览器把 HTML 当 JS 模块加载，
// 严格 MIME 校验直接拒绝，懒加载路由随之静默中止（换镜像后旧页面点 tab 没反应的成因）。
func registerStatic(r *gin.Engine) {
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "接口不存在"}})
			return
		}
		name := strings.TrimPrefix(p, "/")
		if name == "" || name == "index.html" {
			name = "index.html"
		}
		data, err := webFS.ReadFile("web/" + filepath.ToSlash(name))
		if err != nil {
			if strings.HasPrefix(name, "assets/") {
				c.String(http.StatusNotFound, "asset not found")
				return
			}
			data, err = webFS.ReadFile("web/index.html")
			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}
			name = "index.html"
		}
		ct := mime.TypeByExtension(filepath.Ext(name))
		if ct == "" {
			ct = "text/plain; charset=utf-8"
		}
		if name == "index.html" {
			c.Header("Cache-Control", "no-cache")
		} else {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		}
		c.Data(http.StatusOK, ct, data)
	})
}

// registerBackupRoutes 挂载备份接口：
//   - 列表 / 立即备份 / 下载 / 删除 → admin 组（owner/admin 可管备份）；
//   - 一键恢复（覆盖当前全部数据）→ owner 组（最高危操作）。
func registerBackupRoutes(admin, owner gin.IRouter, a *app) {
	repo := backup.New(a.db, a.cfg.DBPath(), a.cfg.BackupDir, a.cfg.KeepBackup)

	admin.GET("/api/backups", func(c *gin.Context) {
		items, err := repo.List()
		if err != nil {
			platform.Fail(c, http.StatusInternalServerError, "BACKUP_LIST_FAILED", "读取备份列表失败")
			return
		}
		platform.OK(c, gin.H{"items": items})
	})

	admin.POST("/api/backups", func(c *gin.Context) {
		info, err := repo.Create()
		if err != nil {
			platform.Fail(c, http.StatusInternalServerError, "BACKUP_CREATE_FAILED", "备份失败: "+err.Error())
			return
		}
		platform.OK(c, info)
	})

	owner.POST("/api/backups/:id/restore", func(c *gin.Context) {
		id := filepath.Base(c.Param("id"))
		backupPath := filepath.Join(a.cfg.BackupDir, id)
		info, err := os.Stat(backupPath)
		if err != nil || info.IsDir() {
			platform.Fail(c, http.StatusNotFound, "BACKUP_NOT_FOUND", "备份不存在")
			return
		}

		if err := a.restore(backupPath); err != nil {
			log.Printf("恢复失败: %v", err)
			platform.Fail(c, http.StatusInternalServerError, "RESTORE_FAILED", "恢复失败，请检查数据目录后重启服务")
			return
		}

		// 恢复后强制重新登录（旧会话属于恢复前的库）
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(auth.CookieName, "", -1, "/", "", false, true)
		platform.OK(c, gin.H{"ok": true, "message": "恢复成功，请重新登录"})
	})

	admin.GET("/api/backups/:id/download", func(c *gin.Context) {
		id := filepath.Base(c.Param("id"))
		backupPath := filepath.Join(a.cfg.BackupDir, id)
		info, err := os.Stat(backupPath)
		if err != nil || info.IsDir() {
			platform.Fail(c, http.StatusNotFound, "BACKUP_NOT_FOUND", "备份不存在")
			return
		}
		// FileAttachment 自动写入 Content-Disposition: attachment，并流式发送
		c.FileAttachment(backupPath, id)
	})

	admin.DELETE("/api/backups/:id", func(c *gin.Context) {
		id := filepath.Base(c.Param("id"))
		items, err := repo.List()
		if err != nil {
			platform.Fail(c, http.StatusInternalServerError, "BACKUP_LIST_FAILED", "读取备份列表失败")
			return
		}
		if err := repo.DeleteFile(id, items); err != nil {
			if err == backup.ErrLatestProtected {
				platform.Fail(c, http.StatusBadRequest, "BACKUP_LATEST_PROTECTED", "最近一份备份不可删除")
				return
			}
			platform.Fail(c, http.StatusInternalServerError, "BACKUP_DELETE_FAILED", err.Error())
			return
		}
		platform.OK(c, gin.H{"ok": true})
	})
}

// restore 热恢复：独占锁内关闭旧库 → 备份覆盖 → 重开库并迁移 → 重建路由。
func (a *app) restore(backupPath string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	oldDB := a.db
	oldDB.Close()

	dbPath := a.cfg.DBPath()
	staged := dbPath + ".pre-restore"
	// 先把旧库挪到暂存位，失败可回滚（数据库与备份通常同盘内 rename，开销极小）
	if err := os.Remove(staged); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(dbPath, staged); err != nil {
		return err
	}
	for _, p := range []string{dbPath + "-wal", dbPath + "-shm"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	rollback := func(cause error) error {
		_ = os.Remove(dbPath)
		_ = os.Rename(staged, dbPath)
		if ndb, err := openDB(a.cfg); err == nil {
			a.db = ndb
			a.authSvc = auth.NewService(ndb)
			a.router = buildRouter(a)
			a.srv.Handler = a.router
		}
		return cause
	}

	if err := copyFile(backupPath, dbPath); err != nil {
		return rollback(err)
	}

	ndb, err := openDB(a.cfg)
	if err != nil {
		return rollback(err)
	}
	if err := platform.Migrate(ndb); err != nil {
		ndb.Close()
		return rollback(err)
	}

	_ = os.Remove(staged)

	a.db = ndb
	a.authSvc = auth.NewService(ndb)
	a.router = buildRouter(a)
	a.srv.Handler = a.router
	return nil
}

// openDB 打开并迁移数据库（失败返回 error；成功由调用方负责 Close）。
func openDB(cfg platform.Config) (*sql.DB, error) {
	db, err := platform.Open(cfg.DBPath())
	if err != nil {
		return nil, err
	}
	if err := platform.Migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := out.ReadFrom(in); err != nil {
		return err
	}
	return out.Sync()
}

// runDailyBackup 每日 03:00（本机时区）自动备份一次，循环调度。
func runDailyBackup(repo *backup.Repo, a *app) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("自动备份协程退出: %v", r)
		}
	}()
	for {
		wait := time.Until(nextBackupTime(time.Now()))
		if wait > 0 {
			time.Sleep(wait)
		}
		a.mu.Lock()
		_, err := repo.Create()
		a.mu.Unlock()
		if err != nil {
			log.Printf("自动备份失败: %v", err)
		} else {
			log.Printf("自动备份完成（%s）", a.cfg.BackupDir)
		}
	}
}

// nextBackupTime 返回下一个本地 03:00。
func nextBackupTime(now time.Time) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}
