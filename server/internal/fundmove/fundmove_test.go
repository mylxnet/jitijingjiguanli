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

// setBank 设置银行存款期初余额（分）。
func setBank(t *testing.T, db *sql.DB, opening int64) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO app_setting(org_id, key, value) VALUES(1,'bank_opening_balance_cents',?)
		 ON CONFLICT(org_id, key) DO UPDATE SET value = excluded.value`,
		strconv.FormatInt(opening, 10)); err != nil {
		t.Fatalf("设置银行期初失败: %v", err)
	}
}

// seedCat 插入科目；kind：normal/asset；asset 科目必须 level=2、residual。
func seedCat(t *testing.T, db *sql.DB, name string, level int, parent any, status, bt, kind string, opening int64, recon bool) int64 {
	t.Helper()
	now := time.Now().UTC()
	inc := 0
	if recon {
		inc = 1
	}
	res, err := db.Exec(`INSERT INTO category(org_id, name, level, parent_id, status, balance_type, kind,
		opening_balance_cents, include_in_reconciliation, sort_order, created_at, updated_at)
		VALUES(1,?,?,?,?,?,?,?,?,0,?,?)`, name, level, parent, status, bt, kind, opening, inc, now, now)
	if err != nil {
		t.Fatalf("插入科目 %s 失败: %v", name, err)
	}
	id, _ := res.LastInsertId()
	return id
}

// seedTxn 直接插入一笔银行收支流水（模拟 transaction 包记账）。
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

// capital 通过 summary.Repo 取资金构成（验证划转是否被资金构成正确吸收）。
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

// ---------- T1：D10 场景验算（投资/收回改变银行与资产，不影响未分配恒等） ----------

func TestD10InvestRecover(t *testing.T) {
	db, r := newEnv(t)
	setBank(t, db, 2000000) // 银行存款期初 200 万（分）

	// 本金（余粮·勾稽）：收到上级拨款 100 万入账
	l1Fund := seedCat(t, db, "本金", 1, nil, "active", "residual", "normal", 0, false)
	principal := seedCat(t, db, "本金", 2, l1Fund, "active", "residual", "normal", 0, true)
	seedTxn(t, db, "2026-09-01", "income", 1000000, principal)

	// 对外投资一级 + 资产二级
	l1Inv := seedCat(t, db, "对外投资", 1, nil, "active", "residual", "normal", 0, false)
	investA := seedCat(t, db, "项目A", 2, l1Inv, "active", "residual", "asset", 0, false)

	// 投出 60 万（银行→资产）
	createMove(t, r, "2026-09-02", "invest", investA, 600000, "项目A投资")

	// 资产科目余额 = 60 万；银行净影响 −60 万
	if got := assetBalance(t, db, investA); got != 600000 {
		t.Errorf("投资后资产科目余额应 600000，实际 %d", got)
	}
	if got := bankDelta(t, db); got != -600000 {
		t.Errorf("投资后银行净额应 -600000，实际 %d", got)
	}

	// 年收益 10 万入经营收入（银行 +10 万）
	l1Biz := seedCat(t, db, "经营收入", 1, nil, "active", "residual", "normal", 0, false)
	dividend := seedCat(t, db, "投资收益", 2, l1Biz, "active", "residual", "normal", 0, false)
	seedTxn(t, db, "2026-09-30", "income", 100000, dividend)

	// 资金构成：bank = 200万 +100万 −60万 +10万 = 250万；asset=60万；勾稽(本金)=100万；未分配=210万
	c := capital(t, db)
	if c.BankBalanceCents != 2500000 {
		t.Errorf("银行存款应 2500000，实际 %d", c.BankBalanceCents)
	}
	if c.AssetTotalCents != 600000 {
		t.Errorf("资产合计应 600000，实际 %d", c.AssetTotalCents)
	}
	if c.EarmarkedCents != 1000000 {
		t.Errorf("专项资金应 1000000，实际 %d", c.EarmarkedCents)
	}
	if c.UnallocatedCents != 2100000 {
		t.Errorf("未分配应 2100000（收益后恒等），实际 %d", c.UnallocatedCents)
	}

	// 收回 20 万（资产→银行）：银行 +20 万，资产 −20 万，未分配不变
	recoverID := createMove(t, r, "2026-10-01", "recover", investA, 200000, "部分收回")
	if got := assetBalance(t, db, investA); got != 400000 {
		t.Errorf("收回后资产科目余额应 400000，实际 %d", got)
	}
	c = capital(t, db)
	if c.BankBalanceCents != 2700000 {
		t.Errorf("收回后银行存款应 2700000，实际 %d", c.BankBalanceCents)
	}
	if c.AssetTotalCents != 400000 {
		t.Errorf("收回后资产合计应 400000，实际 %d", c.AssetTotalCents)
	}
	if c.UnallocatedCents != 2100000 {
		t.Errorf("收回不改变未分配，应 2100000，实际 %d", c.UnallocatedCents)
	}

	// 作废收回单：资产回到 60 万、银行回到 250 万，未分配仍恒等
	w := doJSON(t, r, "PUT", "/api/fund-moves/"+itoa(recoverID), map[string]any{"status": "voided"})
	if w.Code != http.StatusOK {
		t.Fatalf("作废收回失败: %d %s", w.Code, w.Body.String())
	}
	if got := assetBalance(t, db, investA); got != 600000 {
		t.Errorf("作废收回后资产科目余额应回到 600000，实际 %d", got)
	}
	c = capital(t, db)
	if c.BankBalanceCents != 2500000 {
		t.Errorf("作废收回后银行存款应 2500000，实际 %d", c.BankBalanceCents)
	}
	if c.UnallocatedCents != 2100000 {
		t.Errorf("作废收回后未分配应仍 2100000，实际 %d", c.UnallocatedCents)
	}
}

// ---------- T2：创建校验矩阵 ----------

func TestCreateValidations(t *testing.T) {
	db, r := newEnv(t)
	setBank(t, db, 100000) // 银行期初 10 万

	l1 := seedCat(t, db, "对外投资", 1, nil, "active", "residual", "normal", 0, false)
	asset := seedCat(t, db, "项目A", 2, l1, "active", "residual", "asset", 0, false)
	normal := seedCat(t, db, "办公费", 2, l1, "active", "residual", "normal", 0, false)
	inactive := seedCat(t, db, "停用资产", 2, l1, "inactive", "residual", "asset", 0, false)
	otherOrg := int64(99999) // 不存在组织

	_ = otherOrg
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
		{"科目为普通型", map[string]any{"moveDate": "2026-09-01", "kind": "invest", "assetCategoryId": normal, "amountCents": 100}, "FUND_MOVE_NOT_ASSET"},
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

	// 全部拒绝后无脏数据
	if got := assetBalance(t, db, asset); got != 0 {
		t.Errorf("校验拒绝后资产余额应 0，实际 %d", got)
	}
}

// ---------- T3：作废 / 撤销（回滚与恢复 + 留痕） ----------

func TestVoidAndUnvoid(t *testing.T) {
	db, r := newEnv(t)
	setBank(t, db, 500000) // 银行期初 50 万

	l1 := seedCat(t, db, "对外投资", 1, nil, "active", "residual", "normal", 0, false)
	asset := seedCat(t, db, "项目A", 2, l1, "active", "residual", "asset", 0, false)

	id := createMove(t, r, "2026-09-01", "invest", asset, 300000, "投项目A")
	if got := assetBalance(t, db, asset); got != 300000 {
		t.Fatalf("投资后资产余额应 300000，实际 %d", got)
	}

	// 作废：资产归零，银行净额归零
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

	// 撤销：恢复
	w = doJSON(t, r, "PUT", "/api/fund-moves/"+itoa(id), map[string]any{"status": "normal"})
	if w.Code != http.StatusOK {
		t.Fatalf("撤销失败: %d %s", w.Code, w.Body.String())
	}
	if got := assetBalance(t, db, asset); got != 300000 {
		t.Errorf("撤销后资产余额应 300000，实际 %d", got)
	}

	// 留痕：create + void + unvoid
	var logCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM change_log WHERE entity_type='fund_move' AND entity_id=? AND action IN ('create','void','unvoid')`, id).Scan(&logCount); err != nil {
		t.Fatalf("查变更日志失败: %v", err)
	}
	if logCount != 3 {
		t.Errorf("变更日志应 3 条（create+void+unvoid），实际 %d", logCount)
	}

	// 作废不存在 / 非法状态
	w = doJSON(t, r, "PUT", "/api/fund-moves/99999", map[string]any{"status": "voided"})
	if w.Code != http.StatusNotFound {
		t.Errorf("作废不存在应 404，实际 %d", w.Code)
	}
	w = doJSON(t, r, "PUT", "/api/fund-moves/"+itoa(id), map[string]any{"status": "bogus"})
	if w.Code != http.StatusBadRequest || apiErr(t, w) != "INVALID_STATUS" {
		t.Errorf("非法状态应 400 INVALID_STATUS，实际 %d %s", w.Code, w.Body.String())
	}
}

