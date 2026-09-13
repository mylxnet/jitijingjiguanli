package contract

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/platform"
)

// newEnv 打开临时真实 SQLite 库并迁移，返回 db 与挂载 contract 路由的引擎（org 固定为 1）。
func newEnv(t *testing.T) (*sql.DB, *gin.Engine) {
	t.Helper()
	db, err := platform.Open(filepath.Join(t.TempDir(), "contract.db"))
	if err != nil {
		t.Fatalf("打开临时库失败: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := platform.Migrate(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO org(id, name, created_at, updated_at) VALUES(1, '测试组织', '2026-09-02', '2026-09-02')`); err != nil {
		t.Fatalf("插入测试组织失败: %v", err)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	authed := r.Group("", orgCtx())
	NewHandler(db).Register(authed)
	return db, r
}

// orgCtx 模拟已登录组织（与全库测试基建一致）。
func orgCtx() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("orgID", int64(1))
		c.Set("userID", int64(1))
		c.Next()
	}
}

func seedParty(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()
	now := platform.Now()
	res, err := db.Exec(
		`INSERT INTO party(org_id, name, created_at, updated_at) VALUES(1, ?, ?, ?)`, name, now, now)
	if err != nil {
		t.Fatalf("插入往来单位失败: %v", err)
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

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

func dataURL() string { return "data:application/octet-stream;base64,AA==" }

// 合法 date、非法格式、缺省 三种情况的落库行为。
func TestCreateContractExpiresAt(t *testing.T) {
	db, r := newEnv(t)
	pid := seedParty(t, db, "甲单位")

	// 合法到期日 → 落库
	w := doJSON(t, r, "POST", "/api/contracts", map[string]any{
		"partyId": pid, "fileName": "a.pdf", "fileData": dataURL(), "expiresAt": "2026-10-01",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("合法到期日应 200，实际 %d %s", w.Code, w.Body.String())
	}
	var exp *string
	if err := db.QueryRow(`SELECT expires_at FROM contract WHERE org_id=1`).Scan(&exp); err != nil {
		t.Fatalf("查询到期日失败: %v", err)
	}
	if exp == nil || *exp != "2026-10-01" {
		t.Errorf("到期日未正确落地: %v", exp)
	}

	// 非法格式 → 400
	w = doJSON(t, r, "POST", "/api/contracts", map[string]any{
		"partyId": pid, "fileName": "b.pdf", "fileData": dataURL(), "expiresAt": "2026/10/01",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法格式应 400，实际 %d", w.Code)
	}
	if got := apiErr(t, w); got != "INVALID_REQUEST" {
		t.Errorf("错误码应 INVALID_REQUEST，实际 %s", got)
	}

	// 缺省 → 不落到期日（保持 NULL）
	w = doJSON(t, r, "POST", "/api/contracts", map[string]any{
		"partyId": pid, "fileName": "c.pdf", "fileData": dataURL(),
	})
	if w.Code != http.StatusOK {
		t.Fatalf("缺省应 200，实际 %d %s", w.Code, w.Body.String())
	}
	var cnt int
	if err := db.QueryRow(`SELECT COUNT(*) FROM contract WHERE org_id=1 AND expires_at IS NOT NULL`).Scan(&cnt); err != nil {
		t.Fatalf("计数失败: %v", err)
	}
	if cnt != 1 {
		t.Errorf("应有 1 条带到期日的合同，实际 %d", cnt)
	}
}

// /api/contracts/expiring 只返回「已到期或 30 天内」，且排序正确、带 hasExpired/daysUntil。
func TestListExpiring(t *testing.T) {
	db, r := newEnv(t)
	pid := seedParty(t, db, "乙单位")
	repo := NewRepo(db)

	now := platform.Now()
	day := func(offset int) string {
		return now.AddDate(0, 0, offset).Format("2006-01-02")
	}

	// 三个单位，各自一条合同，覆盖 -5(已到期)、+29(即将)、+45(应过滤)
	// 新口径按「单位最新期至」判定，每个单位最多一条提醒
	pids := []int64{pid, seedParty(t, db, "丙单位"), seedParty(t, db, "丁单位")}
	for i, off := range []int{-5, 29, 45} {
		s := day(off)
		if _, err := repo.CreateContract(&Contract{
			OrgID: 1, PartyID: pids[i], FileName: "x" + itoa(int64(off+100)) + ".pdf",
			ContractTitle: "合同" + s, ExpiresAt: &s,
		}, []byte{1}); err != nil {
			t.Fatalf("建合同(%d)失败: %v", off, err)
		}
	}

	w := doJSON(t, r, "GET", "/api/contracts/expiring", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("到期清单应 200，实际 %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Data []ExpiringItem `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析到期清单失败: %v body=%s", err, w.Body.String())
	}
	if len(out.Data) != 2 {
		t.Fatalf("应返回 2 条（-5 和 +29），实际 %d: %+v", len(out.Data), out.Data)
	}
	// 升序：已到期（-5）排前
	first := out.Data[0]
	if first.HasExpired != true {
		t.Error("-5 应标记已到期")
	}
	if first.DaysUntil != -5 {
		t.Errorf("-5 daysUntil 应 -5，实际 %d", first.DaysUntil)
	}
	// 即将到期（+29）
	second := out.Data[1]
	if second.HasExpired {
		t.Error("+29 不应标记已到期")
	}
	if second.DaysUntil != 29 {
		t.Errorf("+29 daysUntil 应 29，实际 %d", second.DaysUntil)
	}
	if second.PartyName != "丙单位" || second.Type != "flow" {
		t.Errorf("单位信息错误: %+v", second)
	}
	_ = db
}

// 修改「合同期至时间」：合法更新、清空（置 null）、非法格式 400、越权 404。
func TestUpdateContractExpiry(t *testing.T) {
	db, r := newEnv(t)
	repo := NewRepo(db)
	pid := seedParty(t, db, "丙单位")

	// 建一条带到期日的合同与一条到期为空的（用于清空测试）
	orig := "2026-12-31"
	c1, err := repo.CreateContract(&Contract{OrgID: 1, PartyID: pid, FileName: "a.pdf", ExpiresAt: &orig}, []byte{1})
	if err != nil {
		t.Fatalf("建合同失败: %v", err)
	}
	nilExp := ""
	c2, err := repo.CreateContract(&Contract{OrgID: 1, PartyID: pid, FileName: "b.pdf", ExpiresAt: &nilExp}, []byte{1})
	if err != nil {
		t.Fatalf("建合同失败: %v", err)
	}

	// 1) 合法更新
	w := doJSON(t, r, "PUT", "/api/contracts/"+itoa(c1.ID)+"/expiry", map[string]any{"expiresAt": "2027-03-01"})
	if w.Code != http.StatusOK {
		t.Fatalf("合法更新应 200，实际 %d %s", w.Code, w.Body.String())
	}
	var got *string
	if err := db.QueryRow(`SELECT expires_at FROM contract WHERE id=?`, c1.ID).Scan(&got); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if got == nil || *got != "2027-03-01" {
		t.Errorf("更新未落地: %v", got)
	}

	// 2) 清空到期（expiresAt 为 null）
	w = doJSON(t, r, "PUT", "/api/contracts/"+itoa(c1.ID)+"/expiry", map[string]any{"expiresAt": nil})
	if w.Code != http.StatusOK {
		t.Fatalf("清空应 200，实际 %d %s", w.Code, w.Body.String())
	}
	if err := db.QueryRow(`SELECT expires_at FROM contract WHERE id=?`, c1.ID).Scan(&got); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if got != nil {
		t.Errorf("清空后应 NULL，实际 %v", got)
	}

	// 3) 非法格式 → 400 INVALID_REQUEST
	w = doJSON(t, r, "PUT", "/api/contracts/"+itoa(c2.ID)+"/expiry", map[string]any{"expiresAt": "2027/03/01"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法格式应 400，实际 %d", w.Code)
	}
	if got := apiErr(t, w); got != "INVALID_REQUEST" {
		t.Errorf("错误码应 INVALID_REQUEST，实际 %s", got)
	}

	// 4) 越权（他组织合同）→ 404
	db.Exec(`INSERT INTO org(id, name, created_at, updated_at) VALUES(2,'他组织','2026-09-02','2026-09-02')`)
	db.Exec(`INSERT INTO contract(org_id, party_id, file_name, expires_at, created_at, updated_at)
		VALUES(2, ?, 'other.pdf', '2027-01-01', '2026-09-02', '2026-09-02')`, pid)
	var cOther int64
	if err := db.QueryRow(`SELECT id FROM contract WHERE org_id=2`).Scan(&cOther); err != nil {
		t.Fatalf("查询他组织合同失败: %v", err)
	}
	w = doJSON(t, r, "PUT", "/api/contracts/"+itoa(cOther)+"/expiry", map[string]any{"expiresAt": "2027-05-01"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("越权应 404，实际 %d", w.Code)
	}
	if got := apiErr(t, w); got != "CONTRACT_NOT_FOUND" {
		t.Errorf("错误码应 CONTRACT_NOT_FOUND，实际 %s", got)
	}
}