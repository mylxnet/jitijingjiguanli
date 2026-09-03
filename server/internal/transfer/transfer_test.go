package transfer

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/category"
	"jititaizhang/server/internal/platform"
)

func newEnv(t *testing.T) (*sql.DB, *gin.Engine) {
	t.Helper()
	db, err := platform.Open(filepath.Join(t.TempDir(), "transfer.db"))
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

// seedCat 插入权益二级（kind=equity，v0.4）。level=1 时是一级分组。
func seedCat(t *testing.T, db *sql.DB, name string, level int, parent any, status string) int64 {
	t.Helper()
	now := time.Now().UTC()
	res, err := db.Exec(`INSERT INTO category(org_id, name, level, parent_id, status, kind,
		sort_order, created_at, updated_at)
		VALUES(1,?,?,?,?,'equity',0,?,?)`, name, level, parent, status, now, now)
	if err != nil {
		t.Fatalf("插入科目 %s 失败: %v", name, err)
	}
	id, _ := res.LastInsertId()
	return id
}

// seedIncome 直接给权益科目记一笔收入（等价于先收钱，建立转出余额）。
func seedIncome(t *testing.T, db *sql.DB, catID, amount int64) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := db.Exec(
		`INSERT INTO txn(org_id, txn_date, direction, amount_cents, category_id, note, status, created_at, updated_at)
		 VALUES(1,'2026-09-01','income',?,?,NULL,'normal',?,?)`, amount, catID, now, now); err != nil {
		t.Fatalf("插入收入失败: %v", err)
	}
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

func apiErr(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var out struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析错误体失败: %v body=%s", err, w.Body.String())
	}
	return out.Error.Code
}

func balance(t *testing.T, db *sql.DB, catID int64) int64 {
	t.Helper()
	repo := category.NewRepo(db)
	b, err := repo.CalcBalance(catID)
	if err != nil {
		t.Fatalf("计算余额失败: %v", err)
	}
	return b
}

