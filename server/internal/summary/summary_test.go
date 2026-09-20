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

// party 插入往来单位并返回 id。
func partyRec(t *testing.T, db *sql.DB, org int64) int64 {
	t.Helper()
	now := time.Now().UTC()
	res, err := db.Exec(`INSERT INTO party(org_id, name, created_at, updated_at) VALUES(?, '测试单位', ?, ?)`,
		org, now, now)
	if err != nil {
		t.Fatalf("插单位失败: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// receivable 插入应收单（返回 id）。
func receivable(t *testing.T, db *sql.DB, org int64, kind string, amt int64) int64 {
	t.Helper()
	now := time.Now().UTC()
	pid := partyRec(t, db, org)
	res, err := db.Exec(`INSERT INTO receivable(org_id, party_id, recv_year, recv_kind, title,
		amount_cents, income_category_id, status, note, created_at, updated_at)
		VALUES(?, ?, 2026, ?, '测试', ?, NULL, 'open', NULL, ?, ?)`, org, pid, kind, amt, now, now)
	if err != nil {
		t.Fatalf("插应收单失败(%s): %v", kind, err)
	}
	id, _ := res.LastInsertId()
	return id
}

// receiptInsert 插入核销记录（部分收/全额收）。
func receiptInsert(t *testing.T, db *sql.DB, org, recvID, amt int64) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO receipt(org_id, receivable_id, amount_cents, receipt_date, method,
		txn_id, note, status, created_at, updated_at)
		VALUES(?, ?, ?, '2026-09-10', 'cash', NULL, NULL, 'normal', ?, ?)`, org, recvID, amt, now, now); err != nil {
		t.Fatalf("插核销失败: %v", err)
	}
}

// TestComposition v0.21 环形构成：资金/在外投资/欠款/可支出四块。
// 验证资金构成排除流转费/管理费欠款，欠款按 recv_kind 分组。
func TestComposition(t *testing.T) {
	db, org := openDB(t)
	setBank(t, db, org, 100000)

	// 资产类：长期投资 L1 + asset 二级（fund_move invest 产生余额）；
	// 再投资 L1 + asset 二级；
	// 其它资产挂「对外投资」L1 → 归入「其他」。
	l1Inv := cat(t, db, org, "长期投资", 1, nil, "equity")
	l1Re := cat(t, db, org, "再投资", 1, nil, "equity")
	l1Other := cat(t, db, org, "对外投资", 1, nil, "equity")
	invA := cat(t, db, org, "项目A", 2, l1Inv, "asset")
	reA := cat(t, db, org, "再投资A", 2, l1Re, "asset")
	otherA := cat(t, db, org, "股权B", 2, l1Other, "asset")
	// welfare 公益支出 equity L2（名即「公益支出」，与 532 模板按名称解析一致），可支出
	l1Welf := cat(t, db, org, "公益支出", 1, nil, "equity")
	welfCat := cat(t, db, org, "公益支出", 2, l1Welf, "equity")

	fundMove(t, db, org, "2026-08-01", "invest", invA, 50000)
	fundMove(t, db, org, "2026-08-02", "invest", reA, 30000)
	fundMove(t, db, org, "2026-08-03", "invest", otherA, 20000)
	// 公益支出科目期初一次性收入，用于余额 = 可支出
	txn(t, db, org, "2026-08-01", "income", 40000, welfCat)

	// 应收欠款：
	// 土地流转费 rent 10000，已收 4000 → 未收 6000
	// 投资收益 dividend 8000，已收 3000 → 未收 5000
	// 其他 other 3000，未收 → 3000（应计入欠款构成「其他」分项，与单位管理页口径对齐）
	rRent := receivable(t, db, org, "rent", 10000)
	rDiv := receivable(t, db, org, "dividend", 8000)
	receivable(t, db, org, "other", 3000)
	receiptInsert(t, db, org, rRent, 4000)
	receiptInsert(t, db, org, rDiv, 3000)
	// 532 分配方案：公益 40000（保证可支出-公益支出 = 40000 − 已出0）
	insert532(t, db, org, 2026, 0, 0, 0, 40000)

	s := NewRepo(db)
	resp, err := s.GetSummary(org, "2026-08-01", "2026-09-30")
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	comp := resp.Composition
	if comp == nil {
		t.Fatal("composition 不应为空")
	}

	// 资金构成：银行存款 + 长期投资 + 应收收益（应收收益=dividend 未收 5000；流转费不计入）
	// 银行存款 = 期初100000 + 收入40000 − 投资划转100000 = 40000
	fundBank := sliceVal(comp.Fund, "银行存款")
	if fundBank != 40000 {
		t.Errorf("资金-银行存款应 %d，实际 %d", 40000, fundBank)
	}
	fundAsset := sliceVal(comp.Fund, "长期投资")
	if fundAsset != 100000 {
		t.Errorf("资金-长期投资应 50000+30000+20000=100000，实际 %d", fundAsset)
	}
	fundInc := sliceVal(comp.Fund, "应收收益")
	if fundInc != 5000 {
		t.Errorf("资金-应收收益应 5000（不含流转费），实际 %d", fundInc)
	}
	// 土地流转费欠款不计入资金构成
	if hasSlice(comp.Fund, "土地流转费") {
		t.Error("资金构成不应含土地流转费欠款")
	}

	// 在外投资构成：长期投资 50000 / 再投资 30000 / 其他 20000
	if sliceVal(comp.Invest, "长期投资") != 50000 {
		t.Errorf("外投-长期投资应 50000，实际 %d", sliceVal(comp.Invest, "长期投资"))
	}
	if sliceVal(comp.Invest, "再投资") != 30000 {
		t.Errorf("外投-再投资应 30000，实际 %d", sliceVal(comp.Invest, "再投资"))
	}
	if sliceVal(comp.Invest, "其他") != 20000 {
		t.Errorf("外投-其他应 20000，实际 %d", sliceVal(comp.Invest, "其他"))
	}

	// 欠款构成：土地流转费 6000 / 流转管理费 0 / 应收收益 5000 / 其他 3000
	if sliceVal(comp.Owe, "土地流转费") != 6000 {
		t.Errorf("欠款-土地流转费应 6000，实际 %d", sliceVal(comp.Owe, "土地流转费"))
	}
	if sliceVal(comp.Owe, "应收收益") != 5000 {
		t.Errorf("欠款-应收收益应 5000，实际 %d", sliceVal(comp.Owe, "应收收益"))
	}
	if sliceVal(comp.Owe, "其他") != 3000 {
		t.Errorf("欠款-其他应 3000，实际 %d", sliceVal(comp.Owe, "其他"))
	}

	// 可支出构成：公益支出科目余额 40000
	if sliceVal(comp.Expense, "公益支出") != 40000 {
		t.Errorf("可支出-公益支出应 40000，实际 %d", sliceVal(comp.Expense, "公益支出"))
	}
}

func sliceVal(slices []Slice, name string) int64 {
	for _, s := range slices {
		if s.Name == name {
			return s.Value
		}
	}
	return 0
}

func hasSlice(slices []Slice, name string) bool {
	for _, s := range slices {
		if s.Name == name {
			return true
		}
	}
	return false
}

// setOpen 直接设定科目期初本金（模拟引导页导入时投资单位 equity 科目带期初本金）。
func setOpen(t *testing.T, db *sql.DB, id, cents int64) {
	t.Helper()
	if _, err := db.Exec(`UPDATE category SET opening_balance_cents = ? WHERE id = ?`, cents, id); err != nil {
		t.Fatalf("设期初本金失败: %v", err)
	}
}

// insert532 插入某年 532 分配方案。
func insert532(t *testing.T, db *sql.DB, org, year, total, re, div, welf int64) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO distribution_532(org_id, year, total_income_cents, reinvest_cents, dividend_cents, welfare_cents, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?)`, org, year, total, re, div, welf, now, now); err != nil {
		t.Fatalf("插532分配失败: %v", err)
	}
}

// TestCompositionInvestmentEquity v0.21 在外投资：投资容器一级（长期投资/再投资/对外投资）下
// 无论 equity 还是 asset 二级都计入（含期初本金），资金构成的「长期投资」取全部在外投资合计。
func TestCompositionInvestmentEquity(t *testing.T) {
	db, org := openDB(t)
	setBank(t, db, org, 50000)

	l1Inv := cat(t, db, org, "长期投资", 1, nil, "equity")
	l1Re := cat(t, db, org, "再投资", 1, nil, "equity")
	l1Other := cat(t, db, org, "对外投资", 1, nil, "equity")
	invA := cat(t, db, org, "临夏投资A", 2, l1Inv, "equity")
	invB := cat(t, db, org, "临夏投资B", 2, l1Re, "equity")
	invO := cat(t, db, org, "股权外投", 2, l1Other, "equity")
	setOpen(t, db, invA, 1500000)
	setOpen(t, db, invB, 800000)
	setOpen(t, db, invO, 200000)
	// equity 投资科目同样用收支记账（expense 抵减本金 = 部分收回）
	txn(t, db, org, "2026-09-01", "expense", 100000, invA) // invA 余额 = 1400000

	// 同组织的普通收入类科目不影响投资聚合，但仍应存在
	l1Inc := cat(t, db, org, "经营收入", 1, nil, "equity")
	incCat := cat(t, db, org, "出租收入", 2, l1Inc, "equity")
	txn(t, db, org, "2026-09-01", "income", 30000, incCat)

	s := NewRepo(db)
	resp, err := s.GetSummary(org, "2026-09-01", "2026-09-30")
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	comp := resp.Composition
	if comp == nil {
		t.Fatal("composition 不应为空")
	}

	// 在外投资：长期投资 1400000 / 再投资 800000 / 其他 200000
	if sliceVal(comp.Invest, "长期投资") != 1400000 {
		t.Errorf("外投-长期投资应 1500000-100000=1400000，实际 %d", sliceVal(comp.Invest, "长期投资"))
	}
	if sliceVal(comp.Invest, "再投资") != 800000 {
		t.Errorf("外投-再投资应 800000，实际 %d", sliceVal(comp.Invest, "再投资"))
	}
	if sliceVal(comp.Invest, "其他") != 200000 {
		t.Errorf("外投-其他应 200000，实际 %d", sliceVal(comp.Invest, "其他"))
	}

	// 资金构成-长期投资 = 全部在外投资合计 = 1400000+800000+200000=2400000
	if sliceVal(comp.Fund, "长期投资") != 2400000 {
		t.Errorf("资金-长期投资应 2400000，实际 %d", sliceVal(comp.Fund, "长期投资"))
	}
}

// TestCompositionExpense v0.21 可支出构成 = 流转管理费科目余额 + 公益支出科目余额。
func TestCompositionExpense(t *testing.T) {
	db, org := openDB(t)

	// 流转管理费一级下收入科目：收到管理费 852000
	l1Flow := cat(t, db, org, "流转管理费", 1, nil, "equity")
	flowCat := cat(t, db, org, "提取管理费", 2, l1Flow, "equity")
	txn(t, db, org, "2026-08-01", "income", 852000, flowCat)

	// 公益支出科目：已支出 52000
	l1W := cat(t, db, org, "分配与支出", 1, nil, "equity")
	wCat := cat(t, db, org, "公益支出", 2, l1W, "equity")
	txn(t, db, org, "2026-08-02", "expense", 52000, wCat)
	// 532分配方案：公益 220000
	insert532(t, db, org, 2026, 1100000, 550000, 330000, 220000)

	s := NewRepo(db)
	resp, err := s.GetSummary(org, "2026-08-01", "2026-09-30")
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	comp := resp.Composition
	if comp == nil {
		t.Fatal("composition 不应为空")
	}

	// 可支出构成：
	// 流转管理费 = 科目余额 852000
	// 公益支出   = 532分配 welfare 220000 − 已出 52000 = 168000
	// 合计 = 852000 + 168000 = 1020000
	if sliceVal(comp.Expense, "流转管理费") != 852000 {
		t.Errorf("可支出-流转管理费应 852000，实际 %d", sliceVal(comp.Expense, "流转管理费"))
	}
	if sliceVal(comp.Expense, "公益支出") != 168000 {
		t.Errorf("可支出-公益支出应 220000-52000=168000，实际 %d", sliceVal(comp.Expense, "公益支出"))
	}
	var sum int64
	for _, e := range comp.Expense {
		sum += e.Value
	}
	if sum != 1020000 {
		t.Errorf("可支出合计应 1020000，实际 %d", sum)
	}
}

// TestCompositionWelfareCrossYear 可支出-公益支出：分配额与已出必须同为累计口径。
// 旧实现分配额只取最近年度（ORDER BY year DESC LIMIT 1），跨年发放会把该项算成负数。
func TestCompositionWelfareCrossYear(t *testing.T) {
	db, org := openDB(t)

	l1W := cat(t, db, org, "分配与支出", 1, nil, "equity")
	wCat := cat(t, db, org, "公益支出", 2, l1W, "equity")

	// 2025 年度拨 100000、当年花掉 30000；2026 年度拨 50000、当年花掉 40000
	insert532(t, db, org, 2025, 200000, 100000, 50000, 100000)
	txn(t, db, org, "2025-12-10", "expense", 30000, wCat)
	insert532(t, db, org, 2026, 100000, 50000, 30000, 50000)
	txn(t, db, org, "2026-08-02", "expense", 40000, wCat)

	s := NewRepo(db)
	resp, err := s.GetSummary(org, "2026-08-01", "2026-09-30")
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	// 累计分配 150000 − 累计已出 70000 = 80000（旧口径会得 50000-70000 = -20000）
	if got := sliceVal(resp.Composition.Expense, "公益支出"); got != 80000 {
		t.Errorf("可支出-公益支出应 150000-70000=80000，实际 %d", got)
	}
}

// TestCompositionInvestAbs v0.21 投资构成取余额绝对值。
func TestCompositionInvestAbs(t *testing.T) {
	db, org := openDB(t)

	l1Inv := cat(t, db, org, "长期投资", 1, nil, "equity")
	invA := cat(t, db, org, "负余额公司", 2, l1Inv, "equity")
	txn(t, db, org, "2026-08-01", "expense", 200000, invA) // bal = -200000
	invB := cat(t, db, org, "正常公司", 2, l1Inv, "equity")
	txn(t, db, org, "2026-08-01", "income", 100000, invB) // bal = +100000

	s := NewRepo(db)
	resp, err := s.GetSummary(org, "2026-08-01", "2026-09-30")
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	comp := resp.Composition
	if comp == nil {
		t.Fatal("composition 不应为空")
	}
	// 长期投资 = |−200000| + 100000 = 300000
	if sliceVal(comp.Invest, "长期投资") != 300000 {
		t.Errorf("外投-长期投资应 300000，实际 %d", sliceVal(comp.Invest, "长期投资"))
	}
	if sliceVal(comp.Fund, "长期投资") != 300000 {
		t.Errorf("资金-长期投资应 300000，实际 %d", sliceVal(comp.Fund, "长期投资"))
	}
}
