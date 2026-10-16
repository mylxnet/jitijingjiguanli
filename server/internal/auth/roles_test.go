// roles_test.go —— 多用户与角色权限的接口级测试（见 docs/27 方案 §10 测试计划）。
//
// 覆盖：角色枚举与登录返回 role、RequireWrite/RequireRole 权限矩阵、
// 账号管理越权约束、最后一个 owner 保护、停用即失效、忘记密码语义收紧。

package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/platform"
)

// newRoleEnv 建临时库 + 迁移，并按权限矩阵挂载 read/biz/admin/owner 四组路由。
func newRoleEnv(t *testing.T) (*Service, *gin.Engine) {
	t.Helper()
	db, err := platform.Open(filepath.Join(t.TempDir(), "roles.db"))
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

	ah := NewHandler(svc)
	ah.Register(r) // public：登录/注册/登出/忘记密码

	// read 组：仅需登录（viewer 也可访问的自服务/只读端点）。
	read := r.Group("", RequireAuth(svc))
	ah.RegisterAuthed(read)
	read.GET("/api/probe-read", okProbe)

	// biz 组：登录 + 非只读账号（GET 放行 viewer，写操作拦 viewer）。
	biz := r.Group("", RequireAuth(svc), RequireWrite())
	biz.GET("/api/probe-write", okProbe)
	biz.POST("/api/probe-write", okProbe)

	// admin 组：owner/admin 可访问。
	admin := r.Group("", RequireAuth(svc), RequireRole(RoleAdmin, RoleOwner))
	ah.RegisterUsers(admin)
	admin.GET("/api/me-role", func(c *gin.Context) {
		role, _ := CurrentRole(c)
		platform.OK(c, gin.H{"role": role})
	})

	// owner 组：仅 owner。
	ownerGrp := r.Group("", RequireAuth(svc), RequireRole(RoleOwner))
	ownerGrp.POST("/api/probe-owner", okProbe)

	return svc, r
}

func okProbe(c *gin.Context) { platform.OK(c, gin.H{"ok": true}) }

// req 发起一次带可选会话 Cookie 的请求。
func req(t *testing.T, r *gin.Engine, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	h := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		h.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		h.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, h)
	return w
}

// decodeData 解析 platform.OK 的 {data: ...} 包裹体。
func decodeData(t *testing.T, body []byte, v any) {
	t.Helper()
	var wrapper struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		t.Fatalf("解析响应失败: %v body=%s", err, body)
	}
	if err := json.Unmarshal(wrapper.Data, v); err != nil {
		t.Fatalf("解析 data 失败: %v body=%s", err, body)
	}
}

// errCode 解析 platform.Fail 的 {error:{code:...}} 包裹体。
func errCode(t *testing.T, body []byte) string {
	t.Helper()
	var out struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("解析错误体失败: %v body=%s", err, body)
	}
	return out.Error.Code
}

// registerOwner 注册首个组织（首人 = owner）并返回其身份。
func registerOwner(t *testing.T, svc *Service, org, username, password string) *Identity {
	t.Helper()
	token, _, err := svc.RegisterOrg(org, username, password)
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	id, err := svc.Resolve(token)
	if err != nil {
		t.Fatalf("会话校验失败: %v", err)
	}
	if id.Role != RoleOwner {
		t.Fatalf("首个注册账号应为 owner，实际 %s", id.Role)
	}
	return id
}

// loginAs 通过接口登录，返回会话 Cookie 与响应中的角色。
func loginAs(t *testing.T, r *gin.Engine, username, password string) (*http.Cookie, string) {
	t.Helper()
	w := req(t, r, http.MethodPost, "/api/auth/login",
		`{"username":"`+username+`","password":"`+password+`"}`, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("登录 %s 应 200，实际 %d body=%s", username, w.Code, w.Body.String())
	}
	var cookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == CookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatalf("登录 %s 未返回会话 Cookie", username)
	}
	var data struct {
		User struct {
			Role string `json:"role"`
		} `json:"user"`
	}
	decodeData(t, w.Body.Bytes(), &data)
	return cookie, data.User.Role
}

// createUserViaAPI 以给定会话调用 POST /api/users 建号。
func createUserViaAPI(t *testing.T, r *gin.Engine, cookie *http.Cookie, username, role string) *httptest.ResponseRecorder {
	t.Helper()
	return req(t, r, http.MethodPost, "/api/users",
		`{"username":"`+username+`","password":"pass123","role":"`+role+`"}`, cookie)
}