// ---------- T4：列表（排序、类型过滤、日期过滤） ----------

func TestListFundMoves(t *testing.T) {
	db, r := newEnv(t)
	setBank(t, db, 1000000)

	l1 := seedCat(t, db, "对外投资", 1, nil, "active", "residual", "normal", 0, false)
	assetA := seedCat(t, db, "项目A", 2, l1, "active", "residual", "asset", 0, false)
	assetB := seedCat(t, db, "项目B", 2, l1, "active", "residual", "asset", 0, false)

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

	// 按类型过滤 recover → 1 笔
	w = doJSON(t, r, "GET", "/api/fund-moves?kind=recover", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析过滤列表失败: %v", err)
	}
	if out.Data.Total != 1 || out.Data.Items[0].Kind != "recover" {
		t.Errorf("recover 过滤应命中 1 笔，实际 %d", out.Data.Total)
	}

	// 日期范围 09-02 ~ 09-30 → 命中 invest 两笔中的 09-02 那笔（1 笔）
	w = doJSON(t, r, "GET", "/api/fund-moves?from=2026-09-02&to=2026-09-30", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析日期过滤列表失败: %v", err)
	}
	if out.Data.Total != 1 {
		t.Errorf("日期过滤应命中 1 笔，实际 %d", out.Data.Total)
	}

	// 跨组织不可见：其他组织无数据（本测试库只含 org=1）
	repo := NewRepo(db)
	items, total, err := repo.List(2, "", "", "", 1, 50)
	if err != nil {
		t.Fatalf("查询组织2失败: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Errorf("组织 2 应看不到任何资金划转，实际 total=%d", total)
	}
}

// ---------- T5：科目删除保护（有资金划转的资产科目不可删） ----------

func TestDeleteGuard(t *testing.T) {
	db, r := newEnv(t)
	setBank(t, db, 500000)

	l1 := seedCat(t, db, "对外投资", 1, nil, "active", "residual", "normal", 0, false)
	asset := seedCat(t, db, "项目A", 2, l1, "active", "residual", "asset", 0, false)
	createMove(t, r, "2026-09-01", "invest", asset, 100000, "投A")

	// 有划转记录 → 删除被拒
	w := doJSON(t, r, "DELETE", "/api/categories/"+itoa(asset), nil)
	if w.Code != http.StatusConflict || apiErr(t, w) != "CATEGORY_IN_USE" {
		t.Errorf("有资金划转的资产科目删除应 409 CATEGORY_IN_USE，实际 %d %s", w.Code, w.Body.String())
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
