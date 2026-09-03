package platform

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// TestMigrate 验证迁移机制：空库应用全部迁移 → 全量表就位 → 重复执行幂等。
func TestMigrate(t *testing.T) {
	db := openTempDB(t)

	if err := Migrate(db); err != nil {
		t.Fatalf("首次迁移失败: %v", err)
	}

	for _, table := range []string{
		"org", "user", "session", "category", "txn", "app_setting", "transfer",
		"transfer_leg", "change_log", "fund_move", "party", "receivable", "receipt",
		"schema_migrations",
	} {
		if !tableExists(t, db, table) {
			t.Errorf("表 %s 不存在", table)
		}
	}

	// 幂等：再次迁移应无副作用、不报错
	if err := Migrate(db); err != nil {
		t.Fatalf("重复迁移失败（应幂等）: %v", err)
	}

	var applied int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatalf("查询迁移记录失败: %v", err)
	}
	if applied != 4 {
		t.Errorf("schema_migrations 应有 4 条记录（001-004），实际 %d", applied)
	}
}

// TestCategoryCheck 验证 D6 强制校验落库：花费型不得参与勾稽；资产型禁止勾稽/花费型。
func TestCategoryCheck(t *testing.T) {
	db := openTempDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	orgID := seedOrg(t, db)

	cases := []struct {
		name string
		sql  string
	}{
		{"花费型+勾稽", `INSERT INTO category(org_id, name, level, status, balance_type, kind,
			opening_balance_cents, include_in_reconciliation, preset, sort_order, created_at, updated_at)
			VALUES(?, '办公费', 2, 'active', 'spending', 'normal', 0, 1, 0, 0, '2026-09-02', '2026-09-02')`},
		{"资产型+勾稽", `INSERT INTO category(org_id, name, level, status, balance_type, kind,
			opening_balance_cents, include_in_reconciliation, preset, sort_order, created_at, updated_at)
			VALUES(?, '投资X', 2, 'active', 'residual', 'asset', 0, 1, 0, 0, '2026-09-02', '2026-09-02')`},
		{"资产型+花费", `INSERT INTO category(org_id, name, level, status, balance_type, kind,
			opening_balance_cents, include_in_reconciliation, preset, sort_order, created_at, updated_at)
			VALUES(?, '投资Y', 2, 'active', 'spending', 'asset', 0, 0, 0, 0, '2026-09-02', '2026-09-02')`},
	}
	for _, tc := range cases {
		if _, err := db.Exec(tc.sql, orgID); err == nil {
			t.Errorf("%s 应被 CHECK 拒绝，实际插入成功", tc.name)
		}
	}
}

// seedOrg 插入一个测试组织。
func seedOrg(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	res, err := db.Exec(
		`INSERT INTO org(name, created_at, updated_at) VALUES('测试组织', '2026-09-02', '2026-09-02')`)
	if err != nil {
		t.Fatalf("插入测试组织失败: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// openTempDB 在系统临时目录建库，避免污染项目目录。
func openTempDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("打开临时库失败: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var n int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name,
	).Scan(&n); err != nil {
		t.Fatalf("查询表 %s 失败: %v", name, err)
	}
	return n > 0
}