// seedUserDirect 直插账号（仅测试用：绕过 service 的建号角色限制，构造多 owner 等场景）。
func seedUserDirect(t *testing.T, svc *Service, orgID int64, username, role, status string) *User {
	t.Helper()
	u, err := svc.repo.CreateUser(orgID, username, "x", role)
	if err != nil {
		t.Fatalf("直插账号 %s 失败: %v", username, err)
	}
	if status != StatusActive {
		if err := svc.repo.UpdateRoleStatus(u.ID, role, status); err != nil {
			t.Fatalf("直改账号状态失败: %v", err)
		}
		u.Status = status
	}
	return u
}

func listUsers(t *testing.T, svc *Service, orgID int64) []*User {
	t.Helper()
	users, err := svc.ListUsers(orgID)
	if err != nil {
		t.Fatalf("列账号失败: %v", err)
	}
	return users
}

func userIDByName(t *testing.T, users []*User, name string) int64 {
	t.Helper()
	for _, u := range users {
		if u.Username == name {
			return u.ID
		}
	}
	t.Fatalf("账号 %s 不存在", name)
	return 0
}

func itoa64(v int64) string { return strconv.FormatInt(v, 10) }

// TestRoleMatrixReadWrite 权限矩阵：viewer 只读、admin 可写可管理一般用户、owner 专属端点仅 owner。
func TestRoleMatrixReadWrite(t *testing.T) {
	svc, r := newRoleEnv(t)
	registerOwner(t, svc, "甲村", "admin", "s3cret")
	ownerCookie, role := loginAs(t, r, "admin", "s3cret")
	if role != RoleOwner {
		t.Fatalf("首个账号登录应返回 owner，实际 %s", role)
	}

	if w := createUserViaAPI(t, r, ownerCookie, "boss", RoleAdmin); w.Code != http.StatusOK {
		t.Fatalf("owner 建 admin 应 200，实际 %d %s", w.Code, w.Body.String())
	}
	if w := createUserViaAPI(t, r, ownerCookie, "guest", RoleViewer); w.Code != http.StatusOK {
		t.Fatalf("owner 建 viewer 应 200，实际 %d %s", w.Code, w.Body.String())
	}
	adminCookie, adminRole := loginAs(t, r, "boss", "pass123")
	viewerCookie, viewerRole := loginAs(t, r, "guest", "pass123")
	if adminRole != RoleAdmin || viewerRole != RoleViewer {
		t.Fatalf("登录角色不符：admin=%s viewer=%s", adminRole, viewerRole)
	}

	// 未登录 → 401
	if w := req(t, r, http.MethodGet, "/api/probe-write", "", nil); w.Code != http.StatusUnauthorized {
		t.Errorf("未登录访问业务端点应 401，实际 %d", w.Code)
	}

	// viewer：GET 放行
	if w := req(t, r, http.MethodGet, "/api/probe-write", "", viewerCookie); w.Code != http.StatusOK {
		t.Errorf("viewer 读业务端点应 200，实际 %d %s", w.Code, w.Body.String())
	}
	// viewer：POST 拦截
	if w := req(t, r, http.MethodPost, "/api/probe-write", `{}`, viewerCookie); w.Code != http.StatusForbidden || errCode(t, w.Body.Bytes()) != "FORBIDDEN" {
		t.Errorf("viewer 写业务端点应 403 FORBIDDEN，实际 %d %s", w.Code, w.Body.String())
	}
	// viewer：管理页拦截
	if w := req(t, r, http.MethodGet, "/api/me-role", "", viewerCookie); w.Code != http.StatusForbidden {
		t.Errorf("viewer 进管理页应 403，实际 %d", w.Code)
	}
	// viewer：owner 专属拦截
	if w := req(t, r, http.MethodPost, "/api/probe-owner", "", viewerCookie); w.Code != http.StatusForbidden {
		t.Errorf("viewer 访问 owner 端点应 403，实际 %d", w.Code)
	}
	// 白名单：viewer 可改自己的密码（read 组）
	if w := req(t, r, http.MethodPut, "/api/auth/password",
		`{"oldPassword":"pass123","newPassword":"pass124"}`, viewerCookie); w.Code != http.StatusOK {
		t.Errorf("viewer 改自己密码应 200，实际 %d %s", w.Code, w.Body.String())
	}

	// admin：写放行、管理页放行、owner 专属拦截
	if w := req(t, r, http.MethodPost, "/api/probe-write", `{}`, adminCookie); w.Code != http.StatusOK {
		t.Errorf("admin 写业务端点应 200，实际 %d %s", w.Code, w.Body.String())
	}
	if w := req(t, r, http.MethodGet, "/api/me-role", "", adminCookie); w.Code != http.StatusOK {
		t.Errorf("admin 进管理页应 200，实际 %d", w.Code)
	}
	if w := req(t, r, http.MethodPost, "/api/probe-owner", "", adminCookie); w.Code != http.StatusForbidden {
		t.Errorf("admin 访问 owner 端点应 403，实际 %d", w.Code)
	}

	// owner：owner 专属端点放行，/api/me-role 回显 owner
	if w := req(t, r, http.MethodPost, "/api/probe-owner", "", ownerCookie); w.Code != http.StatusOK {
		t.Errorf("owner 访问 owner 端点应 200，实际 %d", w.Code)
	}
	w := req(t, r, http.MethodGet, "/api/me-role", "", ownerCookie)
	var d struct {
		Role string `json:"role"`
	}
	decodeData(t, w.Body.Bytes(), &d)
	if d.Role != RoleOwner {
		t.Errorf("/api/me-role 应回显 owner，实际 %s", d.Role)
	}
}

