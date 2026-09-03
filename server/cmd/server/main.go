// Command server 是集体台账后端入口：加载配置、打开数据库、执行迁移、启动 HTTP 服务。
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/auth"
	"jititaizhang/server/internal/platform"
)

func main() {
	cfg := platform.Load()

	db, err := platform.Open(cfg.DBPath())
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()

	if err := platform.Migrate(db); err != nil {
		log.Fatalf("执行迁移失败: %v", err)
	}

	authSvc := auth.NewService(db)
	// 首次部署引导：仅在库中无用户且环境变量齐全时创建初始账号。
	if u, p := os.Getenv("ADMIN_USERNAME"), os.Getenv("ADMIN_PASSWORD"); u != "" && p != "" {
		if err := authSvc.EnsureInitialUser(u, p); err != nil {
			log.Fatalf("创建初始账号失败: %v", err)
		}
	}

	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/api/health", func(c *gin.Context) {
		platform.OK(c, gin.H{"status": "ok"})
	})
	auth.NewHandler(authSvc).Register(r)

	// 以下路由均需登录（v0.1.0 仅用于验证会话链路，业务接口在后续小节加入）
	authed := r.Group("/api", auth.RequireAuth(authSvc))
	authed.GET("/me", func(c *gin.Context) {
		id, _ := auth.CurrentUserID(c)
		platform.OK(c, gin.H{"userID": id})
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("集体台账后端启动，监听 :%s（数据目录 %s）", cfg.Port, cfg.DataDir)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("服务异常退出: %v", err)
	}
}
