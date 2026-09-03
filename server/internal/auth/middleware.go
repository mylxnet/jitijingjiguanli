package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/platform"
)

// ctxUserIDKey 是会话用户 id 在 gin 上下文中的键。
const ctxUserIDKey = "userID"

// RequireAuth 校验会话 Cookie；未登录或已过期返回 401（前端收到 401 跳转登录页）。
func RequireAuth(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(CookieName)
		if err != nil {
			platform.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "未登录或登录已过期")
			c.Abort()
			return
		}
		userID, err := svc.Resolve(token)
		if err != nil {
			platform.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "未登录或登录已过期")
			c.Abort()
			return
		}
		c.Set(ctxUserIDKey, userID)
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
