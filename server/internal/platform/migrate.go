package platform

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate 按文件名升序执行尚未应用的迁移。
//
// 约定：
//   - 迁移文件只增不改（版本化，见 03-design 3.2 目录职责）
//   - 每个迁移在单个事务内执行，失败即回滚
//   - 已执行版本记录在 schema_migrations 表
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at DATETIME NOT NULL
	)`); err != nil {
		return fmt.Errorf("创建 schema_migrations 失败: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("读取迁移目录失败: %w", err)
	}

	var files []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, f := range files {
		applied, err := isApplied(db, f)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + f)
		if err != nil {
			return fmt.Errorf("读取迁移 %s 失败: %w", f, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("开始迁移事务失败: %w", err)
		}
		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("执行迁移 %s 失败: %w", f, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(?, ?)`,
			f, Now()); err != nil {
			tx.Rollback()
			return fmt.Errorf("记录迁移 %s 失败: %w", f, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("提交迁移 %s 失败: %w", f, err)
		}
	}
	return nil
}

func isApplied(db *sql.DB, version string) (bool, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("查询迁移状态失败: %w", err)
	}
	return n > 0, nil
}
