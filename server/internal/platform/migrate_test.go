package platform

import (
	"database/sql"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
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
	if applied != 20 {
		t.Errorf("schema_migrations 应有 20 条记录（001-020），实际 %d", applied)
	}
}

// TestMigration017Backfill 模拟升级：017 之前的老库中，有业务数据的组织自动回填为已建账，
// 空组织保持未建账（确保老组织不被拉进引导页，新组织仍进引导页）。
func TestMigration017Backfill(t *testing.T) {
	db := openTempDB(t)

	// 1) 只应用 001-016，模拟升级前的库（每个迁移单独事务，与 Migrate 行为一致）
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY, applied_at DATETIME NOT NULL)`); err != nil {
		t.Fatalf("创建迁移记录表失败: %v", err)
	}
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		t.Fatalf("读取迁移目录失败: %v", err)
	}
	var files []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") && e.Name() < "017" {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	for _, f := range files {
		content, err := migrationsFS.ReadFile("migrations/" + f)
		if err != nil {
			t.Fatalf("读取迁移 %s 失败: %v", f, err)
		}
		tx, err := db.Begin()
		if err != nil {
			t.Fatalf("开启事务失败: %v", err)
		}
		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			t.Fatalf("应用迁移 %s 失败: %v", f, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(?, '2026-09-02')`, f); err != nil {
			tx.Rollback()
			t.Fatalf("记录迁移 %s 失败: %v", f, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("提交迁移 %s 失败: %v", f, err)
		}
	}

	// 2) 造数据：org1 已有往来单位（老组织），org2 为空（新组织）
	if _, err := db.Exec(`INSERT INTO org(id, name, created_at, updated_at) VALUES
		(1, '老组织', '2026-09-02', '2026-09-02'), (2, '新组织', '2026-09-02', '2026-09-02')`); err != nil {
		t.Fatalf("插入组织失败: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO party(org_id, name, type, created_at, updated_at)
		VALUES(1, 'A单位', 'flow', '2026-09-02', '2026-09-02')`); err != nil {
		t.Fatalf("插入往来单位失败: %v", err)
	}

	// 3) 应用 017 → org1 回填为已建账，org2 保持未建账
	if err := Migrate(db); err != nil {
		t.Fatalf("升级迁移失败: %v", err)
	}
	for _, tc := range []struct {
		org  int64
		want int
		desc string
	}{
		{1, 1, "有业务数据的老组织"},
		{2, 0, "空的新组织"},
	} {
		var got int
		if err := db.QueryRow(`SELECT onboarded FROM org WHERE id=?`, tc.org).Scan(&got); err != nil {
			t.Fatalf("读取 org%d onboarded 失败: %v", tc.org, err)
		}
		if got != tc.want {
			t.Errorf("%s：期望 onboarded=%d，实际 %d", tc.desc, tc.want, got)
		}
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
