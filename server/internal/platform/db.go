package platform

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动（无 CGO，保持静态编译，见 03-design §5.7）
)

// Open 打开 SQLite 数据库并设置连接级 PRAGMA。
//
// 设计决策（见 03-design 4.x / 5.7）：
//   - journal_mode=WAL：读写并发更稳，配合单用户场景足够
//   - foreign_keys=ON：category 自关联等外键约束生效
//   - busy_timeout=5000ms：单写者模型下避免 SQLITE_BUSY 噪音
func Open(dbPath string) (*sql.DB, error) {
	if dir := filepath.Dir(dbPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("创建数据目录失败: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA foreign_keys=ON;",
		"PRAGMA busy_timeout=5000;",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("设置 %s 失败: %w", p, err)
		}
	}

	db.SetMaxOpenConns(1) // SQLite 单写者；单容器内串行访问最安全
	return db, nil
}

// Now 提供统一的写入时间戳（UTC）。
func Now() time.Time {
	return time.Now().UTC()
}
