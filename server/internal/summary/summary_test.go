package summary

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/platform"
	"jititaizhang/server/internal/settings"
)

func newEnv(t *testing.T) (*sql.DB, *gin.Engine) {
	t.Helper()
	db, err := platform.Open(filepath.Join(t.TempDir(), "summary.db"))
	if err != nil {
		t.Fatalf("打开临时库失败: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := platform.Migrate(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewHandler(db).Register(r)
	return db, r
}

func seedCat(t *testing.T, db *sql.DB, name string, level int, parent any, status, bt string, opening int64, reconcile bool) int64 {
	t.Helper()
	now := time.Now().UTC()
	inc := 0
	if reconcile {
		inc = 1
	}
	res, err := db.Exec(`INSERT INTO category(name, level, parent_id, status, balance_type,
		opening_balance_cents, include_in_reconciliation, sort_order, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,0,?,?)`, name, level, parent, status, bt, opening, inc, now, now)
	if err != nil {
		t.Fatalf("插入科目失败: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func seedTxn(t *testing.T, db *sql.DB, date, dir string, amount, catID int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO txn(txn_date,direction,amount_cents,category_id,note,status,created_at,updated_at)
		VALUES(?,?,?,?,NULL,'normal','2026-09-01','2026-09-01')`, date, dir, amount, catID); err != nil {
		t.Fatalf("插入流水失败: %v", err)
	}
}

func seedVoidTxn(t *testing.T, db *sql.DB, dir string, amount, catID int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO txn(txn_date,direction,amount_cents,category_id,note,status,created_at,updated_at)
		VALUES('2026-09-15',?,?,?,NULL,'voided','2026-09-01','2026-09-01')`, dir, amount, catID); err != nil {
		t.Fatalf("插入作废流水失败: %v", err)
	}
}

func seedTransfer(t *testing.T, db *sql.DB, sourceID int64, targetID, amount int64) {
	t.Helper()
	res, err := db.Exec(`INSERT INTO transfer(txn_date, source_category_id, source_amount_cents, note, status, created_at, updated_at)
		VALUES('2026-09-10',?,?,NULL,'normal','2026-09-01','2026-09-01')`, sourceID, amount)
	if err != nil {
		t.Fatalf("插入转账失败: %v", err)
	}
	tid, _ := res.LastInsertId()
	if _, err := db.Exec(`INSERT INTO transfer_leg(transfer_id, category_id, amount_cents) VALUES(?,?,?)`, tid, targetID, amount); err != nil {
		t.Fatalf("插入转账明细失败: %v", err)
	}
}

func getSummary(t *testing.T, r *gin.Engine, query string) SummaryResponse {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/summary"+query, nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("汇总应 200，实际 %d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Data SummaryResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析失败: %v body=%s", err, w.Body.String())
	}
	return out.Data
}

// TestCapitalD6 复现 D6 恒等式 + 停用勾稽科目仍计专项资金。
func TestCapitalD6(t *testing.T) {
	db, r := newEnv(t)
	// 期初银行存款 20 万
	if err := settings.NewRepo(db).Upsert("bank_opening_balance_cents", "200000"); err != nil {
		t.Fatalf("写入银行期初失败: %v", err)
	}

	l1 := seedCat(t, db, "专项应付款", 1, nil, "active", "residual", 0, false)
	road := seedCat(t, db, "修路款", 2, l1, "active", "residual", 30000, true)
	water := seedCat(t, db, "水利款", 2, l1, "active", "residual", 0, true)
	_ = seedCat(t, db, "旧专款(停用)", 2, l1, "inactive", "residual", 10000, true) // 停用仍计勾稽

	spL1 := seedCat(t, db, "经营支出", 1, nil, "active", "residual", 0, false)
	elec := seedCat(t, db, "水电费", 2, spL1, "active", "spending", 0, false)

	// D7 场景：收拨款 5 万进修路款，转账 2 万到水利款；水电费支出 4 万
	seedTxn(t, db, "2026-09-01", "income", 50000, road)
	seedVoidTxn(t, db, "expense", 999999, elec) // 作废：不得影响任何口径
	seedTransfer(t, db, road, water, 20000)
	seedTxn(t, db, "2026-09-02", "expense", 40000, elec)

	s := getSummary(t, r, "")

	// 区间收支
	if s.IncomeTotal != 50000 || s.ExpenseTotal != 40000 || s.Balance != 10000 {
		t.Errorf("收支应 50000/40000/10000，实际 %d/%d/%d", s.IncomeTotal, s.ExpenseTotal, s.Balance)
	}
	// 资金构成：bank=200000+50000-40000=210000；earmark=road60000+water20000+old10000=90000
	if s.Capital == nil {
		t.Fatal("capital 为空")
	}
	if s.Capital.BankBalanceCents != 210000 {
		t.Errorf("bank 应 210000，实际 %d", s.Capital.BankBalanceCents)
	}
	if s.Capital.EarmarkedCents != 90000 {
		t.Errorf("earmark 应 90000（含停用 10000），实际 %d", s.Capital.EarmarkedCents)
	}
	if s.Capital.UnallocatedCents != 120000 {
		t.Errorf("unallocated 应 120000，实际 %d", s.Capital.UnallocatedCents)
	}
	if s.Capital.Warning != "" {
		t.Errorf("不应有警告，实际 %s", s.Capital.Warning)
	}

	// 科目树：一级余额=子科目之和；二级本期发生正确
	if len(s.Categories) != 2 {
		t.Fatalf("一级科目应 2 个，实际 %d", len(s.Categories))
	}
	var pz *CategorySummary
	for _, c := range s.Categories {
		if c.Name == "专项应付款" {
			pz = c
		}
	}
	if pz == nil || len(pz.Children) != 3 {
		t.Fatalf("专项应付款应有 3 个子科目")
	}
	if pz.CurrentBalanceCents != 90000 {
		t.Errorf("专项应付款一级余额应 90000（子科目之和），实际 %d", pz.CurrentBalanceCents)
	}
	if pz.IncomeCents != 50000 {
		t.Errorf("专项应付款一级收入应 50000（子科目之和），实际 %d", pz.IncomeCents)
	}
}

// TestCapitalWarning 未分配为负时给出警告。
func TestCapitalWarning(t *testing.T) {
	db, r := newEnv(t)
	_ = settings.NewRepo(db).Upsert("bank_opening_balance_cents", "100")
	l1 := seedCat(t, db, "专项应付款", 1, nil, "active", "residual", 0, false)
	_ = seedCat(t, db, "超支专款", 2, l1, "active", "residual", 50000, true)

	s := getSummary(t, r, "")
	if s.Capital == nil || s.Capital.UnallocatedCents != -49900 {
		t.Fatalf("unallocated 应 -49900，实际 %v", s.Capital)
	}
	if s.Capital.Warning == "" {
		t.Error("未分配为负时应给出 warning")
	}
}

// TestPeriodFilter 资金构成与科目余额不受区间过滤影响；收支小计随区间。
func TestPeriodFilter(t *testing.T) {
	db, r := newEnv(t)
	_ = settings.NewRepo(db).Upsert("bank_opening_balance_cents", "100000")
	l1 := seedCat(t, db, "经营收入", 1, nil, "active", "residual", 0, false)
	cat := seedCat(t, db, "出租收入", 2, l1, "active", "residual", 0, false)
	seedTxn(t, db, "2026-09-01", "income", 50000, cat)
	seedTxn(t, db, "2026-10-01", "income", 70000, cat)

	s := getSummary(t, r, "?from=2026-10-01&to=2026-10-31")
	if s.IncomeTotal != 70000 {
		t.Errorf("10 月收入应 70000，实际 %d", s.IncomeTotal)
	}
	if s.Capital == nil || s.Capital.BankBalanceCents != 220000 {
		t.Errorf("资金构成应全年 220000，实际 %v", s.Capital)
	}
	var root *CategorySummary
	for _, c := range s.Categories {
		if c.Name == "经营收入" {
			root = c
		}
	}
	if root == nil || root.CurrentBalanceCents != 120000 {
		t.Errorf("科目余额应全年 120000，实际 %v", root)
	}
	if root.IncomeCents != 70000 {
		t.Errorf("一级本期收入应 70000（区间），实际 %d", root.IncomeCents)
	}
}
