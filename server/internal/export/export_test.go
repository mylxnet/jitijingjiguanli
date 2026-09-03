package export

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"fmt"
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

// seed：银行存款期初 ¥1000；科目 资金/修路款(期初¥300,勾稽)、资金/水利款、费用/办公费(花费型)；
// 流水：收修路款¥500(09-01)、支办公费¥200(09-02)、另记一笔作废支出¥88(09-03)。
func seedAll(t *testing.T, db *sql.DB) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO app_setting(org_id, key, value) VALUES(1,'bank_opening_balance_cents','100000')`); err != nil {
		t.Fatalf("设期初失败: %v", err)
	}
	mk := func(name string, level int, parent any, bt string, opening int64, recon bool) int64 {
		inc := 0
		if recon {
			inc = 1
		}
		res, err := db.Exec(`INSERT INTO category(org_id, name, level, parent_id, status, balance_type,
			opening_balance_cents, include_in_reconciliation, sort_order, created_at, updated_at)
			VALUES(1,?,?,?,?,?,?,?,0,?,?)`, name, level, parent, "active", bt, opening, inc, now, now)
		if err != nil {
			t.Fatalf("建科目 %s 失败: %v", name, err)
		}
		id, _ := res.LastInsertId()
		return id
	}
	l1Fund := mk("资金", 1, nil, "residual", 0, false)
	road := mk("修路款", 2, l1Fund, "residual", 30000, true)
	mk("水利款", 2, l1Fund, "residual", 0, true)
	l1Exp := mk("费用", 1, nil, "residual", 0, false)
	office := mk("办公费", 2, l1Exp, "spending", 0, false)

	txn := func(date, dir string, amt int64, cat int64, status string) {
		if _, err := db.Exec(`INSERT INTO txn(org_id, txn_date, direction, amount_cents, category_id, note, status, created_at, updated_at)
			VALUES(1,?,?,?,?,NULL,?,?,?)`, date, dir, amt, cat, status, now, now); err != nil {
			t.Fatalf("记流水失败: %v", err)
		}
	}
	txn("2026-09-01", "income", 50000, road, "normal")   // 收 500 → 修路款
	txn("2026-09-02", "expense", 20000, office, "normal") // 支 200 → 办公费
	txn("2026-09-03", "expense", 8800, office, "voided")  // 作废 88（默认不计）
}

func get(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestExportTransactionsCSV(t *testing.T) {
	db, r := newEnv(t)
	seedAll(t, db)

	w := get(t, r, "/api/export?content=transactions&format=csv")
	if w.Code != http.StatusOK {
		t.Fatalf("导出应 200，实际 %d body=%s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("Content-Type 应 text/csv，实际 %s", ct)
	}
	if !strings.HasPrefix(w.Body.String(), "\xEF\xBB\xBF") {
		t.Error("CSV 应带 UTF-8 BOM（Excel 打开中文不乱码）")
	}
	body := w.Body.String()
	// 日期倒序：09-03(作废,不含) / 09-02 支 / 09-01 收；作废默认排除
	for _, want := range []string{
		"日期,摘要,科目,收入（元）,支出（元）,状态",
		"2026-09-02,,费用 / 办公费,,200.00,正常",
		"2026-09-01,,资金 / 修路款,500.00,,正常",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("CSV 缺少行 %q\n---\n%s", want, body)
		}
	}
	if strings.Contains(body, "88.00") {
		t.Error("作废流水不应出现在默认导出中")
	}
	if !strings.Contains(body, "合计") || !strings.Contains(body, "500.00,200.00") {
		t.Errorf("应含合计行 500/200\n---\n%s", body)
	}

	// includeVoided=true 时作废行出现且带状态
	w2 := get(t, r, "/api/export?content=transactions&format=csv&includeVoided=true")
	if !strings.Contains(w2.Body.String(), "88.00") || !strings.Contains(w2.Body.String(), "已作废") {
		t.Error("includeVoided=true 时应含作废行与状态")
	}
}

func TestExportSummaryCSV(t *testing.T) {
	db, r := newEnv(t)
	seedAll(t, db)

	w := get(t, r, "/api/export?content=summary&format=csv")
	if w.Code != http.StatusOK {
		t.Fatalf("导出应 200，实际 %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{
		"科目,收入（元）,支出（元）,结余（元）",
		"资金,500.00,0.00,500.00",
		"费用,0.00,200.00,-200.00",
		"合计,500.00,200.00,300.00",
		"银行存款余额,1300.00",
		"专项资金合计,800.00",
		"未分配资金,500.00",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("CSV 缺少 %q\n---\n%s", want, body)
		}
	}
}

func TestExportBalanceSheetCSV(t *testing.T) {
	db, r := newEnv(t)
	seedAll(t, db)

	w := get(t, r, "/api/export?content=balance_sheet&format=csv")
	body := w.Body.String()

	// CSV 解析后按单元格断言（全角空格缩进字段会被引号包裹，行匹配不可靠）
	records := parseCSV(t, body)
	assertRow(t, records, []string{"资金", "300.00", "800.00", ""})
	assertRow(t, records, []string{"修路款", "300.00", "800.00", "余粮"})
	assertRow(t, records, []string{"水利款", "0.00", "0.00", "余粮"})
	assertRow(t, records, []string{"办公费", "0.00", "200.00", "花费"})
	assertRow(t, records, []string{"银行存款余额", "1300.00"})
	assertRow(t, records, []string{"专项资金合计", "800.00"})
	assertRow(t, records, []string{"未分配资金", "500.00"})
}

// parseCSV 解析导出 CSV（跳过 BOM 与标题行，首行为表头）。
func parseCSV(t *testing.T, body string) [][]string {
	t.Helper()
	body = strings.TrimPrefix(body, "\xEF\xBB\xBF")
	rd := csv.NewReader(strings.NewReader(body))
	rd.FieldsPerRecord = -1 // 标题行/数据行列数不一，不校验
	recs, err := rd.ReadAll()
	if err != nil {
		t.Fatalf("CSV 解析失败: %v\n%s", err, body)
	}
	return recs
}

// assertRow 断言存在某行：首列 trim 后等于 want[0]，其余单元格精确相等（前缀匹配数目）。
func assertRow(t *testing.T, recs [][]string, want []string) {
	t.Helper()
	for _, rec := range recs {
		if len(rec) < len(want) {
			continue
		}
		if strings.TrimSpace(rec[0]) != want[0] {
			continue
		}
		ok := true
		for i := 1; i < len(want); i++ {
			if rec[i] != want[i] {
				ok = false
				break
			}
		}
		if ok {
			return
		}
	}
	t.Errorf("未找到行 %v\n全部记录:\n%v", want, recs)
}

func TestExportXLSX(t *testing.T) {
	db, r := newEnv(t)
	seedAll(t, db)

	w := get(t, r, "/api/export?content=balance_sheet&format=xlsx")
	if w.Code != http.StatusOK {
		t.Fatalf("导出应 200，实际 %d body=%s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "spreadsheetml") {
		t.Errorf("Content-Type 应为 xlsx，实际 %s", ct)
	}
	// 用 excelize 重开校验结构与关键值（证明文件可被 Excel 类工具读取）
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("xlsx 无法重开（文件损坏）: %v", err)
	}
	defer f.Close()
	sheet := "科目余额表"
	if list := f.GetSheetList(); len(list) == 0 || list[0] != sheet {
		t.Fatalf("缺工作表 %s，现有: %v", sheet, f.GetSheetList())
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("读行失败: %v", err)
	}
	// 首行应为标题「科目余额表」，第二行表头
	if len(rows) < 3 || rows[0][0] != "科目余额表" || rows[1][0] != "科目" {
		t.Errorf("结构不符（标题/表头）: %v", rows[:2])
	}

	// 布局：第 1 行标题、第 2 行表头；此后 资金/修路款/水利款/费用/办公费 5 行 + 表尾 3 行资金构成
	type rowCell struct {
		row int
		col int // 1-based（A=1,B=2,C=3）
		val string
		num bool // 数字列：校验带 #,##0.00 显示格式
	}
	want := []rowCell{
		{3, 1, "资金", false},
		{3, 2, "300", true}, {3, 3, "800", true}, // 一级行合计（原值，格式见 num）
		{4, 1, "修路款", false}, {4, 2, "300", true}, {4, 3, "800", true},
		{5, 3, "0", true}, // 水利款当前余额
		{8, 1, "银行存款余额", false}, {8, 2, "1300", true},
		{10, 1, "未分配资金", false}, {10, 2, "500", true},
	}
	for _, c := range want {
		got, err := f.GetCellValue(sheet, cellName(c.col, c.row), excelize.Options{RawCellValue: true})
		if err != nil {
			t.Errorf("读取 %d 行 %d 列失败: %v", c.row, c.col, err)
			continue
		}
		if strings.TrimSpace(got) != c.val {
			t.Errorf("单元格(%d,%d) 值应 %q，实际 %q", c.row, c.col, c.val, got)
			continue
		}
		if c.num {
			// 校验数字格式（#,##0.00 或 0.00）——Excel 打开显示两位小数、右对齐
			styleID, err := f.GetCellStyle(sheet, cellName(c.col, c.row))
			if err != nil {
				t.Errorf("取样式 %d 行 %d 列失败: %v", c.row, c.col, err)
				continue
			}
			st, err := f.GetStyle(styleID)
			if err != nil {
				t.Errorf("读样式失败: %v", err)
				continue
			}
			hasFmt := st.NumFmt == 4 || (st.CustomNumFmt != nil && strings.Contains(*st.CustomNumFmt, "0.00"))
			right := st.Alignment != nil && st.Alignment.Horizontal == "right"
			if !hasFmt {
				t.Errorf("单元格(%d,%d) 缺金额格式 numFmt（#,##0.00），样式=%+v", c.row, c.col, st)
			}
			if !right {
				t.Errorf("单元格(%d,%d) 未右对齐，样式=%+v", c.row, c.col, st)
			}
		}
	}
}

// cellName A1 风格坐标（测试用）。
func cellName(col, row int) string {
	nm, _ := excelize.ColumnNumberToName(col)
	return nm + fmt.Sprintf("%d", row)
}

func TestExportValidation(t *testing.T) {
	_, r := newEnv(t)
	cases := []struct {
		name string
		path string
		want string
	}{
		{"坏 content", "/api/export?content=bogus&format=csv", "INVALID_CONTENT"},
		{"坏 format", "/api/export?content=summary&format=doc", "INVALID_FORMAT"},
		{"缺 content", "/api/export?format=csv", "INVALID_CONTENT"},
	}
	for _, tc := range cases {
		w := get(t, r, tc.path)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s 应 400，实际 %d", tc.name, w.Code)
			continue
		}
		if !strings.Contains(w.Body.String(), tc.want) {
			t.Errorf("%s 错误码应含 %s，实际 %s", tc.name, tc.want, w.Body.String())
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

