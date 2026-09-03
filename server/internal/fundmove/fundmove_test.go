package fundmove

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
	"jititaizhang/server/internal/summary"
)

func newEnv(t *testing.T) (*sql.DB, *gin.Engine) {
	t.Helper()
	db, err := platform.Open(filepath.Join(t.TempDir(), "fundmove.db"))
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
	category.NewHandler(db).Register(authed)
	NewHandler(db).Register(authed)
	return db, r
}

func setBank(t *testing.T, db *sql.DB, opening int64) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO app_setting(org_id, key, value) VALUES(1,'bank_opening_balance_cents',?)
		 ON CONFLICT(org_id, key) DO UPDATE SET value = excluded.value`,
		strconv.FormatInt(opening, 10)); err != nil {
		t.Fatalf("设置银行期初失败: %v", err)
	}
}

// seedCat 建科目：kind=asset 为资产二级（资金划转用），否则权益 equity。
func seedCat(t *testing.T, db *sql.DB, name string, level int, parent any, status, kind string) int64 {
	t.Helper()
	now := time.Now().UTC()
	if kind == "" {
		kind = "equity"
	}
	res, err := db.Exec(`INSERT INTO category(org_id, name, level, parent_id, status, kind,
		sort_order, created_at, updated_at)
		VALUES(1,?,?,?,?,?,0,?,?)`, name, level, parent, status, kind, now, now)
	if err != nil {
		t.Fatalf("插入科目 %s 失败: %v", name, err)
	}
	id, _ := res.LastInsertId()
	return id
}

// seedTxn 直接插入一笔银行收支（模拟记账；equity 科目记收入可提升银行余额与权益）。
func seedTxn(t *testing.T, db *sql.DB, date, direction string, amountCents, catID int64) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := db.Exec(
		`INSERT INTO txn(org_id, txn_date, direction, amount_cents, category_id, note, status, created_at, updated_at)
		 VALUES(1,?,?,?,?,NULL,'normal',?,?)`, date, direction, amountCents, catID, now, now); err != nil {
		t.Fatalf("插入流水失败: %v", err)
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

func assetBalance(t *testing.T, db *sql.DB, catID int64) int64 {
	t.Helper()
	repo := category.NewRepo(db)
	b, err := repo.AssetBalance(catID)
	if err != nil {
		t.Fatalf("计算资产余额失败: %v", err)
	}
	return b
}

func bankDelta(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	repo := category.NewRepo(db)
	d, err := repo.BankDelta(1)
	if err != nil {
		t.Fatalf("计算资金划转银行净额失败: %v", err)
	}
	return d
}

func capital(t *testing.T, db *sql.DB) *summary.Capital {
	t.Helper()
	s := summary.NewRepo(db)
	resp, err := s.GetSummary(1, "", "")
	if err != nil {
		t.Fatalf("取汇总失败: %v", err)
	}
	return resp.Capital
}

func createMove(t *testing.T, r *gin.Engine, date, kind string, catID, amount int64, note string) int64 {
	t.Helper()
	w := doJSON(t, r, "POST", "/api/fund-moves", map[string]any{
		"moveDate": date, "kind": kind, "assetCategoryId": catID, "amountCents": amount, "note": note,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("创建资金划转失败: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}
	return out.Data.ID
}

// TestInvestRecover v0.4：投资/收回改变银行与资产；权益只随到账收入变化（收付实现）。
func TestInvestRecover(t *testing.T) {
	db, r := newEnv(t)
	setBank(t, db, 2000000)

	l1Fund := seedCat(t, db, "本金", 1, nil, "active", "equity")
	principal := seedCat(t, db, "上级补助", 2, l1Fund, "active", "equity")
	seedTxn(t, db, "2026-09-01", "income", 1000000, principal)

	l1Inv := seedCat(t, db, "对外投资", 1, nil, "active", "equity")
	investA := seedCat(t, db, "项目A", 2, l1Inv, "active", "asset")

	createMove(t, r, "2026-09-02", "invest", investA, 600000, "投资")
	if got := assetBalance(t, db, investA); got != 600000 {
		t.Errorf("投资后资产余额应 600000，实际 %d", got)
	}
	if got := bankDelta(t, db); got != -600000 {
		t.Errorf("投资后银行净额应 -600000，实际 %d", got)
	}
	c := capital(t, db)
	if c.BankBalanceCents != 2400000 {
		t.Errorf("银行应 2400000，实际 %d", c.BankBalanceCents)
	}
	if c.AssetTotalCents != 600000 {
		t.Errorf("资产合计应 600000，实际 %d", c.AssetTotalCents)
	}
	if c.EquityTotalCents != 1000000 {
		t.Errorf("权益合计应 1000000（只计到账收入），实际 %d", c.EquityTotalCents)
	}

	recoverID := createMove(t, r, "2026-09-10", "recover", investA, 200000, "收回")
	c = capital(t, db)
	if c.BankBalanceCents != 2600000 || c.AssetTotalCents != 400000 {
		t.Errorf("收回后 bank/asset 不对：%d/%d", c.BankBalanceCents, c.AssetTotalCents)
	}
	if c.EquityTotalCents != 1000000 {
		t.Errorf("收回不改变权益，应 1000000，实际 %d", c.EquityTotalCents)
	}

	w := doJSON(t, r, "PUT", "/api/fund-moves/"+itoa(recoverID), map[string]any{"status": "voided"})
	if w.Code != http.StatusOK {
		t.Fatalf("作废收回失败: %d %s", w.Code, w.Body.String())
	}
	if got := assetBalance(t, db, investA); got != 600000 {
		t.Errorf("作废收回后资产应 600000，实际 %d", got)
	}
}

// TestCreateValidations 校验矩阵。
func TestCreateValidations(t *testing.T) {
	db, r := newEnv(t)
	setBank(t, db, 100000)

	l1 := seedCat(t, db, "对外投资", 1, nil, "active", "equity")
	asset := seedCat(t, db, "项目A", 2, l1, "active", "asset")
	normal := seedCat(t, db, "投资收益", 2, l1, "active", "equity")
	inactive := seedCat(t, db, "停用资产", 2, l1, "inactive", "asset")

	cases := []struct {
		name string
		body map[string]any
		want string
	}{
		{"金额为负", map[string]any{"moveDate": "2026-09-01", "kind": "invest", "assetCategoryId": asset, "amountCents": -1}, "INVALID_AMOUNT"},
		{"金额为零", map[string]any{"moveDate": "2026-09-01", "kind": "invest", "assetCategoryId": asset, "amountCents": 0}, "INVALID_AMOUNT"},
		{"坏日期", map[string]any{"moveDate": "2026/09/01", "kind": "invest", "assetCategoryId": asset, "amountCents": 100}, "INVALID_DATE"},
		{"方向非法", map[string]any{"moveDate": "2026-09-01", "kind": "sideways", "assetCategoryId": asset, "amountCents": 100}, "INVALID_REQUEST"},
		{"科目不存在", map[string]any{"moveDate": "2026-09-01", "kind": "invest", "assetCategoryId": 88888, "amountCents": 100}, "CATEGORY_NOT_FOUND"},
		{"科目为一级", map[string]any{"moveDate": "2026-09-01", "kind": "invest", "assetCategoryId": l1, "amountCents": 100}, "FUND_MOVE_NOT_ASSET"},
		{"科目为权益", map[string]any{"moveDate": "2026-09-01", "kind": "invest", "assetCategoryId": normal, "amountCents": 100}, "FUND_MOVE_NOT_ASSET"},
		{"科目停用", map[string]any{"moveDate": "2026-09-01", "kind": "invest", "assetCategoryId": inactive, "amountCents": 100}, "CATEGORY_INACTIVE"},
		{"投资超银行余额", map[string]any{"moveDate": "2026-09-01", "kind": "invest", "assetCategoryId": asset, "amountCents": 100001}, "FUND_MOVE_INSUFFICIENT_BANK"},
		{"收回超资产在外金额", map[string]any{"moveDate": "2026-09-01", "kind": "recover", "assetCategoryId": asset, "amountCents": 1}, "FUND_MOVE_INSUFFICIENT_ASSET"},
	}

	for _, tc := range cases {
		w := doJSON(t, r, "POST", "/api/fund-moves", tc.body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: 应 400，实际 %d", tc.name, w.Code)
			continue
		}
		if got := apiErr(t, w); got != tc.want {
			t.Errorf("%s: 错误码应 %s，实际 %s", tc.name, tc.want, got)
		}
	}

	if got := assetBalance(t, db, asset); got != 0 {
		t.Errorf("校验拒绝后资产余额应 0，实际 %d", got)
	}
}

// TestVoidAndUnvoid 作废/撤销 + 留痕。
func TestVoidAndUnvoid(t *testing.T) {
	db, r := newEnv(t)
	setBank(t, db, 500000)

	l1 := seedCat(t, db, "对外投资", 1, nil, "active", "equity")
	asset := seedCat(t, db, "项目A", 2, l1, "active", "asset")

	id := createMove(t, r, "2026-09-01", "invest", asset, 300000, "投资")
	if got := assetBalance(t, db, asset); got != 300000 {
		t.Fatalf("投资后资产余额应 300000，实际 %d", got)
	}

	w := doJSON(t, r, "PUT", "/api/fund-moves/"+itoa(id), map[string]any{"status": "voided"})
	if w.Code != http.StatusOK {
		t.Fatalf("作废失败: %d %s", w.Code, w.Body.String())
	}
	if got := assetBalance(t, db, asset); got != 0 {
		t.Errorf("作废后资产余额应 0，实际 %d", got)
	}
	if got := bankDelta(t, db); got != 0 {
		t.Errorf("作废后银行净额应 0，实际 %d", got)
	}

	w = doJSON(t, r, "PUT", "/api/fund-moves/"+itoa(id), map[string]any{"status": "normal"})
	if w.Code != http.StatusOK {
		t.Fatalf("撤销失败: %d %s", w.Code, w.Body.String())
	}
	if got := assetBalance(t, db, asset); got != 300000 {
		t.Errorf("撤销后资产余额应 300000，实际 %d", got)
	}

	var logCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM change_log WHERE entity_type='fund_move' AND entity_id=? AND action IN ('create','void','unvoid')`, id).Scan(&logCount); err != nil {
		t.Fatalf("查变更日志失败: %v", err)
	}
	if logCount != 3 {
		t.Errorf("留痕应 3 条（create+void+unvoid），实际 %d", logCount)
	}
}

