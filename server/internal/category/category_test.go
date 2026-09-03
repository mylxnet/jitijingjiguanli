package category

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/platform"
)

// newEnv 建临时库 + 迁移 + category 路由。
func newEnv(t *testing.T) (*sql.DB, *gin.Engine) {
	t.Helper()
	db, err := platform.Open(filepath.Join(t.TempDir(), "category.db"))
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

func errCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var out struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析错误响应失败: %v body=%s", err, w.Body.String())
	}
	return out.Error.Code
}

func decodeData[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var out struct {
		Data T `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析响应失败: %v body=%s", err, w.Body.String())
	}
	return out.Data
}

func createCat(t *testing.T, r *gin.Engine, name string, level int, parent *int64, bt string, opening int64, inc bool) *Category {
	t.Helper()
	w := doJSON(t, r, http.MethodPost, "/api/categories", map[string]any{
		"name": name, "level": level, "parentId": parent, "balanceType": bt,
		"openingBalanceCents": opening, "includeInReconciliation": inc,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("创建科目 %s 失败: %d %s", name, w.Code, w.Body.String())
	}
	return decodeData[*Category](t, w)
}

func int64p(v int64) *int64 { return &v }

func TestEmptyListReturnsArray(t *testing.T) {
	_, r := newEnv(t)
	w := doJSON(t, r, http.MethodGet, "/api/categories", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("空库列表应 200，实际 %d", w.Code)
	}
	var out struct {
		Data []*Category `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析响应失败: %v body=%s", err, w.Body.String())
	}
	if out.Data == nil {
		t.Errorf("空库应返回 [] 而非 null（前端直接遍历 data），实际 body=%s", w.Body.String())
	}
}

func TestCreateAndListTree(t *testing.T) {
	_, r := newEnv(t)

	l1 := createCat(t, r, "专项应付款", 1, nil, "residual", 0, false)
	_ = createCat(t, r, "修路款", 2, int64p(l1.ID), "residual", 30000, true)
	_ = createCat(t, r, "水利款", 2, int64p(l1.ID), "residual", 0, true)

	w := doJSON(t, r, http.MethodGet, "/api/categories", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("列表应 200，实际 %d", w.Code)
	}
	tree := decodeData[[]*Category](t, w)
	if len(tree) != 1 {
		t.Fatalf("一级科目应为 1 个，实际 %d", len(tree))
	}
	root := tree[0]
	if len(root.Children) != 2 {
		t.Fatalf("二级科目应为 2 个，实际 %d", len(root.Children))
	}
	// 修路款余额 = 期初 30000
	if root.Children[0].BalanceCents == nil || *root.Children[0].BalanceCents != 30000 {
		t.Errorf("修路款余额应 30000，实际 %v", root.Children[0].BalanceCents)
	}
	if root.BalanceCents == nil || *root.BalanceCents != 30000 {
		t.Errorf("一级合计应 30000，实际 %v", root.BalanceCents)
	}
}

