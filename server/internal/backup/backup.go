// Package backup 提供 SQLite 一致性备份与恢复（F7/R3）。
//
// 备份方式：SQLite VACUUM INTO 生成一致性快照（03-design §5.6），快照与数据同库同版本，可直接覆盖恢复。
// 保留策略：保留最近 keep 份，超出删除最旧（每日自动备份场景下默认 30 天）。
package backup

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const filePrefix = "jz-backup-"
const fileSuffix = ".db"

// BackupInfo 备份文件信息。
type BackupInfo struct {
	ID        string `json:"id"`
	Size      int64  `json:"sizeBytes"`
	CreatedAt string `json:"createdAt"`
}

// Repo 管理备份目录与快照文件。
type Repo struct {
	db     *sql.DB
	dbPath string
	dir    string
	keep   int
}

// New 创建 Repo。dbPath 为当前数据库文件路径，dir 为备份目录，keep 为保留份数。
func New(db *sql.DB, dbPath, dir string, keep int) *Repo {
	if keep < 1 {
		keep = 1
	}
	return &Repo{db: db, dbPath: dbPath, dir: dir, keep: keep}
}

// List 列出备份（按时间倒序，最新在前）。
func (r *Repo) List() ([]BackupInfo, error) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupInfo{}, nil
		}
		return nil, fmt.Errorf("读取备份目录失败: %w", err)
	}

	var items []BackupInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), filePrefix) || !strings.HasSuffix(e.Name(), fileSuffix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		ts := strings.TrimSuffix(strings.TrimPrefix(e.Name(), filePrefix), fileSuffix)
		items = append(items, BackupInfo{ID: e.Name(), Size: info.Size(), CreatedAt: ts})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	return items, nil
}

// Create 立即生成一份一致性备份并执行保留策略清理。
func (r *Repo) Create() (*BackupInfo, error) {
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建备份目录失败: %w", err)
	}
	name := filePrefix + time.Now().UTC().Format("20060102-150405") + fileSuffix
	path := filepath.Join(r.dir, name)

	// VACUUM INTO 参数不能走绑定占位符，路径为内部生成、不含单引号，直接拼接安全。
	if _, err := r.db.Exec("VACUUM INTO '" + path + "'"); err != nil {
		return nil, fmt.Errorf("生成备份失败: %w", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("读取备份文件失败: %w", err)
	}

	if err := r.prune(); err != nil {
		return nil, err
	}

	ts := strings.TrimSuffix(strings.TrimPrefix(name, filePrefix), fileSuffix)
	return &BackupInfo{ID: name, Size: info.Size(), CreatedAt: ts}, nil
}

// RestoreFile 用备份文件覆盖目标数据库文件（含清理 -wal/-shm）。
// ⚠️ 调用方必须先关闭正在使用的数据库连接，恢复后重新打开。
func RestoreFile(dbPath, backupPath string) error {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}
	for _, p := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("清理旧数据文件失败: %w", err)
		}
	}

	src, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("打开备份文件失败: %w", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(dbPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("创建数据库文件失败: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("复制备份失败: %w", err)
	}
	return dst.Sync()
}

// prune 删除超出保留份数的旧备份。
func (r *Repo) prune() error {
	items, err := r.List()
	if err != nil {
		return err
	}
	if len(items) <= r.keep {
		return nil
	}
	for _, it := range items[r.keep:] {
		if err := os.Remove(filepath.Join(r.dir, it.ID)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("清理旧备份失败: %w", err)
		}
	}
	return nil
}
