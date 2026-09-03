package summary

import (
	"database/sql"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"jititaizhang/server/internal/platform"
)

func openDB(t *testing.T) (*sql.DB, int64) {
	t.Helper()
	db, err := platform.Open(filepath.Join(t.TempDir(), "summary.db"))
	if err != nil {
		t.Fatalf("打开临时库失败: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := platform.Migrate(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	now := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO org(id, name, created_at, updated_at) VALUES(1,'测试组织',?,?)`, now, now); err != nil {
		t.Fatalf("插组织失败: %v", err)
	}
	return db, 1
}

func setBank(t *testing.T, db *sql.DB, org int64, opening int64) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO app_setting(org_id, key, value) VALUES(?,'bank_opening_balance_cents',?)
		 ON CONFLICT(org_id, key) DO UPDATE SET value = excluded.value`, org, strconv.FormatInt(opening, 10)); err != nil {
		t.Fatalf("设置银行期初失败: %v", err)
	}
}

func cat(t *testing.T, db *sql.DB, org int64, name string, level int, parent any, kind string) int64 {
	t.Helper()
	now := time.Now().UTC()
	res, err := db.Exec(`INSERT INTO category(org_id, name, level, parent_id, status, kind,
		sort_order, created_at, updated_at) VALUES(?,?,?,?, 'active', ?,0,?,?)`,
		org, name, level, parent, kind, now, now)
	if err != nil {
		t.Fatalf("插科目 %s 失败: %v", name, err)
	}
	id, _ := res.LastInsertId()
	return id
}

func txn(t *testing.T, db *sql.DB, org int64, date, dir string, amt, cid int64) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO txn(org_id, txn_date, direction, amount_cents, category_id, note, status, created_at, updated_at)
		VALUES(?,?,?,?,?,NULL,'normal',?,?)`, org, date, dir, amt, cid, now, now); err != nil {
		t.Fatalf("插流水失败: %v", err)
	}
}

func fundMove(t *testing.T, db *sql.DB, org int64, date, kind string, assetID, amt int64) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO fund_move(org_id, move_date, kind, asset_category_id, amount_cents, note, status, created_at, updated_at)
		VALUES(?,?,?,?,?,NULL,'normal',?,?)`, org, date, kind, assetID, amt, now, now); err != nil {
		t.Fatalf("插资金划转失败: %v", err)
	}
}

// TestCapitalEquity v0.4 资金构成：bank/asset/equity。
func TestCapitalEquity(t *testing.T) {
	db, org := openDB(t)
	setBank(t, db, org, 200000)

	l1Fund := cat(t, db, org, "本金", 1, nil, "equity")
	principal := cat(t, db, org, "上级补助", 2, l1Fund, "equity")
	l1Dist := cat(t, db, org, "分配与支出", 1, nil, "equity")
	welfare := cat(t, db, org, "福利发放", 2, l1Dist, "equity")
	l1Inv := cat(t, db, org, "对外投资", 1, nil, "equity")
	invA := cat(t, db, org, "项目A", 2, l1Inv, "asset")

	txn(t, db, org, "2026-09-01", "income", 100000, principal)
	txn(t, db, org, "2026-09-02", "expense", 30000, welfare)
	fundMove(t, db, org, "2026-09-05", "invest", invA, 60000)

	s := NewRepo(db)
	resp, err := s.GetSummary(org, "2026-09-01", "2026-09-30")
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	c := resp.Capital
	if c.BankBalanceCents != 210000 {
		t.Errorf("银行应 200000+100000-30000-60000=210000，实际 %d", c.BankBalanceCents)
	}
	if c.AssetTotalCents != 60000 {
		t.Errorf("资产合计应 60000，实际 %d", c.AssetTotalCents)
	}
	if c.EquityTotalCents != 70000 {
		t.Errorf("权益合计应 100000-30000=70000，实际 %d", c.EquityTotalCents)
	}

	// 区间收支：incomeTotal 100000、expenseTotal 30000
	if resp.IncomeTotal != 100000 || resp.ExpenseTotal != 30000 || resp.Balance != 70000 {
		t.Errorf("区间收支不对: in=%d out=%d bal=%d", resp.IncomeTotal, resp.ExpenseTotal, resp.Balance)
	}
}

// TestCategoryTreeAndPeriod 科目树：一级=子项和；资产行按划转余额；发生额为区间内。
func TestCategoryTreeAndPeriod(t *testing.T) {
	db, org := openDB(t)
	l1Fund := cat(t, db, org, "本金", 1, nil, "equity")
	principal := cat(t, db, org, "上级补助", 2, l1Fund, "equity")
	welfare := cat(t, db, org, "福利发放", 2, l1Fund, "equity")

	txn(t, db, org, "2026-08-01", "income", 10000, principal)
	txn(t, db, org, "2026-09-01", "income", 20000, principal)
	txn(t, db, org, "2026-09-02", "expense", 5000, welfare)

	s := NewRepo(db)
	resp, err := s.GetSummary(org, "2026-09-01", "2026-09-30")
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	if len(resp.Categories) != 1 {
		t.Fatalf("应有 1 个一级，实际 %d", len(resp.Categories))
	}
	root := resp.Categories[0]
	// 一级余额 = 20000 - 5000 + (8月收入10000 不参与当前余额？余额全年累计)
	if root.CurrentBalanceCents != 25000 {
		t.Errorf("一级余额应 10000+20000-5000=25000，实际 %d", root.CurrentBalanceCents)
	}
	// 区间发生额只计 9 月：principal income 20000；welfare expense 5000
	for _, child := range root.Children {
		if child.Name == "上级补助" {
			if child.IncomeCents != 20000 {
				t.Errorf("上级补助区间收入应 20000，实际 %d", child.IncomeCents)
			}
		}
		if child.Name == "福利发放" && child.ExpenseCents != 5000 {
			t.Errorf("福利发放区间支出应 5000，实际 %d", child.ExpenseCents)
		}
	}
}
