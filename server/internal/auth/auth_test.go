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

// newTestEnv 建临时库 + 迁移 + 初始账号，并搭好 gin 路由（含受保护路由）。
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
	if err := svc.EnsureInitialUser("admin", "s3cret"); err != nil {
		t.Fatalf("创建初始账号失败: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewHandler(svc).Register(r)
	protected := r.Group("/", RequireAuth(svc))
	protected.GET("/api/me", func(c *gin.Context) {
		id, _ := CurrentUserID(c)
		platform.OK(c, gin.H{"userID": id})
	})
	return svc, r
}

func TestEnsureInitialUserIdempotent(t *testing.T) {
	svc, _ := newTestEnv(t)
	// 再次调用不应报错、不应新增用户
	if err := svc.EnsureInitialUser("another", "whatever"); err != nil {
		t.Fatalf("重复初始化应无副作用: %v", err)
	}
	rows, err := svc.repo.db.Query(`SELECT username FROM user`)
	if err != nil {
		t.Fatalf("查询用户失败: %v", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
	}
	if count != 1 {
		t.Errorf("用户数应为 1，实际 %d", count)
	}
}

func TestLoginAndSession(t *testing.T) {
	svc, _ := newTestEnv(t)

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

	userID, err := svc.Resolve(token)
	if err != nil {
		t.Fatalf("会话校验失败: %v", err)
	}
	if userID <= 0 {
		t.Errorf("用户 id 应为正数，实际 %d", userID)
	}

	if err := svc.Logout(token); err != nil {
		t.Fatalf("登出失败: %v", err)
	}
	if _, err := svc.Resolve(token); err != ErrUnauthorized {
		t.Errorf("登出后会话应失效，实际 %v", err)
	}
}

func TestRequireAuth(t *testing.T) {
	_, r := newTestEnv(t)

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

	// 3) 登录后携带 Cookie → 200 且能取到 userID
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
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if out.Data.UserID <= 0 {
		t.Errorf("受保护路由应能取到用户 id，实际 %d", out.Data.UserID)
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