// TestUserManagementRules 账号管理越权约束：admin 只能管 viewer；不能对自己操作。
func TestUserManagementRules(t *testing.T) {
	svc, r := newRoleEnv(t)
	owner := registerOwner(t, svc, "甲村", "admin", "s3cret")
	ownerCookie, _ := loginAs(t, r, "admin", "s3cret")

	if w := createUserViaAPI(t, r, ownerCookie, "boss", RoleAdmin); w.Code != http.StatusOK {
		t.Fatalf("owner 建 admin 应 200，实际 %d %s", w.Code, w.Body.String())
	}
	if w := createUserViaAPI(t, r, ownerCookie, "guest", RoleViewer); w.Code != http.StatusOK {
		t.Fatalf("owner 建 viewer 应 200，实际 %d %s", w.Code, w.Body.String())
	}
	adminCookie, _ := loginAs(t, r, "boss", "pass123")
	viewerCookie, _ := loginAs(t, r, "guest", "pass123")

	users := listUsers(t, svc, owner.OrgID)
	bossID := userIDByName(t, users, "boss")
	guestID := userIDByName(t, users, "guest")

	// admin 不能建 admin → 403；不能建 owner → 400（角色不合法）
	if w := createUserViaAPI(t, r, adminCookie, "boss2", RoleAdmin); w.Code != http.StatusForbidden || errCode(t, w.Body.Bytes()) != "FORBIDDEN" {
		t.Errorf("admin 建 admin 应 403 FORBIDDEN，实际 %d %s", w.Code, w.Body.String())
	}
	if w := createUserViaAPI(t, r, adminCookie, "owner2", RoleOwner); w.Code != http.StatusBadRequest || errCode(t, w.Body.Bytes()) != "INVALID_ROLE" {
		t.Errorf("admin 建 owner 应 400 INVALID_ROLE，实际 %d %s", w.Code, w.Body.String())
	}
	// admin 建 viewer → 200
	if w := createUserViaAPI(t, r, adminCookie, "guest2", RoleViewer); w.Code != http.StatusOK {
		t.Errorf("admin 建 viewer 应 200，实际 %d %s", w.Code, w.Body.String())
	}

	// admin 不能动 owner / admin
	if w := req(t, r, http.MethodDelete, "/api/users/"+itoa64(owner.UserID), "", adminCookie); w.Code != http.StatusForbidden || errCode(t, w.Body.Bytes()) != "FORBIDDEN" {
		t.Errorf("admin 停用 owner 应 403，实际 %d %s", w.Code, w.Body.String())
	}
	if w := req(t, r, http.MethodPatch, "/api/users/"+itoa64(bossID), `{"status":"disabled"}`, adminCookie); w.Code != http.StatusForbidden {
		t.Errorf("admin 改 admin 应 403，实际 %d %s", w.Code, w.Body.String())
	}

	// 不能对自己操作（owner 停用自己）
	if w := req(t, r, http.MethodDelete, "/api/users/"+itoa64(owner.UserID), "", ownerCookie); w.Code != http.StatusBadRequest || errCode(t, w.Body.Bytes()) != "SELF_OPERATION" {
		t.Errorf("owner 对自己停用应 400 SELF_OPERATION，实际 %d %s", w.Code, w.Body.String())
	}

	// admin 停用 viewer → 200，且该 viewer 会话立即失效、无法再登录
	if w := req(t, r, http.MethodDelete, "/api/users/"+itoa64(guestID), "", adminCookie); w.Code != http.StatusOK {
		t.Fatalf("admin 停用 viewer 应 200，实际 %d %s", w.Code, w.Body.String())
	}
	if w := req(t, r, http.MethodGet, "/api/probe-read", "", viewerCookie); w.Code != http.StatusUnauthorized {
		t.Errorf("被停用 viewer 的旧会话应 401，实际 %d", w.Code)
	}
	if _, _, _, err := svc.Login("guest", "pass123"); err != ErrInvalidCredentials {
		t.Errorf("被停用账号登录应报 ErrInvalidCredentials，实际 %v", err)
	}

	// owner 重置他人密码 → 200
	guest3 := userIDByName(t, listUsers(t, svc, owner.OrgID), "guest2")
	if w := req(t, r, http.MethodPost, "/api/users/"+itoa64(guest3)+"/reset-password", "", ownerCookie); w.Code != http.StatusOK {
		t.Errorf("owner 重置密码应 200，实际 %d %s", w.Code, w.Body.String())
	}
}

