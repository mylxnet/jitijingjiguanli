package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/platform"
)

// 会话身份在 gin 上下文中的键。
// 沿用既有 userID/orgID 键名（测试基建依赖），并按多用户需要补充 role/username。
const (
	ctxUserIDKey   = "userID"
	ctxOrgIDKey    = "orgID"
	ctxRoleKey     = "role"
	ctxUsernameKey = "username"
)

// RequireAuth 校验会话 Cookie；未登录或已过期返回 401（前端收到 401 跳转登录页）。
// 校验成功后把 用户 id / 所属组织 id / 角色 / 用户名 写入上下文
// （v0.3 F9 数据隔离 + docs/27 角色鉴权与操作留痕的共同依据）。
func RequireAuth(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(CookieName)
		if err != nil {
			platform.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "未登录或登录已过期")
			c.Abort()
			return
		}
		id, err := svc.Resolve(token)
		if err != nil {
			platform.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "未登录或登录已过期")
			c.Abort()
			return
		}
		c.Set(ctxUserIDKey, id.UserID)
		c.Set(ctxOrgIDKey, id.OrgID)
		c.Set(ctxRoleKey, id.Role)
		c.Set(ctxUsernameKey, id.Username)
		c.Next()
	}
}

// CurrentUserID 从 gin 上下文取出当前用户 id。仅可在 RequireAuth 之后的处理器中调用。
func CurrentUserID(c *gin.Context) (int64, bool) {
	v, ok := c.Get(ctxUserIDKey)
	if !ok {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok
}

// CurrentOrgID 从 gin 上下文取出当前用户所属组织 id（多组织数据隔离）。
// 仅可在 RequireAuth 之后的处理器中调用。
func CurrentOrgID(c *gin.Context) (int64, bool) {
	v, ok := c.Get(ctxOrgIDKey)
	if !ok {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok
}

// CurrentRole 从 gin 上下文取出当前账号角色（owner/admin/viewer）。
// 仅可在 RequireAuth 之后的处理器中调用。
func CurrentRole(c *gin.Context) (string, bool) {
	v, ok := c.Get(ctxRoleKey)
	if !ok {
		return "", false
	}
	role, ok := v.(string)
	return role, ok
}

// CurrentUsername 从 gin 上下文取出当前登录账号名（操作日志留痕用）。
// 仅可在 RequireAuth 之后的处理器中调用。
func CurrentUsername(c *gin.Context) string {
	v, ok := c.Get(ctxUsernameKey)
	if !ok {
		return ""
	}
	name, _ := v.(string)
	return name
}

// IdentityOf 组装当前登录身份（用户 id / 组织 id / 用户名 / 角色）。
// 仅可在 RequireAuth 之后的处理器中调用。
func IdentityOf(c *gin.Context) (*Identity, bool) {
	userID, ok := CurrentUserID(c)
	if !ok {
		return nil, false
	}
	orgID, _ := CurrentOrgID(c)
	role, _ := CurrentRole(c)
	return &Identity{
		UserID:   userID,
		OrgID:    orgID,
		Username: CurrentUsername(c),
		Role:     role,
	}, true
}

// RequireRole 要求当前账号角色在允许列表内，否则 403。
// 必须挂在 RequireAuth 之后（依赖上下文中的角色）。
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		role, ok := CurrentRole(c)
		if !ok || !allowed[role] {
			platform.Fail(c, http.StatusForbidden, "FORBIDDEN", "当前账号无操作权限")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireWrite 只读账号（viewer）写操作拦截：约定「非 GET/HEAD 即写操作」。
// 组级别统一挂载，业务 handler 无需逐个判断；admin/owner 一律放行。
// 自服务端点（登出、修改自己的密码）不在本中间件作用域内（挂在 read 组）。
func RequireWrite() gin.HandlerFunc {
	return func(c *gin.Context) {
		m := c.Request.Method
		if m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions {
			c.Next()
			return
		}
		role, ok := CurrentRole(c)
		if !ok {
			platform.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "未登录或登录已过期")
			c.Abort()
			return
		}
		if role != RoleOwner && role != RoleAdmin {
			platform.Fail(c, http.StatusForbidden, "FORBIDDEN", "当前账号无操作权限")
			c.Abort()
			return
		}
		c.Next()
	}
}
