package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/platform"
)

// newTestEnv 建临时库 + 迁移，返回认证服务与路由（含受保护路由）。
func newTestEnv(t *testing.T) (*Service, *gin.Engine) {
	t.Helper()
	db, err := platform.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatalf("打开临时库失败: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := platform.Migrate(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	svc := NewService(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewHandler(svc).Register(r)
	protected := r.Group("/", RequireAuth(svc))
	protected.GET("/api/me", func(c *gin.Context) {
		id, _ := CurrentUserID(c)
		oid, _ := CurrentOrgID(c)
		platform.OK(c, gin.H{"userID": id, "orgID": oid})
	})
	return svc, r
}

// TestRegisterOrg 自助注册：建组织 + 管理员账号 + 预置科目；单用户部署下禁止重复注册。
func TestRegisterOrg(t *testing.T) {
	svc, _ := newTestEnv(t)

	token, expires, err := svc.RegisterOrg("甲村", "admin", "s3cret")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	if token == "" || expires.IsZero() {
		t.Fatal("注册后应签发会话（自动登录）")
	}

	userID, orgID, err := svc.Resolve(token)
	if err != nil {
		t.Fatalf("会话校验失败: %v", err)
	}
	if userID <= 0 || orgID <= 0 {
		t.Errorf("注册应返回有效用户与组织 id，userID=%d orgID=%d", userID, orgID)
	}

	// 预置科目（8 一级 + 9 二级 = 17，preset=1）
	var presetCount, l1Count, l2Count int
	if err := svc.repo.db.QueryRow(
		`SELECT COUNT(*) FROM category WHERE org_id = ? AND preset = 1`, orgID).Scan(&presetCount); err != nil {
		t.Fatalf("统计预置科目失败: %v", err)
	}
	_ = svc.repo.db.QueryRow(`SELECT COUNT(*) FROM category WHERE org_id = ? AND preset = 1 AND level = 1`, orgID).Scan(&l1Count)
	_ = svc.repo.db.QueryRow(`SELECT COUNT(*) FROM category WHERE org_id = ? AND preset = 1 AND level = 2`, orgID).Scan(&l2Count)
	if presetCount != 17 || l1Count != 8 || l2Count != 9 {
		t.Errorf("预置科目应为 17 个（8 一级 + 9 二级），实际 总数%d（一级%d 二级%d）", presetCount, l1Count, l2Count)
	}

	// 密码过短
	if _, _, err := svc.RegisterOrg("乙村", "village2", "123"); err != ErrInvalidPassword {
		t.Errorf("短密码应报 ErrInvalidPassword，实际 %v", err)
	}
	// 缺组织名
	if _, _, err := svc.RegisterOrg("  ", "village2", "123456"); err != ErrInvalidOrgName {
		t.Errorf("空组织名应报 ErrInvalidOrgName，实际 %v", err)
	}

	// 单用户部署：已有用户后禁止重复注册（用户名可用性检查因此不再触发）
	if _, _, err := svc.RegisterOrg("乙村", "village2", "s3cret2"); err != ErrRegistrationClosed {
		t.Errorf("已有用户后再次注册应报 ErrRegistrationClosed，实际 %v", err)
	}
	var users, orgs int
	_ = svc.repo.db.QueryRow(`SELECT COUNT(*) FROM user`).Scan(&users)
	_ = svc.repo.db.QueryRow(`SELECT COUNT(*) FROM org`).Scan(&orgs)
	if users != 1 || orgs != 1 {
		t.Errorf("重复注册被拒后应仍为 1 用户 1 组织，实际 users=%d orgs=%d", users, orgs)
	}
}

func TestLoginAndSession(t *testing.T) {
	svc, _ := newTestEnv(t)
	if _, _, err := svc.RegisterOrg("甲村", "admin", "s3cret"); err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	if _, _, err := svc.Login("admin", "wrong"); err != ErrInvalidCredentials {
		t.Fatalf("错误密码应返回 ErrInvalidCredentials，实际 %v", err)
	}
	if _, _, err := svc.Login("nobody", "s3cret"); err != ErrInvalidCredentials {
		t.Fatalf("不存在账号应返回同一错误（避免账号枚举），实际 %v", err)
	}

	token, expires, err := svc.Login("admin", "s3cret")
	if err != nil {
		t.Fatalf("正确口令登录失败: %v", err)
	}
	if token == "" || expires.IsZero() {
		t.Fatal("登录应返回非空 token 与过期时间")
	}

	userID, orgID, err := svc.Resolve(token)
	if err != nil {
		t.Fatalf("会话校验失败: %v", err)
	}
	if userID <= 0 || orgID <= 0 {
		t.Errorf("Resolve 应返回用户与组织 id，userID=%d orgID=%d", userID, orgID)
	}

	if err := svc.Logout(token); err != nil {
		t.Fatalf("登出失败: %v", err)
	}
	if _, _, err := svc.Resolve(token); err != ErrUnauthorized {
		t.Errorf("登出后会话应失效，实际 %v", err)
	}
}

func TestRequireAuth(t *testing.T) {
	svc, r := newTestEnv(t)
	if _, _, err := svc.RegisterOrg("甲村", "admin", "s3cret"); err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	// 1) 无 Cookie → 401
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("无 Cookie 应 401，实际 %d", w.Code)
	}
	assertErrCode(t, w.Body.Bytes(), "UNAUTHORIZED")

	// 2) 伪造 token → 401
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: "forged"})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("伪造 token 应 401，实际 %d", w.Code)
	}

	// 3) 登录后携带 Cookie → 200 且能取到 userID 与 orgID
	w = httptest.NewRecorder()
	body := strings.NewReader(`{"username":"admin","password":"s3cret"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("登录应 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	var session *http.Cookie
	for _, c := range cookies {
		if c.Name == CookieName {
			session = c
		}
	}
	if session == nil {
		t.Fatal("登录响应应包含会话 Cookie")
	}
	if !session.HttpOnly {
		t.Error("会话 Cookie 必须为 HttpOnly")
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(session)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("携带有效会话应 200，实际 %d", w.Code)
	}
	var out struct {
		Data struct {
			UserID int `json:"userID"`
			OrgID  int `json:"orgID"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if out.Data.UserID <= 0 || out.Data.OrgID <= 0 {
		t.Errorf("受保护路由应能取到用户与组织 id，userID=%d orgID=%d", out.Data.UserID, out.Data.OrgID)
	}
}

func TestLoginBadRequest(t *testing.T) {
	_, r := newTestEnv(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		strings.NewReader(`{"username":"admin"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺少密码应 400，实际 %d", w.Code)
	}
	assertErrCode(t, w.Body.Bytes(), "INVALID_REQUEST")
}

// TestRegisterBadRequest 注册参数缺失 → 400。
func TestRegisterBadRequest(t *testing.T) {
	_, r := newTestEnv(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register",
		strings.NewReader(`{"orgName":"甲村"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("注册缺账号应 400，实际 %d", w.Code)
	}
}

func assertErrCode(t *testing.T, body []byte, want string) {
	t.Helper()
	var out struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("解析错误响应失败: %v，body=%s", err, body)
	}
	if out.Error.Code != want {
		t.Errorf("错误码应为 %s，实际 %s", want, out.Error.Code)
	}
}
