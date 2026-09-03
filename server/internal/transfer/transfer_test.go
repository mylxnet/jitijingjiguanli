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
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewHandler(db).Register(r)
	return db, r
}

// seedCat 插入科目，返回 id。opening 为期初余额（分），recon 是否参与资金勾稽。
func seedCat(t *testing.T, db *sql.DB, name string, level int, parent any, status, bt string, opening int64, recon bool) int64 {
	t.Helper()
	now := time.Now().UTC()
	inc := 0
	if recon {
		inc = 1
	}
	res, err := db.Exec(`INSERT INTO category(name, level, parent_id, status, balance_type,
		opening_balance_cents, include_in_reconciliation, sort_order, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,0,?,?)`, name, level, parent, status, bt, opening, inc, now, now)
	if err != nil {
		t.Fatalf("插入科目 %s 失败: %v", name, err)
	}
	id, _ := res.LastInsertId()
	return id
}

// seedTxn 直接插入一笔收支（模拟 transaction 包记账），返回金额对科目余额的影响。
func seedTxn(t *testing.T, db *sql.DB, date, direction string, amountCents, catID int64) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := db.Exec(
		`INSERT INTO txn(txn_date, direction, amount_cents, category_id, note, status, created_at, updated_at)
		 VALUES(?,?,?,?,NULL,'normal',?,?)`, date, direction, amountCents, catID, now, now); err != nil {
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

// apiErr 解析统一错误体，返回 code。
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

// ---------- T1：D7 五步验算（转账影响科目余额、作废回滚） ----------

func TestD7FiveStep(t *testing.T) {
	db, r := newEnv(t)

	// 科目：专项资金（余粮·勾稽）、经营收入/本年收益（余粮·不勾稽）、经营支出（花费型）
	l1Fund := seedCat(t, db, "专项应付款", 1, nil, "active", "residual", 0, false)
	road := seedCat(t, db, "修路款", 2, l1Fund, "active", "residual", 30000, true) // 期初 3 万，勾稽
	water := seedCat(t, db, "水利款", 2, l1Fund, "active", "residual", 0, true)    // 勾稽

	l1Biz := seedCat(t, db, "经营收支", 1, nil, "active", "residual", 0, false)
	revenue := seedCat(t, db, "经营收入", 2, l1Biz, "active", "residual", 0, false)
	profit := seedCat(t, db, "本年收益", 2, l1Biz, "active", "residual", 0, false)

	l1Exp := seedCat(t, db, "经营支出类", 1, nil, "active", "residual", 0, false)
	expense := seedCat(t, db, "经营支出", 2, l1Exp, "active", "spending", 0, false)

	// ① 转账：修路款 → 水利款 2 万
	w := doJSON(t, r, "POST", "/api/transfers", map[string]any{
		"txnDate": "2026-09-01", "sourceCategoryId": road, "sourceAmountCents": 20000, "note": "专款调剂",
		"legs": []map[string]any{{"categoryId": water, "amountCents": 20000}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("① 转账失败: %d %s", w.Code, w.Body.String())
	}
	if got := balance(t, db, road); got != 10000 {
		t.Errorf("① 修路款余额应 10000，实际 %d", got)
	}
	if got := balance(t, db, water); got != 20000 {
		t.Errorf("① 水利款余额应 20000，实际 %d", got)
	}

	// ② 收经营收入 6 万；③ 支经营支出 4 万（直接插 txn 模拟，transaction 包已有覆盖）
	seedTxn(t, db, "2026-09-02", "income", 60000, revenue)
	seedTxn(t, db, "2026-09-03", "expense", 40000, expense)
	if got := balance(t, db, revenue); got != 60000 {
		t.Errorf("② 经营收入余额应 60000，实际 %d", got)
	}
	if got := balance(t, db, expense); got != 40000 {
		t.Errorf("③ 经营支出余额应 40000（花费型累计），实际 %d", got)
	}

	// ④ 期末结转：花费型科目经营支出（余额 40000）→ 本年收益（R4 允许作转出方）
	w = doJSON(t, r, "POST", "/api/transfers", map[string]any{
		"txnDate": "2026-12-31", "sourceCategoryId": expense, "sourceAmountCents": 40000, "note": "年末结转",
		"legs": []map[string]any{{"categoryId": profit, "amountCents": 40000}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("④ 结转失败: %d %s", w.Code, w.Body.String())
	}
	if got := balance(t, db, expense); got != 0 {
		t.Errorf("④ 经营支出结转后余额应 0，实际 %d", got)
	}
	if got := balance(t, db, profit); got != 40000 {
		t.Errorf("④ 本年收益应 40000，实际 %d", got)
	}

	// ⑤ 结转：经营收入 → 本年收益 6 万
	w = doJSON(t, r, "POST", "/api/transfers", map[string]any{
		"txnDate": "2026-12-31", "sourceCategoryId": revenue, "sourceAmountCents": 60000, "note": "年末结转",
		"legs": []map[string]any{{"categoryId": profit, "amountCents": 60000}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("⑤ 结转失败: %d %s", w.Code, w.Body.String())
	}
	if got := balance(t, db, revenue); got != 0 {
		t.Errorf("⑤ 经营收入结转后余额应 0，实际 %d", got)
	}
	if got := balance(t, db, profit); got != 100000 {
		t.Errorf("⑤ 本年收益应 100000，实际 %d", got)
	}
}

// ---------- T2：创建校验矩阵（R2/R3/R4 + 基础校验） ----------

func TestCreateValidations(t *testing.T) {
	db, r := newEnv(t)

	l1 := seedCat(t, db, "资金", 1, nil, "active", "residual", 0, false)
	src := seedCat(t, db, "修路款", 2, l1, "active", "residual", 10000, true)  // 余额 1 万
	dst := seedCat(t, db, "水利款", 2, l1, "active", "residual", 0, true)      // 余粮可转入
	spd := seedCat(t, db, "办公费", 2, l1, "active", "spending", 0, false)     // 花费型
	off := seedCat(t, db, "停用科目", 2, l1, "inactive", "residual", 0, false) // 停用
	_ = off

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
		{"转入金额为负", map[string]any{"txnDate": "2026-09-01", "sourceCategoryId": src, "sourceAmountCents": 100,
			"legs": []map[string]any{{"categoryId": dst, "amountCents": -100}}}, "INVALID_AMOUNT"},
		{"Σ转入≠转出", map[string]any{"txnDate": "2026-09-01", "sourceCategoryId": src, "sourceAmountCents": 100,
			"legs": []map[string]any{{"categoryId": dst, "amountCents": 90}}}, "TRANSFER_AMOUNT_MISMATCH"},
		{"转入=转出科目", map[string]any{"txnDate": "2026-09-01", "sourceCategoryId": src, "sourceAmountCents": 100,
			"legs": []map[string]any{{"categoryId": src, "amountCents": 100}}}, "TRANSFER_SAME_CATEGORY"},
		{"同科目两条转入", map[string]any{"txnDate": "2026-09-01", "sourceCategoryId": src, "sourceAmountCents": 100,
			"legs": []map[string]any{{"categoryId": dst, "amountCents": 60}, {"categoryId": dst, "amountCents": 40}}}, "TRANSFER_DUPLICATE_LEG"},
		{"转入科目为一级", map[string]any{"txnDate": "2026-09-01", "sourceCategoryId": src, "sourceAmountCents": 100,
			"legs": []map[string]any{{"categoryId": l1, "amountCents": 100}}}, "TRANSFER_LEG_NOT_LEAF"},
		{"转入科目为花费型", map[string]any{"txnDate": "2026-09-01", "sourceCategoryId": src, "sourceAmountCents": 100,
			"legs": []map[string]any{{"categoryId": spd, "amountCents": 100}}}, "TRANSFER_SPENDING_LEG"},
		{"转入科目不存在", map[string]any{"txnDate": "2026-09-01", "sourceCategoryId": src, "sourceAmountCents": 100,
			"legs": []map[string]any{{"categoryId": 88888, "amountCents": 100}}}, "CATEGORY_NOT_FOUND"},
		{"转出超余额", map[string]any{"txnDate": "2026-09-01", "sourceCategoryId": src, "sourceAmountCents": 10001,
			"legs": []map[string]any{{"categoryId": dst, "amountCents": 10001}}}, "TRANSFER_INSUFFICIENT_BALANCE"},
	}

	for _, tc := range cases {
		w := doJSON(t, r, "POST", "/api/transfers", tc.body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: 应 400，实际 %d", tc.name, w.Code)
			continue
		}
		if got := apiErr(t, w); got != tc.want {
			t.Errorf("%s: 错误码应 %s，实际 %s", tc.name, tc.want, got)
		}
	}

	// 全部拒绝后余额不变（无脏数据）
	if got := balance(t, db, src); got != 10000 {
		t.Errorf("校验拒绝后转出科目余额应仍 10000，实际 %d", got)
	}
	if got := balance(t, db, dst); got != 0 {
		t.Errorf("校验拒绝后转入科目余额应仍 0，实际 %d", got)
	}
}

// ---------- T3：作废 / 撤销（原子回滚与恢复） ----------

func TestVoidAndUnvoid(t *testing.T) {
	db, r := newEnv(t)

	l1 := seedCat(t, db, "资金", 1, nil, "active", "residual", 0, false)
	src := seedCat(t, db, "修路款", 2, l1, "active", "residual", 30000, true)
	dst := seedCat(t, db, "水利款", 2, l1, "active", "residual", 0, true)

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

	// 作废：余额应全部回滚
	w = doJSON(t, r, "PUT", "/api/transfers/"+itoa(id), map[string]any{"status": "voided"})
	if w.Code != http.StatusOK {
		t.Fatalf("作废失败: %d %s", w.Code, w.Body.String())
	}
	if got := balance(t, db, src); got != 30000 {
		t.Errorf("作废后修路款应回滚至 30000，实际 %d", got)
	}
	if got := balance(t, db, dst); got != 0 {
		t.Errorf("作废后水利款应回滚至 0，实际 %d", got)
	}

	// 撤销：余额再次变化
	w = doJSON(t, r, "PUT", "/api/transfers/"+itoa(id), map[string]any{"status": "normal"})
	if w.Code != http.StatusOK {
		t.Fatalf("撤销失败: %d %s", w.Code, w.Body.String())
	}
	if got := balance(t, db, src); got != 10000 {
		t.Errorf("撤销后修路款应 10000，实际 %d", got)
	}
	if got := balance(t, db, dst); got != 20000 {
		t.Errorf("撤销后水利款应 20000，实际 %d", got)
	}

	// 变更日志应有 void + unvoid 两条
	var logCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM change_log WHERE entity_type='transfer' AND entity_id=? AND action IN ('void','unvoid')`, id).Scan(&logCount); err != nil {
		t.Fatalf("查变更日志失败: %v", err)
	}
	if logCount != 2 {
		t.Errorf("变更日志应恰好 2 条（void+unvoid），实际 %d", logCount)
	}

	// 作废不存在 / 非法状态
	w = doJSON(t, r, "PUT", "/api/transfers/99999", map[string]any{"status": "voided"})
	if w.Code != http.StatusNotFound {
		t.Errorf("作废不存在转账应 404，实际 %d", w.Code)
	}
	w = doJSON(t, r, "PUT", "/api/transfers/"+itoa(id), map[string]any{"status": "bogus"})
	if w.Code != http.StatusBadRequest || apiErr(t, w) != "INVALID_STATUS" {
		t.Errorf("非法状态应 400 INVALID_STATUS，实际 %d %s", w.Code, w.Body.String())
	}
}

// ---------- T4：列表（含 legs、排序、科目过滤） ----------

func TestListTransfers(t *testing.T) {
	db, r := newEnv(t)

	l1 := seedCat(t, db, "资金", 1, nil, "active", "residual", 0, false)
	src := seedCat(t, db, "修路款", 2, l1, "active", "residual", 100000, true)
	dstA := seedCat(t, db, "水利款", 2, l1, "active", "residual", 0, true)
	dstB := seedCat(t, db, "公积公益金", 2, l1, "active", "residual", 0, true)

	// 一笔 1 转 N：修路款 5 万 → 水利款 3 万 + 公积公益金 2 万
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

	// 全量
	w := doJSON(t, r, "GET", "/api/transfers", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析列表失败: %v", err)
	}
	if out.Data.Total != 2 {
		t.Fatalf("应有 2 笔转账，实际 %d", out.Data.Total)
	}
	if len(out.Data.Items) != 2 || len(out.Data.Items[0].Legs) != 1 || len(out.Data.Items[1].Legs) != 2 {
		t.Errorf("列表顺序或 legs 数量不对：首笔应最新（1 leg），次笔 2 legs")
	}
	// 首笔 09-02（新日期在前）
	if out.Data.Items[0].TxnDate != "2026-09-02" {
		t.Errorf("列表应按日期倒序，首笔应 09-02，实际 %s", out.Data.Items[0].TxnDate)
	}

	// 按转入科目过滤（公积公益金应命中两笔）
	w = doJSON(t, r, "GET", "/api/transfers?categoryId="+itoa(dstB), nil)
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析过滤列表失败: %v", err)
	}
	if out.Data.Total != 2 {
		t.Errorf("按转入科目过滤应命中 2 笔，实际 %d", out.Data.Total)
	}

	// 按转出科目过滤（命中全部）+ 日期范围
	w = doJSON(t, r, "GET", "/api/transfers?from=2026-09-02&to=2026-09-30", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析日期过滤列表失败: %v", err)
	}
	if out.Data.Total != 1 {
		t.Errorf("日期过滤应命中 1 笔，实际 %d", out.Data.Total)
	}
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
