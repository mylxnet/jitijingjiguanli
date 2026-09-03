package category

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

	"jititaizhang/server/internal/platform"
)

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
	seedTestOrg(t, db)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	authed := r.Group("", orgCtx())
	NewHandler(db).Register(authed)
	return db, r
}

// seedCat 直接插科目（kind: equity/asset；level=1 时 parent=nil）。
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
		t.Fatalf("插科目 %s 失败: %v", name, err)
	}
	id, _ := res.LastInsertId()
	return id
}

func seedTxn(t *testing.T, db *sql.DB, date, dir string, amt, catID int64) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO txn(org_id, txn_date, direction, amount_cents, category_id, note, status, created_at, updated_at)
		VALUES(1,?,?,?,?,NULL,'normal',?,?)`, date, dir, amt, catID, now, now); err != nil {
		t.Fatalf("插流水失败: %v", err)
	}
}

func seedMove(t *testing.T, db *sql.DB, kind string, assetID, amt int64) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO fund_move(org_id, move_date, kind, asset_category_id, amount_cents, note, status, created_at, updated_at)
		VALUES(1,'2026-09-01',?,?,?,NULL,'normal',?,?)`, kind, assetID, amt, now, now); err != nil {
		t.Fatalf("插资金划转失败: %v", err)
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

// TestTreeAndBalances 树与余额：权益=收支相抵；资产=资金划转；一级=子项和。
func TestTreeAndBalances(t *testing.T) {
	db, r := newEnv(t)
	l1Inc := seedCat(t, db, "经营收入", 1, nil, "active", "equity")
	income := seedCat(t, db, "投资收益", 2, l1Inc, "active", "equity")
	l1Dist := seedCat(t, db, "分配与支出", 1, nil, "active", "equity")
	welfare := seedCat(t, db, "福利发放", 2, l1Dist, "active", "equity")
	l1Inv := seedCat(t, db, "对外投资", 1, nil, "active", "equity")
	invest := seedCat(t, db, "项目A", 2, l1Inv, "active", "asset")

	seedTxn(t, db, "2026-09-01", "income", 50000, income)
	seedTxn(t, db, "2026-09-02", "expense", 20000, welfare)
	seedMove(t, db, "invest", invest, 40000)

	w := doJSON(t, r, "GET", "/api/categories", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("列表失败: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Data []*Category `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	byL1 := map[string]*Category{}
	for _, root := range out.Data {
		byL1[root.Name] = root
	}
	if byL1["经营收入"].Children[0].BalanceCents == nil || *byL1["经营收入"].Children[0].BalanceCents != 50000 {
		t.Errorf("投资收益余额应 50000，实际 %v", byL1["经营收入"].Children[0].BalanceCents)
	}
	if byL1["分配与支出"].Children[0].BalanceCents == nil || *byL1["分配与支出"].Children[0].BalanceCents != -20000 {
		t.Errorf("福利发放余额应 -20000，实际 %v", byL1["分配与支出"].Children[0].BalanceCents)
	}
	if byL1["对外投资"].Children[0].BalanceCents == nil || *byL1["对外投资"].Children[0].BalanceCents != 40000 {
		t.Errorf("项目A资产余额应 40000，实际 %v", byL1["对外投资"].Children[0].BalanceCents)
	}
}

// TestCreateValidations 创建校验（v0.4）。
func TestCreateValidations(t *testing.T) {
	db, r := newEnv(t)
	l1 := seedCat(t, db, "对外投资", 1, nil, "active", "equity")

	// 一级建资产 → 拒绝
	w := doJSON(t, r, "POST", "/api/categories", map[string]any{"name": "A", "level": 1, "kind": "asset"})
	if w.Code != http.StatusBadRequest || apiErr(t, w) != "ASSET_LEVEL" {
		t.Errorf("一级资产应 400 ASSET_LEVEL，实际 %d %s", w.Code, w.Body.String())
	}
	// 二级缺父
	w = doJSON(t, r, "POST", "/api/categories", map[string]any{"name": "B", "level": 2, "kind": "equity"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("二级缺父应 400，实际 %d", w.Code)
	}
	// 重名（同父）
	doJSON(t, r, "POST", "/api/categories", map[string]any{"name": "项目A", "level": 2, "parentId": l1, "kind": "asset"})
	w = doJSON(t, r, "POST", "/api/categories", map[string]any{"name": "项目A", "level": 2, "parentId": l1, "kind": "asset"})
	if w.Code != http.StatusConflict || apiErr(t, w) != "CATEGORY_NAME_DUP" {
		t.Errorf("重名应 409 CATEGORY_NAME_DUP，实际 %d %s", w.Code, w.Body.String())
	}
	// 父不是一级
	l2 := seedCat(t, db, "子级", 2, l1, "active", "equity")
	w = doJSON(t, r, "POST", "/api/categories", map[string]any{"name": "C", "level": 2, "parentId": l2, "kind": "equity"})
	if w.Code != http.StatusBadRequest || apiErr(t, w) != "CATEGORY_PARENT_INVALID" {
		t.Errorf("父不是一级应 400 CATEGORY_PARENT_INVALID，实际 %d %s", w.Code, w.Body.String())
	}
}

// TestUpdateChangelog 改名/停用留痕。
func TestUpdateChangelog(t *testing.T) {
	db, r := newEnv(t)
	l1 := seedCat(t, db, "对外投资", 1, nil, "active", "equity")
	id := seedCat(t, db, "项目A", 2, l1, "active", "asset")

	w := doJSON(t, r, "PUT", "/api/categories/"+itoa(id), map[string]any{"name": "项目A2"})
	if w.Code != http.StatusOK {
		t.Fatalf("改名失败: %d %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, "PUT", "/api/categories/"+itoa(id), map[string]any{"status": "inactive"})
	if w.Code != http.StatusOK {
		t.Fatalf("停用失败: %d %s", w.Code, w.Body.String())
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM change_log WHERE entity_type='category' AND entity_id=?`, id).Scan(&n); err != nil {
		t.Fatalf("查留痕失败: %v", err)
	}
	if n != 2 {
		t.Errorf("留痕应 2 条（改名+停用），实际 %d", n)
	}
}

// TestDeleteGuards 删除保护。
func TestDeleteGuards(t *testing.T) {
	db, r := newEnv(t)
	l1 := seedCat(t, db, "对外投资", 1, nil, "active", "equity")
	invest := seedCat(t, db, "项目A", 2, l1, "active", "asset")
	seedMove(t, db, "invest", invest, 40000)

	// 有资金划转 → 拒绝
	w := doJSON(t, r, "DELETE", "/api/categories/"+itoa(invest), nil)
	if w.Code != http.StatusConflict || apiErr(t, w) != "CATEGORY_IN_USE" {
		t.Errorf("资产被引用删除应 409，实际 %d %s", w.Code, w.Body.String())
	}
	// 一级仍有子 → 拒绝
	w = doJSON(t, r, "DELETE", "/api/categories/"+itoa(l1), nil)
	if w.Code != http.StatusConflict || apiErr(t, w) != "CATEGORY_HAS_CHILD" {
		t.Errorf("一级有子删除应 409，实际 %d %s", w.Code, w.Body.String())
	}

	// 空权益科目可删
	empty := seedCat(t, db, "空科目", 2, l1, "active", "equity")
	w = doJSON(t, r, "DELETE", "/api/categories/"+itoa(empty), nil)
	if w.Code != http.StatusOK {
		t.Errorf("空科目删除应 200，实际 %d %s", w.Code, w.Body.String())
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