func TestCreateValidations(t *testing.T) {
	_, r := newEnv(t)
	l1 := createCat(t, r, "管理费用", 1, nil, "residual", 0, false)
	leaf := createCat(t, r, "办公费", 2, int64p(l1.ID), "spending", 0, false)

	cases := []struct {
		name string
		body map[string]any
		code int
		want string
	}{
		{"二级缺父", map[string]any{"name": "a", "level": 2, "balanceType": "residual"}, http.StatusBadRequest, "INVALID_REQUEST"},
		{"父非一级", map[string]any{"name": "b", "level": 2, "parentId": leaf.ID, "balanceType": "residual"}, http.StatusBadRequest, "CATEGORY_PARENT_INVALID"},
		{"花费型勾稽", map[string]any{"name": "c", "level": 2, "parentId": l1.ID, "balanceType": "spending", "includeInReconciliation": true}, http.StatusBadRequest, "INVALID_REQUEST"},
		{"一级设期初", map[string]any{"name": "d", "level": 1, "balanceType": "residual", "openingBalanceCents": 100}, http.StatusBadRequest, "LEVEL1_NO_OPENING"},
		{"一级勾稽", map[string]any{"name": "e", "level": 1, "balanceType": "residual", "includeInReconciliation": true}, http.StatusBadRequest, "LEVEL1_NO_RECONCILE"},
		{"重名", map[string]any{"name": "管理费用", "level": 1, "balanceType": "residual"}, http.StatusConflict, "CATEGORY_NAME_DUP"},
	}
	for _, tc := range cases {
		w := doJSON(t, r, http.MethodPost, "/api/categories", tc.body)
		if w.Code != tc.code {
			t.Errorf("%s: 应 %d，实际 %d body=%s", tc.name, tc.code, w.Code, w.Body.String())
			continue
		}
		if got := errCode(t, w); got != tc.want {
			t.Errorf("%s: 错误码应 %s，实际 %s", tc.name, tc.want, got)
		}
	}

	// 不同一级下同名二级应允许
	l1b := createCat(t, r, "管理费用B", 1, nil, "residual", 0, false)
	w := doJSON(t, r, http.MethodPost, "/api/categories", map[string]any{
		"name": "办公费", "level": 2, "parentId": l1b.ID, "balanceType": "spending"})
	if w.Code != http.StatusOK {
		t.Errorf("异父同名应 200，实际 %d %s", w.Code, w.Body.String())
	}
}

func TestUpdateWritesChangelog(t *testing.T) {
	db, r := newEnv(t)
	l1 := createCat(t, r, "管理费用", 1, nil, "residual", 0, false)
	c := createCat(t, r, "办公费", 2, int64p(l1.ID), "spending", 0, false)

	w := doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/categories/%d", c.ID),
		map[string]any{"name": "办公费(改名)", "status": "inactive", "openingBalanceCents": 500})
	if w.Code != http.StatusOK {
		t.Fatalf("更新应 200，实际 %d %s", w.Code, w.Body.String())
	}

	var logs int
	if err := db.QueryRow(`SELECT COUNT(*) FROM change_log WHERE entity_type='category' AND entity_id=?`, c.ID).Scan(&logs); err != nil {
		t.Fatalf("查询留痕失败: %v", err)
	}
	if logs != 3 {
		t.Errorf("应产生 3 条留痕（改名/停用/改期初），实际 %d", logs)
	}

	// 一级科目改期初应 400
	w = doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/categories/%d", l1.ID),
		map[string]any{"openingBalanceCents": 1})
	if w.Code != http.StatusBadRequest {
		t.Errorf("一级改期初应 400，实际 %d", w.Code)
	}
}

func TestDeleteProtection(t *testing.T) {
	db, r := newEnv(t)
	l1 := createCat(t, r, "管理费用", 1, nil, "residual", 0, false)
	l2 := createCat(t, r, "办公费", 2, int64p(l1.ID), "spending", 0, false)
	free := createCat(t, r, "差旅费", 2, int64p(l1.ID), "spending", 0, false)

	// 一级含子 → 409
	w := doJSON(t, r, http.MethodDelete, fmt.Sprintf("/api/categories/%d", l1.ID), nil)
	if w.Code != http.StatusConflict {
		t.Errorf("一级含子删除应 409，实际 %d", w.Code)
	}

	// 给 l2 插入一笔流水 → 删除 409
	_, err := db.Exec(`INSERT INTO txn(txn_date,direction,amount_cents,category_id,note,status,created_at,updated_at)
		VALUES('2026-09-01','expense',100,?,NULL,'normal','2026-09-01','2026-09-01')`, l2.ID)
	if err != nil {
		t.Fatalf("插入流水失败: %v", err)
	}
	w = doJSON(t, r, http.MethodDelete, fmt.Sprintf("/api/categories/%d", l2.ID), nil)
	if w.Code != http.StatusConflict || errCode(t, w) != "CATEGORY_IN_USE" {
		t.Errorf("被引用删除应 409 CATEGORY_IN_USE，实际 %d %s", w.Code, errCode(t, w))
	}

	// 无引用 → 200
	w = doJSON(t, r, http.MethodDelete, fmt.Sprintf("/api/categories/%d", free.ID), nil)
	if w.Code != http.StatusOK {
		t.Errorf("未引用删除应 200，实际 %d %s", w.Code, w.Body.String())
	}
}

