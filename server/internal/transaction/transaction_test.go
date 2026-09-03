package transaction

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/platform"
)

func newEnv(t *testing.T) (*sql.DB, *gin.Engine) {
	t.Helper()
	db, err := platform.Open(filepath.Join(t.TempDir(), "txn.db"))
	if err != nil {
		t.Fatalf("打开临时库失败: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := platform.Migrate(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	seedTestOrg(t, db)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	authed := r.Group("", orgCtx())
	NewHandler(db).Register(authed)
	return db, r
}

func seedCat(t *testing.T, db *sql.DB, name string, level int, parent any, status, bt string) int64 {
	t.Helper()
	now := time.Now().UTC()
	res, err := db.Exec(`INSERT INTO category(org_id, name, level, parent_id, status, balance_type,
		opening_balance_cents, include_in_reconciliation, sort_order, created_at, updated_at)
		VALUES(1,?,?,?,?,?,0,0,0,?,?)`, name, level, parent, status, bt, now, now)
	if err != nil {
		t.Fatalf("插入科目 %s 失败: %v", name, err)
	}
	id, _ := res.LastInsertId()
	return id
}

func doJSON(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("编码请求失败: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func createTxnOK(t *testing.T, r *gin.Engine, date, dir string, amount, catID int64, note string) *Transaction {
	t.Helper()
	w := doJSON(t, r, http.MethodPost, "/api/transactions", map[string]any{
		"txnDate": date, "direction": dir, "amountCents": amount, "categoryId": catID, "note": note,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("创建流水失败: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Data *Transaction `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	return out.Data
}

func TestCreateListSummary(t *testing.T) {
	db, r := newEnv(t)
	l1 := seedCat(t, db, "经营收入", 1, nil, "active", "residual")
	cat := seedCat(t, db, "出租收入", 2, l1, "active", "residual")

	createTxnOK(t, r, "2026-09-01", "income", 50000, cat, "房租")
	createTxnOK(t, r, "2026-09-02", "expense", 3000, cat, "维修")

	w := doJSON(t, r, http.MethodGet, "/api/transactions?from=2026-09-01&to=2026-09-30", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("列表应 200，实际 %d", w.Code)
	}
	var out struct {
		Data ListTransactionsResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析失败: %v body=%s", err, w.Body.String())
	}
	if out.Data.Total != 2 {
		t.Errorf("总数应 2，实际 %d", out.Data.Total)
	}
	if out.Data.Summary.IncomeTotal != 50000 || out.Data.Summary.ExpenseTotal != 3000 || out.Data.Summary.Balance != 47000 {
		t.Errorf("汇总应 50000/3000/47000，实际 %d/%d/%d",
			out.Data.Summary.IncomeTotal, out.Data.Summary.ExpenseTotal, out.Data.Summary.Balance)
	}
	// 关键字搜索
	w = doJSON(t, r, http.MethodGet, "/api/transactions?keyword=房租", nil)
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.Data.Total != 1 {
		t.Errorf("关键字搜索应命中 1 条，实际 %d", out.Data.Total)
	}
}

func TestCreateValidations(t *testing.T) {
	db, r := newEnv(t)
	l1 := seedCat(t, db, "管理费用", 1, nil, "active", "residual")
	active := seedCat(t, db, "办公费", 2, l1, "active", "spending")
	inactive := seedCat(t, db, "停用费", 2, l1, "inactive", "spending")

	cases := []struct {
		name string
		body map[string]any
	}{
		{"金额零", map[string]any{"txnDate": "2026-09-01", "direction": "expense", "amountCents": 0, "categoryId": active}},
		{"金额负", map[string]any{"txnDate": "2026-09-01", "direction": "expense", "amountCents": -5, "categoryId": active}},
		{"日期坏", map[string]any{"txnDate": "2026/09/01", "direction": "expense", "amountCents": 100, "categoryId": active}},
		{"方向坏", map[string]any{"txnDate": "2026-09-01", "direction": "transfer", "amountCents": 100, "categoryId": active}},
		{"科目不存在", map[string]any{"txnDate": "2026-09-01", "direction": "expense", "amountCents": 100, "categoryId": 9999}},
		{"一级科目", map[string]any{"txnDate": "2026-09-01", "direction": "expense", "amountCents": 100, "categoryId": l1}},
		{"停用科目", map[string]any{"txnDate": "2026-09-01", "direction": "expense", "amountCents": 100, "categoryId": inactive}},
	}
	for _, tc := range cases {
		w := doJSON(t, r, http.MethodPost, "/api/transactions", tc.body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: 应 400，实际 %d body=%s", tc.name, w.Code, w.Body.String())
		}
	}
}

func TestUpdateVoidAndChangelog(t *testing.T) {
	db, r := newEnv(t)
	l1 := seedCat(t, db, "管理费用", 1, nil, "active", "residual")
	cat := seedCat(t, db, "办公费", 2, l1, "active", "spending")
	other := seedCat(t, db, "差旅费", 2, l1, "active", "spending")

	txn := createTxnOK(t, r, "2026-09-01", "expense", 1000, cat, "打印纸")

	// 编辑金额 + 换科目 + 改摘要
	w := doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/transactions/%d", txn.ID), map[string]any{
		"amountCents": 1500, "categoryId": other, "note": "打印纸×3", "date": "2026-09-02",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("编辑应 200，实际 %d %s", w.Code, w.Body.String())
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM change_log WHERE entity_type='transaction' AND entity_id=? AND action='update'`, txn.ID).Scan(&n); err != nil {
		t.Fatalf("查询留痕失败: %v", err)
	}
	if n < 3 {
		t.Errorf("编辑应至少 3 条 update 留痕（金额/科目/摘要/日期），实际 %d", n)
	}

	// 作废 → 从默认列表消失、汇总归零；留痕 action=void
	w = doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/transactions/%d", txn.ID), map[string]any{"status": "voided"})
	if w.Code != http.StatusOK {
		t.Fatalf("作废应 200，实际 %d", w.Code)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM change_log WHERE entity_type='transaction' AND entity_id=? AND action='void'`, txn.ID).Scan(&n); err != nil {
		t.Fatalf("查询作废留痕失败: %v", err)
	}
	if n != 1 {
		t.Errorf("作废留痕应 1 条 action=void，实际 %d", n)
	}
	w = doJSON(t, r, http.MethodGet, "/api/transactions", nil)
	var list struct {
		Data ListTransactionsResponse `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if list.Data.Total != 0 {
		t.Errorf("作废后默认列表应为空，实际 %d", list.Data.Total)
	}
	if list.Data.Summary.ExpenseTotal != 0 {
		t.Errorf("作废后默认汇总应为 0，实际 %d", list.Data.Summary.ExpenseTotal)
	}

	// includeVoided=true 时可见且汇总口径一致
	w = doJSON(t, r, http.MethodGet, "/api/transactions?includeVoided=true", nil)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if list.Data.Total != 1 || list.Data.Summary.ExpenseTotal != 1500 {
		t.Errorf("includeVoided 应见 1 条且汇总 1500，实际 total=%d sum=%d", list.Data.Total, list.Data.Summary.ExpenseTotal)
	}

	// 撤销作废 → 恢复可见；留痕 action=unvoid
	w = doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/transactions/%d", txn.ID), map[string]any{"status": "normal"})
	if w.Code != http.StatusOK {
		t.Fatalf("撤销应 200，实际 %d", w.Code)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM change_log WHERE entity_type='transaction' AND entity_id=? AND action='unvoid'`, txn.ID).Scan(&n); err != nil {
		t.Fatalf("查询撤销留痕失败: %v", err)
	}
	if n != 1 {
		t.Errorf("撤销留痕应 1 条 action=unvoid，实际 %d", n)
	}
}

func TestUpdateValidations(t *testing.T) {
	db, r := newEnv(t)
	l1 := seedCat(t, db, "管理费用", 1, nil, "active", "residual")
	cat := seedCat(t, db, "办公费", 2, l1, "active", "spending")
	txn := createTxnOK(t, r, "2026-09-01", "expense", 1000, cat, "")

	cases := []struct {
		name string
		body map[string]any
	}{
		{"金额零", map[string]any{"amountCents": 0}},
		{"金额负", map[string]any{"amountCents": -1}},
		{"日期坏", map[string]any{"date": "x"}},
		{"方向坏", map[string]any{"direction": "bad"}},
		{"状态坏", map[string]any{"status": "deleted"}},
		{"换一级科目", map[string]any{"categoryId": l1}},
		{"换不存在科目", map[string]any{"categoryId": 4242}},
	}
	for _, tc := range cases {
		w := doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/transactions/%d", txn.ID), tc.body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: 应 400，实际 %d body=%s", tc.name, w.Code, w.Body.String())
		}
	}
}

// seedTestOrg 插入固定测试组织（id=1，每个测试库独立，首个组织 id 恒为 1）。
func seedTestOrg(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO org(id, name, created_at, updated_at) VALUES(1, '测试组织', '2026-09-02', '2026-09-02')`); err != nil {
		t.Fatalf("插入测试组织失败: %v", err)
	}
}

// orgCtx 测试中间件：把固定组织/用户写入 gin 上下文（等价于登录态）。
func orgCtx() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("orgID", int64(1))
		c.Set("userID", int64(1))
		c.Next()
	}
}