// TestListAndDeleteGuard 列表与删除保护。
func TestListAndDeleteGuard(t *testing.T) {
	db, r := newEnv(t)
	setBank(t, db, 1000000)

	l1 := seedCat(t, db, "对外投资", 1, nil, "active", "equity")
	assetA := seedCat(t, db, "项目A", 2, l1, "active", "asset")
	assetB := seedCat(t, db, "项目B", 2, l1, "active", "asset")

	createMove(t, r, "2026-09-01", "invest", assetA, 400000, "投A")
	createMove(t, r, "2026-09-02", "invest", assetB, 200000, "投B")
	createMove(t, r, "2026-10-01", "recover", assetA, 100000, "收A部分")

	var out struct {
		Data struct {
			Items []FundMove `json:"items"`
			Total int        `json:"total"`
		} `json:"data"`
	}
	w := doJSON(t, r, "GET", "/api/fund-moves", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析列表失败: %v", err)
	}
	if out.Data.Total != 3 {
		t.Fatalf("应有 3 笔资金划转，实际 %d", out.Data.Total)
	}
	if out.Data.Items[0].MoveDate != "2026-10-01" {
		t.Errorf("列表应按日期倒序，首笔应 10-01，实际 %s", out.Data.Items[0].MoveDate)
	}

	w = doJSON(t, r, "GET", "/api/fund-moves?kind=recover", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析过滤列表失败: %v", err)
	}
	if out.Data.Total != 1 {
		t.Errorf("recover 过滤应命中 1 笔，实际 %d", out.Data.Total)
	}

	// 有划转记录的资产科目删除被拒
	w = doJSON(t, r, "DELETE", "/api/categories/"+itoa(assetA), nil)
	if w.Code != http.StatusConflict || apiErr(t, w) != "CATEGORY_IN_USE" {
		t.Errorf("有资金划转的资产科目删除应 409 CATEGORY_IN_USE，实际 %d %s", w.Code, w.Body.String())
	}

	// 跨组织不可见
	repo := NewRepo(db)
	items, total, err := repo.List(2, "", "", "", 1, 50)
	if err != nil {
		t.Fatalf("组织2查询失败: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Errorf("组织 2 应看不到资金划转，实际 total=%d", total)
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
