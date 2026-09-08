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

	// _time_format=sqlite：让 modernc.org/sqlite 用统一的 SQLite 时间文本格式
	// （2006-01-02 15:04:05）往返序列化 time.Time，否则 DATETIME 列读出为 string，
	// Scan 到 time.Time 会报 "unsupported Scan"（曾导致登录/会话鉴权整体失效）。
	dsn := dbPath + "?_time_format=sqlite"
	db, err := sql.Open("sqlite", dsn)
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

// tzCN 固定东八区（Asia/Shanghai，无夏令时）。用 FixedZone 而非 LoadLocation，
// 避免静态编译产物缺时区数据库时 LoadLocation 失败。
var tzCN = time.FixedZone("Asia/Shanghai", 8*60*60)

// Now 提供统一的写入时间戳（北京时间，UTC+8）。
func Now() time.Time {
	return time.Now().In(tzCN)
}