// TestTransferBalance 权益科目间转账：转出减、转入加。
func TestTransferBalance(t *testing.T) {
	db, r := newEnv(t)
	l1 := seedCat(t, db, "资金", 1, nil, "active")
	src := seedCat(t, db, "修路款", 2, l1, "active")
	dst := seedCat(t, db, "水利款", 2, l1, "active")
	seedIncome(t, db, src, 30000)

	w := doJSON(t, r, "POST", "/api/transfers", map[string]any{
		"txnDate": "2026-09-02", "sourceCategoryId": src, "sourceAmountCents": 20000, "note": "专款调剂",
		"legs": []map[string]any{{"categoryId": dst, "amountCents": 20000}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("转账失败: %d %s", w.Code, w.Body.String())
	}
	if got := balance(t, db, src); got != 10000 {
		t.Errorf("转出后余额应 10000，实际 %d", got)
	}
	if got := balance(t, db, dst); got != 20000 {
		t.Errorf("转入后余额应 20000，实际 %d", got)
	}
}

// TestCreateValidations v0.4 转账校验（资产科目禁止，无花费型规则）。
func TestCreateValidations(t *testing.T) {
	db, r := newEnv(t)
	l1 := seedCat(t, db, "资金", 1, nil, "active")
	src := seedCat(t, db, "修路款", 2, l1, "active")
	dst := seedCat(t, db, "水利款", 2, l1, "active")
	off := seedCat(t, db, "停用科目", 2, l1, "inactive")
	seedIncome(t, db, src, 10000)

	cases := []struct {
		name string
		body map[string]any
		want string
	}{
		{"转出金额为负", map[string]any{"txnDate": "2026-09-01", "sourceCategoryId": src, "sourceAmountCents": -1,
			"legs": []map[string]any{{"categoryId": dst, "amountCents": 1}}}, "INVALID_AMOUNT"},
		{"坏日期", map[string]any{"txnDate": "2026/09/01", "sourceCategoryId": src, "sourceAmountCents": 100,
			"legs": []map[string]any{{"categoryId": dst, "amountCents": 100}}}, "INVALID_DATE"},
		{"转出科目不存在", map[string]any{"txnDate": "2026-09-01", "sourceCategoryId": 99999, "sourceAmountCents": 100,
			"legs": []map[string]any{{"categoryId": dst, "amountCents": 100}}}, "CATEGORY_NOT_FOUND"},
		{"转出科目为一级", map[string]any{"txnDate": "2026-09-01", "sourceCategoryId": l1, "sourceAmountCents": 100,
			"legs": []map[string]any{{"categoryId": dst, "amountCents": 100}}}, "TRANSFER_SOURCE_NOT_LEAF"},
		{"Σ转入≠转出", map[string]any{"txnDate": "2026-09-01", "sourceCategoryId": src, "sourceAmountCents": 100,
			"legs": []map[string]any{{"categoryId": dst, "amountCents": 90}}}, "TRANSFER_AMOUNT_MISMATCH"},
		{"转入=转出科目", map[string]any{"txnDate": "2026-09-01", "sourceCategoryId": src, "sourceAmountCents": 100,
			"legs": []map[string]any{{"categoryId": src, "amountCents": 100}}}, "TRANSFER_SAME_CATEGORY"},
		{"同科目两条转入", map[string]any{"txnDate": "2026-09-01", "sourceCategoryId": src, "sourceAmountCents": 100,
			"legs": []map[string]any{{"categoryId": dst, "amountCents": 60}, {"categoryId": dst, "amountCents": 40}}}, "TRANSFER_DUPLICATE_LEG"},
		{"转入科目为一级", map[string]any{"txnDate": "2026-09-01", "sourceCategoryId": src, "sourceAmountCents": 100,
			"legs": []map[string]any{{"categoryId": l1, "amountCents": 100}}}, "TRANSFER_LEG_NOT_LEAF"},
		{"转入科目停用", map[string]any{"txnDate": "2026-09-01", "sourceCategoryId": src, "sourceAmountCents": 100,
			"legs": []map[string]any{{"categoryId": off, "amountCents": 100}}}, "CATEGORY_INACTIVE"},
		{"转出超余额", map[string]any{"txnDate": "2026-09-01", "sourceCategoryId": src, "sourceAmountCents": 10001,
			"legs": []map[string]any{{"categoryId": dst, "amountCents": 10001}}}, "TRANSFER_INSUFFICIENT_BALANCE"},
	}

	for _, tc := range cases {
		w := doJSON(t, r, "POST", "/api/transfers", tc.body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: 应 400，实际 %d (%s)", tc.name, w.Code, w.Body.String())
			continue
		}
		if got := apiErr(t, w); got != tc.want {
			t.Errorf("%s: 错误码应 %s，实际 %s", tc.name, tc.want, got)
		}
	}

	if got := balance(t, db, src); got != 10000 {
		t.Errorf("校验拒绝后余额应仍 10000，实际 %d", got)
	}
}

// TestVoidAndUnvoid 作废/撤销转账：余额回滚与恢复、留痕。
func TestVoidAndUnvoid(t *testing.T) {
	db, r := newEnv(t)
	l1 := seedCat(t, db, "资金", 1, nil, "active")
	src := seedCat(t, db, "修路款", 2, l1, "active")
	dst := seedCat(t, db, "水利款", 2, l1, "active")
	seedIncome(t, db, src, 30000)

	w := doJSON(t, r, "POST", "/api/transfers", map[string]any{
		"txnDate": "2026-09-01", "sourceCategoryId": src, "sourceAmountCents": 20000,
		"legs": []map[string]any{{"categoryId": dst, "amountCents": 20000}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("建转账失败: %d %s", w.Code, w.Body.String())
	}
	var created struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}
	id := created.Data.ID

	w = doJSON(t, r, "PUT", "/api/transfers/"+itoa(id), map[string]any{"status": "voided"})
	if w.Code != http.StatusOK {
		t.Fatalf("作废失败: %d %s", w.Code, w.Body.String())
	}
	if got := balance(t, db, src); got != 30000 {
		t.Errorf("作废后转出应回 30000，实际 %d", got)
	}
	if got := balance(t, db, dst); got != 0 {
		t.Errorf("作废后转入应回 0，实际 %d", got)
	}

	w = doJSON(t, r, "PUT", "/api/transfers/"+itoa(id), map[string]any{"status": "normal"})
	if w.Code != http.StatusOK {
		t.Fatalf("撤销失败: %d %s", w.Code, w.Body.String())
	}
	if got := balance(t, db, src); got != 10000 {
		t.Errorf("撤销后转出应 10000，实际 %d", got)
	}
	if got := balance(t, db, dst); got != 20000 {
		t.Errorf("撤销后转入应 20000，实际 %d", got)
	}

	var logCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM change_log WHERE entity_type='transfer' AND entity_id=? AND action IN ('void','unvoid')`, id).Scan(&logCount); err != nil {
		t.Fatalf("查变更日志失败: %v", err)
	}
	if logCount != 2 {
		t.Errorf("留痕应 2 条（void+unvoid），实际 %d", logCount)
	}

	w = doJSON(t, r, "PUT", "/api/transfers/99999", map[string]any{"status": "voided"})
	if w.Code != http.StatusNotFound {
		t.Errorf("作废不存在应 404，实际 %d", w.Code)
	}
	w = doJSON(t, r, "PUT", "/api/transfers/"+itoa(id), map[string]any{"status": "bogus"})
	if w.Code != http.StatusBadRequest || apiErr(t, w) != "INVALID_STATUS" {
		t.Errorf("非法状态应 400 INVALID_STATUS，实际 %d %s", w.Code, w.Body.String())
	}
}

// TestListTransfers 列表（排序、日期过滤）。
func TestListTransfers(t *testing.T) {
	db, r := newEnv(t)
	l1 := seedCat(t, db, "资金", 1, nil, "active")
	src := seedCat(t, db, "修路款", 2, l1, "active")
	dstA := seedCat(t, db, "水利款", 2, l1, "active")
	dstB := seedCat(t, db, "公积公益金", 2, l1, "active")
	seedIncome(t, db, src, 1000000)

	doJSON(t, r, "POST", "/api/transfers", map[string]any{
		"txnDate": "2026-09-01", "sourceCategoryId": src, "sourceAmountCents": 50000,
		"legs": []map[string]any{{"categoryId": dstA, "amountCents": 30000}, {"categoryId": dstB, "amountCents": 20000}},
	})
	doJSON(t, r, "POST", "/api/transfers", map[string]any{
		"txnDate": "2026-09-02", "sourceCategoryId": src, "sourceAmountCents": 10000,
		"legs": []map[string]any{{"categoryId": dstB, "amountCents": 10000}},
	})

	var out struct {
		Data struct {
			Items []Transfer `json:"items"`
			Total int        `json:"total"`
		} `json:"data"`
	}

	w := doJSON(t, r, "GET", "/api/transfers", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析列表失败: %v", err)
	}
	if out.Data.Total != 2 {
		t.Fatalf("应有 2 笔转账，实际 %d", out.Data.Total)
	}
	if out.Data.Items[0].TxnDate != "2026-09-02" {
		t.Errorf("列表应按日期倒序，首笔应 09-02，实际 %s", out.Data.Items[0].TxnDate)
	}

	w = doJSON(t, r, "GET", "/api/transfers?from=2026-09-02&to=2026-09-30", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析过滤列表失败: %v", err)
	}
	if out.Data.Total != 1 {
		t.Errorf("日期过滤应命中 1 笔，实际 %d", out.Data.Total)
	}
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}

func seedTestOrg(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO org(id, name, created_at, updated_at) VALUES(1, '测试组织', '2026-09-02', '2026-09-02')`); err != nil {
		t.Fatalf("插入测试组织失败: %v", err)
	}
}

func orgCtx() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("orgID", int64(1))
		c.Set("userID", int64(1))
		c.Next()
	}
}
