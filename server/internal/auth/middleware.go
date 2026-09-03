package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/platform"
)

// ctxUserIDKey / ctxOrgIDKey 是会话用户/组织在 gin 上下文中的键。
const (
	ctxUserIDKey = "userID"
	ctxOrgIDKey  = "orgID"
)

// RequireAuth 校验会话 Cookie；未登录或已过期返回 401（前端收到 401 跳转登录页）。
// 校验成功后把 用户 id 与其所属组织 id 写入上下文（数据隔离依据，见 v0.3 F9）。
func RequireAuth(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(CookieName)
		if err != nil {
			platform.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "未登录或登录已过期")
			c.Abort()
			return
		}
		userID, orgID, err := svc.Resolve(token)
		if err != nil {
			platform.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "未登录或登录已过期")
			c.Abort()
			return
		}
		c.Set(ctxUserIDKey, userID)
		c.Set(ctxOrgIDKey, orgID)
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