// TestCalcBalanceD67 用 D6/D7 验算表核对余额公式：
//
//	余粮型 residual = 期初 + 收 − 支 + 转入 − 转出
//	花费型 spending = 期初 + 支 − 收 − 转出（转入禁）
func TestCalcBalanceD67(t *testing.T) {
	db, r := newEnv(t)
	inc := true
	l1 := createCat(t, r, "专项应付款", 1, nil, "residual", 0, false)
	road := createCat(t, r, "修路款", 2, int64p(l1.ID), "residual", 30000, inc)
	water := createCat(t, r, "水利款", 2, int64p(l1.ID), "residual", 0, inc)

	// D6 步骤①：收财政拨修路款 5 万
	insertTxn(t, db, "2026-09-01", "income", 50000, road.ID)
	// D7 步骤①：修路款 → 水利款 2 万（转账）
	insertTransfer(t, db, road.ID, []leg{{water.ID, 20000}})

	repo := NewRepo(db)
	roadBal, err := repo.CalcBalance(road.ID)
	if err != nil {
		t.Fatalf("计算修路款余额失败: %v", err)
	}
	// 30000 + 50000 − 20000 = 60000
	if roadBal != 60000 {
		t.Errorf("修路款余额应 60000，实际 %d", roadBal)
	}
	waterBal, _ := repo.CalcBalance(water.ID)
	if waterBal != 20000 {
		t.Errorf("水利款余额应 20000，实际 %d", waterBal)
	}

	// 花费型：支 4 万 → 余额 4 万；结转转出 4 万 → 归零
	expL1 := createCat(t, r, "经营支出", 1, nil, "residual", 0, false)
	elec := createCat(t, r, "水电费", 2, int64p(expL1.ID), "spending", 0, false)
	insertTxn(t, db, "2026-09-02", "expense", 40000, elec.ID)
	bal, _ := repo.CalcBalance(elec.ID)
	if bal != 40000 {
		t.Errorf("花费型支出后余额应 40000，实际 %d", bal)
	}
	// 结转：水电费 → 本年收益（花费型作转出方清零，R4）
	incomeL1 := createCat(t, r, "本年收益", 1, nil, "residual", 0, false)
	profit := createCat(t, r, "本年收益科目", 2, int64p(incomeL1.ID), "residual", 0, false)
	insertTransfer(t, db, elec.ID, []leg{{profit.ID, 40000}})
	bal, _ = repo.CalcBalance(elec.ID)
	if bal != 0 {
		t.Errorf("花费型结转后余额应 0，实际 %d", bal)
	}
}

type leg struct {
	catID  int64
	amount int64
}

func insertTxn(t *testing.T, db *sql.DB, date, dir string, amount, catID int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO txn(txn_date,direction,amount_cents,category_id,note,status,created_at,updated_at)
		VALUES(?,?,?,?,NULL,'normal','2026-09-01','2026-09-01')`, date, dir, amount, catID); err != nil {
		t.Fatalf("插入流水失败: %v", err)
	}
}

func insertTransfer(t *testing.T, db *sql.DB, sourceID int64, legs []leg) {
	t.Helper()
	total := int64(0)
	for _, l := range legs {
		total += l.amount
	}
	res, err := db.Exec(`INSERT INTO transfer(txn_date, source_category_id, source_amount_cents, note, status, created_at, updated_at)
		VALUES('2026-09-01',?,?,NULL,'normal','2026-09-01','2026-09-01')`, sourceID, total)
	if err != nil {
		t.Fatalf("插入转账失败: %v", err)
	}
	tid, _ := res.LastInsertId()
	for _, l := range legs {
		if _, err := db.Exec(`INSERT INTO transfer_leg(transfer_id, category_id, amount_cents)
			VALUES(?,?,?)`, tid, l.catID, l.amount); err != nil {
			t.Fatalf("插入转账明细失败: %v", err)
		}
	}
}
