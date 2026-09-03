// Package platform 提供跨特性的基础设施：配置加载、数据库连接、迁移执行。
package platform

import (
	"os"
	"path/filepath"
)

// Config 是应用运行配置。全部来自环境变量，带默认值，便于 Docker 部署。
type Config struct {
	Port       string // HTTP 监听端口
	DataDir    string // 数据目录（含 SQLite 文件）
	BackupDir  string // 备份目录（建议与数据不同盘，见 03-design R3/F7）
	KeepBackup int    // 备份保留份数
	AutoBackup bool   // 是否启用每日 03:00 自动备份
}

// Load 从环境变量读取配置，未设置时用默认值。
//
// 环境变量约定（与 deploy/.env.example 对齐）：
//   - APP_PORT        默认 8080
//   - DATA_DIR        默认 ./data
//   - APP_BACKUP_DIR  默认 ./backups
//   - APP_KEEP_BACKUP 默认 30
//   - APP_AUTO_BACKUP 默认 1（=1 启用每日 03:00 自动备份；=0 关闭）
func Load() Config {
	return Config{
		Port:       envOr("APP_PORT", "8080"),
		DataDir:    envOr("DATA_DIR", "./data"),
		BackupDir:  envOr("APP_BACKUP_DIR", "./backups"),
		KeepBackup: envInt("APP_KEEP_BACKUP", 30),
		AutoBackup: envOr("APP_AUTO_BACKUP", "1") != "0",
	}
}

// DBPath 返回 SQLite 数据库文件路径。
// ⚠️ 该目录必须位于容器宿主机的本地文件系统上，禁止放 NFS/SMB 挂载（见 03-design 5.5）。
func (c Config) DBPath() string {
	return filepath.Join(c.DataDir, "jititaizhang.db")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n := 0
	for _, ch := range v {
		if ch < '0' || ch > '9' {
			return def
		}
		n = n*10 + int(ch-'0')
	}
	if n <= 0 {
		return def
	}
	return n
}
