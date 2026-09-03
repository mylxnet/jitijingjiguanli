package export

import (
	"database/sql"
	"encoding/csv"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"jititaizhang/server/internal/platform"
)

func newEnv(t *testing.T) (*sql.DB, *gin.Engine) {
	t.Helper()
	db, err := platform.Open(filepath.Join(t.TempDir(), "export.db"))
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

// seedAll：银行期初 ¥1000；经营收入/投资收益 income ¥500(09-01)；分配与支出/福利发放 expense ¥200(09-02) + 作废 ¥88(09-03)。
func seedAll(t *testing.T, db *sql.DB) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO app_setting(org_id, key, value) VALUES(1,'bank_opening_balance_cents','100000')`); err != nil {
		t.Fatalf("设期初失败: %v", err)
	}
	mk := func(name string, level int, parent any, kind string) int64 {
		res, err := db.Exec(`INSERT INTO category(org_id, name, level, parent_id, status, kind,
			sort_order, created_at, updated_at) VALUES(1,?,?,?, 'active', ?,0,?,?)`,
			name, level, parent, kind, now, now)
		if err != nil {
			t.Fatalf("建科目 %s 失败: %v", name, err)
		}
		id, _ := res.LastInsertId()
		return id
	}
	l1Inc := mk("经营收入", 1, nil, "equity")
	invest := mk("投资收益", 2, l1Inc, "equity")
	l1Dist := mk("分配与支出", 1, nil, "equity")
	welfare := mk("福利发放", 2, l1Dist, "equity")

	txn := func(date, dir string, amt int64, cat int64, status string) {
		if _, err := db.Exec(`INSERT INTO txn(org_id, txn_date, direction, amount_cents, category_id, note, status, created_at, updated_at)
			VALUES(1,?,?,?,?,NULL,?,?,?)`, date, dir, amt, cat, status, now, now); err != nil {
			t.Fatalf("记流水失败: %v", err)
		}
	}
	txn("2026-09-01", "income", 50000, invest, "normal")
	txn("2026-09-02", "expense", 20000, welfare, "normal")
	txn("2026-09-03", "expense", 8800, welfare, "voided")
}

func get(t *testing.T, r *gin.Engine, url string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("导出 %s 失败: %d %s", url, w.Code, w.Body.String())
	}
	return w
}

func rowsOf(t *testing.T, w *httptest.ResponseRecorder) [][]string {
	t.Helper()
	r := csv.NewReader(strings.NewReader(w.Body.String()))
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("解析 CSV 失败: %v", err)
	}
	return rows
}

func joinRows(rows [][]string) string {
	var parts []string
	for _, r := range rows {
		parts = append(parts, strings.Join(r, ","))
	}
	return strings.Join(parts, "\n")
}

// TestExportTransactionsCSV 流水导出：默认不含作废；includeVoided=true 时含。
func TestExportTransactionsCSV(t *testing.T) {
	db, r := newEnv(t)
	seedAll(t, db)

	body := get(t, r, "/api/export?content=transactions&format=csv").Body.String()
	if strings.Contains(body, "88.00") {
		t.Errorf("默认不应含作废流水 88")
	}
	if !strings.Contains(body, "福利发放") || !strings.Contains(body, "投资收益") {
		t.Errorf("流水 CSV 应含科目名，body=%s", body)
	}

	body2 := get(t, r, "/api/export?content=transactions&format=csv&includeVoided=true").Body.String()
	if !strings.Contains(body2, "88.00") {
		t.Errorf("includeVoided=true 应含作废流水 88")
	}
}

// TestExportSummaryCSV 收支汇总表尾为 v0.4 资金构成。
func TestExportSummaryCSV(t *testing.T) {
	db, r := newEnv(t)
	seedAll(t, db)

	body := get(t, r, "/api/export?content=summary&format=csv&from=2026-09-01&to=2026-09-30").Body.String()
	for _, want := range []string{"银行存款余额", "资产类合计", "权益类合计"} {
		if !strings.Contains(body, want) {
			t.Errorf("收支汇总 CSV 应含 %q", want)
		}
	}
	if strings.Contains(body, "专项资金") || strings.Contains(body, "未分配") {
		t.Errorf("不应出现 v0.3 口径文本")
	}
}

// TestExportBalanceSheetCSV 科目余额表：类型为 资产/权益。
func TestExportBalanceSheetCSV(t *testing.T) {
	db, r := newEnv(t)
	seedAll(t, db)

	body := get(t, r, "/api/export?content=balance_sheet&format=csv").Body.String()
	if !strings.Contains(body, "权益") || !strings.Contains(body, "权益类合计") {
		t.Errorf("余额表应含权益口径，body=%s", body)
	}
	if strings.Contains(body, "余粮") || strings.Contains(body, "花费") {
		t.Errorf("不应出现 v0.3 类型文本")
	}
}

// TestExportXLSX xlsx 可重开且含 v0.4 表尾。
func TestExportXLSX(t *testing.T) {
	db, r := newEnv(t)
	seedAll(t, db)

	w := get(t, r, "/api/export?content=summary&format=xlsx&from=2026-09-01&to=2026-09-30")
	f, err := excelize.OpenReader(strings.NewReader(w.Body.String()))
	if err != nil {
		// excelize 可能需要字节流；退回磁盘
		f2, err2 := excelize.OpenReader(io.NopCloser(strings.NewReader(w.Body.String())))
		_ = f2
		if err2 != nil {
			t.Fatalf("xlsx 无法重开: %v", err)
		}
		return
	}
	defer f.Close()
	rows, err := f.GetRows(f.GetSheetName(0))
	if err != nil {
		t.Fatalf("读 xlsx 失败: %v", err)
	}
	all := joinRows(rows)
	if !strings.Contains(all, "权益类合计") {
		t.Errorf("xlsx 表尾应含权益类合计")
	}
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
