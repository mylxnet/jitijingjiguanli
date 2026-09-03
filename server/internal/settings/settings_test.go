package settings

import (
	"database/sql"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/platform"
)

func newEnv(t *testing.T) *gin.Engine {
	t.Helper()
	db, err := platform.Open(filepath.Join(t.TempDir(), "settings.db"))
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
	return r
}

func get(t *testing.T, r *gin.Engine) int64 {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/settings", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("读取配置应 200，实际 %d", w.Code)
	}
	var out struct {
		Data Settings `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	return out.Data.BankOpeningBalanceCents
}

func TestSettingsRoundtrip(t *testing.T) {
	r := newEnv(t)

	// 未设置 → 0
	if v := get(t, r); v != 0 {
		t.Errorf("默认期初应为 0，实际 %d", v)
	}

	// 设置 200000（分）
	put := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}

	if w := put(`{"bankOpeningBalanceCents":200000}`); w.Code != http.StatusOK {
		t.Fatalf("设置应 200，实际 %d %s", w.Code, w.Body.String())
	}
	if v := get(t, r); v != 200000 {
		t.Errorf("期初应 200000，实际 %d", v)
	}

	// 覆盖更新
	if w := put(`{"bankOpeningBalanceCents":150000}`); w.Code != http.StatusOK {
		t.Fatalf("覆盖应 200，实际 %d", w.Code)
	}
	if v := get(t, r); v != 150000 {
		t.Errorf("期初应 150000，实际 %d", v)
	}

	// 负数拒绝
	if w := put(`{"bankOpeningBalanceCents":-1}`); w.Code != http.StatusBadRequest {
		t.Errorf("负数应 400，实际 %d", w.Code)
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

