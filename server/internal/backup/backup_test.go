package backup

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"jititaizhang/server/internal/platform"
)

func openDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := platform.Open(path)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	if err := platform.Migrate(db); err != nil {
		db.Close()
		t.Fatalf("迁移失败: %v", err)
	}
	if _, err := db.Exec(
		`INSERT OR IGNORE INTO org(id, name, created_at, updated_at) VALUES(1, '测试组织', '2026-09-02', '2026-09-02')`); err != nil {
		db.Close()
		t.Fatalf("插入测试组织失败: %v", err)
	}
	return db
}

func setVal(t *testing.T, db *sql.DB, v string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO app_setting(org_id, key, value) VALUES(1,'probe',?)
		 ON CONFLICT(org_id, key) DO UPDATE SET value = excluded.value`, v); err != nil {
		t.Fatalf("写入探针值失败: %v", err)
	}
}

func getVal(t *testing.T, db *sql.DB) string {
	t.Helper()
	var v string
	if err := db.QueryRow(`SELECT value FROM app_setting WHERE org_id=1 AND key='probe'`).Scan(&v); err != nil {
		t.Fatalf("读取探针值失败: %v", err)
	}
	return v
}

// TestCreateListRestore 备份 → 改数据 → 恢复 → 数据回退（F7 验收核心）。
func TestCreateListRestore(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "jititaizhang.db")
	backDir := filepath.Join(dir, "backups")

	db := openDB(t, dbPath)
	defer db.Close()
	setVal(t, db, "before")

	repo := New(db, dbPath, backDir, 10)
	b1, err := repo.Create()
	if err != nil {
		t.Fatalf("备份失败: %v", err)
	}
	if b1.ID == "" || b1.Size == 0 {
		t.Errorf("备份信息不完整: %+v", b1)
	}

	// 修改数据
	setVal(t, db, "after")

	// 关闭连接 → 覆盖恢复 → 重开验证
	if err := db.Close(); err != nil {
		t.Fatalf("关闭数据库失败: %v", err)
	}
	if err := RestoreFile(dbPath, filepath.Join(backDir, b1.ID)); err != nil {
		t.Fatalf("恢复失败: %v", err)
	}
	db2 := openDB(t, dbPath)
	defer db2.Close()
	if got := getVal(t, db2); got != "before" {
		t.Errorf("恢复后探针值应回到 before，实际 %s", got)
	}

	// 列表可见 1 份
	items, err := New(db2, dbPath, backDir, 10).List()
	if err != nil {
		t.Fatalf("列出备份失败: %v", err)
	}
	if len(items) != 1 || items[0].ID != b1.ID {
		t.Errorf("列表应恰含本次备份，实际 %+v", items)
	}
}

// TestPruneKeep 保留策略：超出 keep 的最旧备份被清理。
func TestPruneKeep(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "jititaizhang.db")
	backDir := filepath.Join(dir, "backups")

	db := openDB(t, dbPath)
	defer db.Close()

	repo := New(db, dbPath, backDir, 2)
	// 生成 3 份（时间戳按秒，必要时错开）
	for i := 0; i < 3; i++ {
		if _, err := repo.Create(); err != nil {
			t.Fatalf("第 %d 次备份失败: %v", i+1, err)
		}
		time.Sleep(1100 * time.Millisecond)
	}
	items, err := repo.List()
	if err != nil {
		t.Fatalf("列出备份失败: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("保留 2 份应只剩 2 个备份，实际 %d", len(items))
	}
}
