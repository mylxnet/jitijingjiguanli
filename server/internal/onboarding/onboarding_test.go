package onboarding

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"jititaizhang/server/internal/platform"
)

// newEnv 打开临时库 + 迁移 + 种子组织 + 预置容器 L1，返回 router。
func newEnv(t *testing.T) (*sql.DB, *gin.Engine) {
	t.Helper()
	db, err := platform.Open(t.TempDir() + "/onboard.db")
	if err != nil {
		t.Fatalf("打开临时库失败: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := platform.Migrate(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	_, _ = db.Exec(`INSERT INTO org(id, name, created_at, updated_at) VALUES(1, '测试组织', '2026-09-02', '2026-09-02')`)
	// 预置容器一级科目（flow: 土地流转费收入/流转管理费；invest: 长期投资；reinvest: 再投资）
	for _, name := range []string{"土地流转费收入", "流转管理费", "长期投资", "再投资"} {
		if _, err := db.Exec(
			`INSERT INTO category(org_id, name, level, parent_id, status, kind, sort_order, created_at, updated_at)
			 VALUES(1, ?, 1, NULL, 'active', 'equity', 0, '2026-09-02', '2026-09-02')`, name); err != nil {
			t.Fatalf("插入容器科目失败 %s: %v", name, err)
		}
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	authed := r.Group("", func(c *gin.Context) { c.Set("orgID", int64(1)); c.Set("userID", int64(1)); c.Next() })
	NewHandler(db).Register(authed)
	return db, r
}

// buildTestXlsx 基于真实模板填充示例数据后返回文件字节。
type testRow struct{ cells []string }

func buildTestXlsx(t *testing.T, flow, invest, reinvest []testRow) []byte {
	t.Helper()
	buf, err := buildTemplate()
	if err != nil {
		t.Fatalf("生成模板失败: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("打开模板失败: %v", err)
	}
	defer f.Close()
	fill := func(sheet string, rows []testRow) {
		for i, row := range rows {
			for j, v := range row.cells {
				cell, _ := excelize.CoordinatesToCellName(j+1, i+2)
				_ = f.SetCellValue(sheet, cell, v)
			}
		}
	}
	fill("流转企业", flow)
	fill("投资公司", invest)
	fill("再投资", reinvest)
	var out bytes.Buffer
	if _, err := f.WriteTo(&out); err != nil {
		t.Fatalf("写出 xlsx 失败: %v", err)
	}
	return out.Bytes()
}

func doImport(t *testing.T, r *gin.Engine, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "基础数据导入模板.xlsx")
	_, _ = fw.Write(data)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/onboarding/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeResult(t *testing.T, w *httptest.ResponseRecorder) *ImportResult {
	t.Helper()
	var out struct {
		Data *ImportResult `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析结果失败: %v body=%s", err, w.Body.String())
	}
	return out.Data
}

func l2ID(t *testing.T, db *sql.DB, l1, l2 string) int64 {
	var l1id, l2id int64
	if err := db.QueryRow(`SELECT id FROM category WHERE org_id=1 AND name=? AND level=1`, l1).Scan(&l1id); err != nil {
		return 0
	}
	if err := db.QueryRow(`SELECT id FROM category WHERE org_id=1 AND name=? AND parent_id=?`, l2, l1id).Scan(&l2id); err != nil {
		return 0
	}
	return l2id
}

// TestImportCreatesPartiesAndAccounts 三 sheet：流转企业+投资公司正常建，再投资留空。
func TestImportCreatesPartiesAndAccounts(t *testing.T) {
	db, r := newEnv(t)

	data := buildTestXlsx(t,
		[]testRow{{[]string{"蓝天农业合作社", "13800001234", "120", "800", "50"}}, {[]string{"绿源家庭农场", "", "45"}}},
		[]testRow{{[]string{"X科技有限公司", "", "500000", "6"}}},
		nil, // 再投资留空 → 不建账
	)
	w := doImport(t, r, data)
	if w.Code != 200 {
		t.Fatalf("导入返回 %d: %s", w.Code, w.Body.String())
	}
	res := decodeResult(t, w)
	if res.Created != 3 || res.Failed != 0 {
		t.Fatalf("期望成功3失败0，实际 成功%d 失败%d", res.Created, res.Failed)
	}

	// 单位与类型
	var flowC, invC int
	_ = db.QueryRow(`SELECT COUNT(*) FROM party WHERE org_id=1 AND type='flow'`).Scan(&flowC)
	_ = db.QueryRow(`SELECT COUNT(*) FROM party WHERE org_id=1 AND type='invest'`).Scan(&invC)
	if flowC != 2 || invC != 1 {
		t.Fatalf("期望 flow2 invest1，实际 flow%d invest%d", flowC, invC)
	}

	// 联动建同名科目
	if l2ID(t, db, "土地流转费收入", "蓝天农业合作社") == 0 {
		t.Fatalf("流转企业未自动创建同名二级科目")
	}
	if l2ID(t, db, "长期投资", "X科技有限公司") == 0 {
		t.Fatalf("投资公司未自动创建同名二级科目")
	}

	// 投资本金 → 同名科目期初（500000 元 = 50000000 分）
	var opening int64
	if err := db.QueryRow(`SELECT opening_balance_cents FROM category WHERE id=?`, l2ID(t, db, "长期投资", "X科技有限公司")).Scan(&opening); err != nil {
		t.Fatalf("读取期初失败: %v", err)
	}
	if opening != 50000000 {
		t.Fatalf("投资本金期初应为 50000000，实际 %d", opening)
	}
	// 再投资留空：不应有再投资类单位
	var rc int
	_ = db.QueryRow(`SELECT COUNT(*) FROM party WHERE org_id=1 AND type='reinvest'`).Scan(&rc)
	if rc != 0 {
		t.Fatalf("再投资留空却创建了 %d 个单位", rc)
	}
}

// TestImportDuplicateFails 同表内重复名称 → 失败行，且正确计数；跨类型允许同名。
func TestImportDuplicateFails(t *testing.T) {
	_, r := newEnv(t)
	data := buildTestXlsx(t,
		[]testRow{{[]string{"蓝天农业合作社", "", "120"}}, {[]string{"蓝天农业合作社", "", "80"}}, {[]string{"绿源家庭农场", "", "45"}}},
		[]testRow{{[]string{"蓝天农业合作社", "", "100000"}}}, // 跨类型同名允许
		nil,
	)
	w := doImport(t, r, data)
	res := decodeResult(t, w)
	// 流转企业：2 行同名 → 第2行失败；投资公司与流转企业同名 → 允许。
	if res.Created != 3 || res.Failed != 1 {
		t.Fatalf("期望成功3失败1，实际 成功%d 失败%d errors=%v", res.Created, res.Failed, res.Errors)
	}
}

// TestImportInvalidFile 非 xlsx → 400。
func TestImportInvalidFile(t *testing.T) {
	_, r := newEnv(t)
	if w := doImport(t, r, []byte("not an excel")); w.Code != 400 {
		t.Fatalf("非法文件应返回 400，实际 %d", w.Code)
	}
}

// TestCompleteOnboarding 完成引导接口：把 org.onboarded 置 1，且重复调用幂等。
func TestCompleteOnboarding(t *testing.T) {
	db, r := newEnv(t)

	var n int
	if err := db.QueryRow(`SELECT onboarded FROM org WHERE id=1`).Scan(&n); err != nil {
		t.Fatalf("读取 onboarded 失败: %v", err)
	}
	if n != 0 {
		t.Fatalf("新组织 onboarded 应为 0，实际 %d", n)
	}

	for i := 0; i < 2; i++ { // 幂等：重复调用仍返回 200 且保持 1
		req := httptest.NewRequest(http.MethodPost, "/api/onboarding/complete", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("第 %d 次调用返回 %d: %s", i+1, w.Code, w.Body.String())
		}
		if err := db.QueryRow(`SELECT onboarded FROM org WHERE id=1`).Scan(&n); err != nil {
			t.Fatalf("读取 onboarded 失败: %v", err)
		}
		if n != 1 {
			t.Fatalf("完成后 onboarded 应为 1，实际 %d", n)
		}
	}
}