// TestLastOwnerProtection 最后一个 owner 保护：可停用多余 owner，但不得让系统失去全部 owner。
func TestLastOwnerProtection(t *testing.T) {
	svc, r := newRoleEnv(t)
	owner := registerOwner(t, svc, "甲村", "admin", "s3cret")
	ownerCookie, _ := loginAs(t, r, "admin", "s3cret")
	second := seedUserDirect(t, svc, owner.OrgID, "owner2", RoleOwner, StatusActive)

	// 两个启用的 owner：可停用其中一个
	if w := req(t, r, http.MethodDelete, "/api/users/"+itoa64(second.ID), "", ownerCookie); w.Code != http.StatusOK {
		t.Fatalf("两 owner 时停用其一应 200，实际 %d %s", w.Code, w.Body.String())
	}
	// 仅剩一个启用的 owner：再改该 owner 的角色应被拒（LAST_OWNER）
	if w := req(t, r, http.MethodPatch, "/api/users/"+itoa64(second.ID), `{"role":"admin"}`, ownerCookie); w.Code != http.StatusBadRequest || errCode(t, w.Body.Bytes()) != "LAST_OWNER" {
		t.Errorf("失去最后 owner 应 400 LAST_OWNER，实际 %d %s", w.Code, w.Body.String())
	}
}

// TestResolveRejectsDisabled 会话校验：账号被停用后，即便会话行还在也拒绝（软停用即时生效）。
func TestResolveRejectsDisabled(t *testing.T) {
	svc, r := newRoleEnv(t)
	owner := registerOwner(t, svc, "甲村", "admin", "s3cret")
	ownerCookie, _ := loginAs(t, r, "admin", "s3cret")
	if w := createUserViaAPI(t, r, ownerCookie, "guest", RoleViewer); w.Code != http.StatusOK {
		t.Fatalf("建 viewer 失败: %d %s", w.Code, w.Body.String())
	}
	guestCookie, _ := loginAs(t, r, "guest", "pass123")
	guestID := userIDByName(t, listUsers(t, svc, owner.OrgID), "guest")

	// 直改状态为停用（不删会话），模拟「会话行仍在但账号已停用」
	if err := svc.repo.UpdateRoleStatus(guestID, RoleViewer, StatusDisabled); err != nil {
		t.Fatalf("停用失败: %v", err)
	}
	if w := req(t, r, http.MethodGet, "/api/probe-read", "", guestCookie); w.Code != http.StatusUnauthorized {
		t.Errorf("停用账号的会话应 401，实际 %d %s", w.Code, w.Body.String())
	}
}

// TestResetPasswordSemantics 忘记密码：无用户 → 404；单 owner → 允许；多 owner → 403。
func TestResetPasswordSemantics(t *testing.T) {
	svc, r := newRoleEnv(t)

	// 无用户
	w := req(t, r, http.MethodPost, "/api/auth/reset-password", "", nil)
	if w.Code != http.StatusNotFound || errCode(t, w.Body.Bytes()) != "NO_USER" {
		t.Fatalf("无用户时忘记密码应 404 NO_USER，实际 %d %s", w.Code, w.Body.String())
	}

	owner := registerOwner(t, svc, "甲村", "admin", "s3cret")
	// 单 owner：允许重置为 admin888
	w = req(t, r, http.MethodPost, "/api/auth/reset-password", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("单 owner 忘记密码应 200，实际 %d %s", w.Code, w.Body.String())
	}
	if _, _, role, err := svc.Login("admin", "admin888"); err != nil || role != RoleOwner {
		t.Errorf("重置后应能以 admin888 登录且角色 owner，err=%v role=%s", err, role)
	}

	// 多 owner：拒绝
	seedUserDirect(t, svc, owner.OrgID, "owner2", RoleOwner, StatusActive)
	w = req(t, r, http.MethodPost, "/api/auth/reset-password", "", nil)
	if w.Code != http.StatusForbidden || errCode(t, w.Body.Bytes()) != "RESET_NOT_ALLOWED" {
		t.Errorf("多 owner 忘记密码应 403 RESET_NOT_ALLOWED，实际 %d %s", w.Code, w.Body.String())
	}
}
