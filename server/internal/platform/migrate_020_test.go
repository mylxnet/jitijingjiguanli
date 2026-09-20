package platform

import (
	"database/sql"
	"io/fs"
	"sort"
	"strings"
	"testing"
)

// applyMigrationsBefore 只应用文件名 < boundary 的迁移，模拟升级前的老库。
func applyMigrationsBefore(t *testing.T, db *sql.DB, boundary string) {
	t.Helper()
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
		if strings.HasSuffix(e.Name(), ".sql") && e.Name() < boundary {
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
}

func mustInt(t *testing.T, db *sql.DB, query string, args ...any) int64 {
	t.Helper()
	var v int64
	if err := db.QueryRow(query, args...).Scan(&v); err != nil {
		t.Fatalf("查询失败(%s): %v", query, err)
	}
	return v
}

// TestMigration020ReinvestIncomeCat 验证迁移 020：
// ①逐组织补建 L1「再投资收益」并把后续容器 sort 后移；
// ②把误记在「再投资」本金科目下的再投资收益现金流水精确挪到新容器下的同名二级，
//
//	使本金科目恢复真实投出（不再有 income 痕迹）；
//
// ③抵销核销（txn_id 指向发放支出流水）一律不动；
// ④重复执行幂等：不重复建容器、不重复后移 sort、不重复挪流水。
func TestMigration020ReinvestIncomeCat(t *testing.T) {
	db := openTempDB(t)
	applyMigrationsBefore(t, db, "020")

	orgID := seedOrg(t, db)
	const stamp = "2026-09-02"

	exec := func(query string, args ...any) int64 {
		t.Helper()
		res, err := db.Exec(query, args...)
		if err != nil {
			t.Fatalf("种子数据失败(%s): %v", query, err)
		}
		id, _ := res.LastInsertId()
		return id
	}
	cat := func(name string, level int, parent any, sortN int) int64 {
		return exec(`INSERT INTO category(org_id, name, level, parent_id, status, kind, preset, sort_order, created_at, updated_at)
			VALUES(?,?,?,?, 'active','equity',1,?,?,?)`, orgID, name, level, parent, sortN, stamp, stamp)
	}

	party := exec(`INSERT INTO party(org_id, name, type, created_at, updated_at)
		VALUES(?, '羊鸡福', 'reinvest', ?, ?)`, orgID, stamp, stamp)

	l1Reinvest := cat("再投资", 1, nil, 3)
	cat("投资收益", 1, nil, 5)
	l1Land := cat("土地流转费收入", 1, nil, 6)
	l1Fee := cat("流转管理费", 1, nil, 7)
	l1Dist := cat("分配与支出", 1, nil, 8)

	// 本金科目：期初 1,683.445（真实投出），收益错记后余额被抬到 1,733.948
	l2Principal := exec(`INSERT INTO category(org_id, name, level, parent_id, status, kind, preset,
			opening_balance_cents, sort_order, created_at, updated_at)
		VALUES(?, '羊鸡福', 2, ?, 'active','equity',0, 1683445, 0, ?, ?)`, orgID, l1Reinvest, stamp, stamp)
	l2Welfare := cat("成员分红", 2, l1Dist, 1)

	// ① 现金核销：再投资收益 50,503 落在本金科目下（迁移要修的就是这个）
	recCash := exec(`INSERT INTO receivable(org_id, party_id, recv_year, recv_kind, title, amount_cents,
			income_category_id, status, note, created_at, updated_at)
		VALUES(?,?,2026,'reinvest_dividend','2026年度再投资收益',50503, NULL,'closed',NULL,?,?)`,
		orgID, party, stamp, stamp)
	txnCash := exec(`INSERT INTO txn(org_id, txn_date, direction, amount_cents, category_id, note, status, created_at, updated_at)
		VALUES(?, '2026-09-20','income',50503,?, '收缴核销（收到投资收益/分红）','normal',?,?)`,
		orgID, l2Principal, stamp, stamp)
	exec(`INSERT INTO receipt(org_id, receivable_id, amount_cents, receipt_date, method, txn_id, note, status, created_at, updated_at)
		VALUES(?,?,?,?, 'cash',?, NULL,'normal',?,?)`, orgID, recCash, 50503, "2026-09-20", txnCash, stamp, stamp)

	// ② 抵销核销：receipt.txn_id 指向发放支出流水（expense），不得被当成收益挪走
	recOffset := exec(`INSERT INTO receivable(org_id, party_id, recv_year, recv_kind, title, amount_cents,
			income_category_id, status, note, created_at, updated_at)
		VALUES(?,?,2026,'reinvest_dividend','2026年度再投资收益（抵销）',30000, NULL,'open',NULL,?,?)`,
		orgID, party, stamp, stamp)
	txnOffset := exec(`INSERT INTO txn(org_id, txn_date, direction, amount_cents, category_id, note, status, created_at, updated_at)
		VALUES(?, '2026-09-20','expense',30000,?, '532-成员分配发放','normal',?,?)`,
		orgID, l2Welfare, stamp, stamp)
	exec(`INSERT INTO receipt(org_id, receivable_id, amount_cents, receipt_date, method, txn_id, note, status, created_at, updated_at)
		VALUES(?,?,?,?, 'offset',?, NULL,'normal',?,?)`, orgID, recOffset, 30000, "2026-09-20", txnOffset, stamp, stamp)

	if err := Migrate(db); err != nil {
		t.Fatalf("升级迁移失败: %v", err)
	}

	// ① 新容器就位 + sort 后移
	newL1 := mustInt(t, db, `SELECT id FROM category WHERE org_id=? AND level=1 AND name='再投资收益'`, orgID)
	if got := mustInt(t, db, `SELECT COUNT(*) FROM category WHERE org_id=? AND level=1 AND name='再投资收益'`, orgID); got != 1 {
		t.Fatalf("「再投资收益」L1 应恰好 1 条，实际 %d", got)
	}
	if got := mustInt(t, db, `SELECT preset FROM category WHERE id=?`, newL1); got != 1 {
		t.Errorf("「再投资收益」应为预置科目 preset=1，实际 %d", got)
	}
	for id, want := range map[int64]int64{newL1: 6, l1Land: 7, l1Fee: 8, l1Dist: 9} {
		if got := mustInt(t, db, `SELECT sort_order FROM category WHERE id=?`, id); got != want {
			t.Errorf("科目 %d sort_order 应为 %d，实际 %d", id, want, got)
		}
	}
	if got := mustInt(t, db, `SELECT COALESCE(parent_id, 0) FROM category WHERE id=?`, newL1); got != 0 {
		t.Errorf("「再投资收益」应为一级（parent_id NULL），实际 parent_id=%d", got)
	}

	// ② 收益流水挪到 再投资收益/羊鸡福，本金科目恢复无收益痕迹
	dst := mustInt(t, db, `SELECT id FROM category WHERE org_id=? AND level=2 AND name='羊鸡福' AND parent_id=?`, orgID, newL1)
	if got := mustInt(t, db, `SELECT category_id FROM txn WHERE id=?`, txnCash); got != dst {
		t.Errorf("收益流水 %d 应挪到 再投资收益/羊鸡福(%d)，实际 category_id=%d", txnCash, dst, got)
	}
	if got := mustInt(t, db, `SELECT COUNT(*) FROM txn WHERE category_id=? AND direction='income'`, l2Principal); got != 0 {
		t.Errorf("本金科目下不应再有收益流水，实际 %d 条", got)
	}
	if got := mustInt(t, db, `SELECT opening_balance_cents FROM category WHERE id=?`, l2Principal); got != 1683445 {
		t.Errorf("本金科目期初不应被迁移改动，期望 1683445，实际 %d", got)
	}
	if got := mustInt(t, db, `SELECT opening_balance_cents FROM category WHERE id=?`, dst); got != 0 {
		t.Errorf("新建收益二级期初应为 0（不虚增本金），实际 %d", got)
	}

	// ③ 抵销流水不动
	if got := mustInt(t, db, `SELECT category_id FROM txn WHERE id=?`, txnOffset); got != l2Welfare {
		t.Errorf("抵销关联的支出流水科目不应变动，期望 %d，实际 %d", l2Welfare, got)
	}

	// ④ 幂等：重复执行 020 脚本本体
	content, err := migrationsFS.ReadFile("migrations/020_reinvest_income_cat.sql")
	if err != nil {
		t.Fatalf("读取迁移 020 失败: %v", err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("开启事务失败: %v", err)
	}
	if _, err := tx.Exec(string(content)); err != nil {
		tx.Rollback()
		t.Fatalf("重复执行迁移 020 失败: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	if got := mustInt(t, db, `SELECT COUNT(*) FROM category WHERE org_id=? AND level=1 AND name='再投资收益'`, orgID); got != 1 {
		t.Errorf("重复执行后「再投资收益」L1 仍应 1 条，实际 %d", got)
	}
	if got := mustInt(t, db, `SELECT sort_order FROM category WHERE id=?`, l1Dist); got != 9 {
		t.Errorf("重复执行不应再次后移 sort，分配与支出 期望 9，实际 %d", got)
	}
	if got := mustInt(t, db, `SELECT COUNT(*) FROM category WHERE org_id=? AND level=2 AND parent_id=?`, orgID, newL1); got != 1 {
		t.Errorf("重复执行不应重复建收益二级，期望 1，实际 %d", got)
	}
	if got := mustInt(t, db, `SELECT category_id FROM txn WHERE id=?`, txnCash); got != dst {
		t.Errorf("重复执行后收益流水仍应指向 %d，实际 %d", dst, got)
	}
	if err := Migrate(db); err != nil {
		t.Errorf("再次 Migrate 应跳过已应用版本且不报错: %v", err)
	}
}
