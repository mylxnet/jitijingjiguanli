// 账号管理 HTTP 接口（挂在 admin 组：owner/admin 可访问，见 docs/27 §5.3）。

package auth

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/platform"
)

// RegisterUsers 挂载账号管理路由（由 main 在 admin 组内调用）。
func (h *Handler) RegisterUsers(r gin.IRouter) {
	r.GET("/api/users", h.ListUsers)
	r.POST("/api/users", h.CreateUser)
	r.PATCH("/api/users/:id", h.UpdateUser)
	r.DELETE("/api/users/:id", h.DisableUser)
	r.POST("/api/users/:id/reset-password", h.ResetUserPassword)
}

// ListUsers GET /api/users —— 列出本组织账号。
func (h *Handler) ListUsers(c *gin.Context) {
	orgID, ok := CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	users, err := h.svc.ListUsers(orgID)
	if err != nil {
		platform.Fail(c, http.StatusInternalServerError, "LIST_USERS_FAILED", "读取账号列表失败")
		return
	}
	if users == nil {
		users = []*User{}
	}
	platform.OK(c, gin.H{"items": users})
}

// CreateUser POST /api/users —— 新建账号。
func (h *Handler) CreateUser(c *gin.Context) {
	actor, ok := IdentityOf(c)
	if !ok {
		h.unauthorized(c)
		return
	}

	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platform.Fail(c, http.StatusBadRequest, "INVALID_REQUEST", "请填写账号、初始密码与角色")
		return
	}

	u, err := h.svc.CreateUser(actor.Role, actor.OrgID, req.Username, req.Password, req.Role)
	if err != nil {
		h.failUserOp(c, err)
		return
	}
	platform.OK(c, gin.H{"user": u})
}

// UpdateUser PATCH /api/users/:id —— 改角色 / 启用停用。
func (h *Handler) UpdateUser(c *gin.Context) {
	actor, ok := IdentityOf(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	targetID, ok := h.pathID(c)
	if !ok {
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platform.Fail(c, http.StatusBadRequest, "INVALID_REQUEST", "参数不合法")
		return
	}
	if req.Role == nil && req.Status == nil {
		platform.Fail(c, http.StatusBadRequest, "INVALID_REQUEST", "请提供要修改的角色或状态")
		return
	}

	u, err := h.svc.UpdateUser(actor, actor.OrgID, targetID, req.Role, req.Status)
	if err != nil {
		h.failUserOp(c, err)
		return
	}
	platform.OK(c, gin.H{"user": u})
}

// DisableUser DELETE /api/users/:id —— 停用账号（软删）+ 强制下线。
func (h *Handler) DisableUser(c *gin.Context) {
	actor, ok := IdentityOf(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	targetID, ok := h.pathID(c)
	if !ok {
		return
	}

	disabled := StatusDisabled
	if _, err := h.svc.UpdateUser(actor, actor.OrgID, targetID, nil, &disabled); err != nil {
		h.failUserOp(c, err)
		return
	}
	platform.OK(c, gin.H{"ok": true})
}

// ResetUserPassword POST /api/users/:id/reset-password —— 重置为初始密码 admin888 并强制下线。
func (h *Handler) ResetUserPassword(c *gin.Context) {
	actor, ok := IdentityOf(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	targetID, ok := h.pathID(c)
	if !ok {
		return
	}

	if err := h.svc.ResetUserPassword(actor, actor.OrgID, targetID); err != nil {
		h.failUserOp(c, err)
		return
	}
	platform.OK(c, gin.H{"ok": true, "message": "密码已重置为 admin888"})
}

// pathID 解析路径参数 :id。
func (h *Handler) pathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		platform.Fail(c, http.StatusBadRequest, "INVALID_REQUEST", "账号 id 不合法")
		return 0, false
	}
	return id, true
}

// unauthorized 未登录统一响应。
func (h *Handler) unauthorized(c *gin.Context) {
	platform.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "未登录或登录已过期")
}

// failUserOp 账号管理业务错误 → 统一响应。
func (h *Handler) failUserOp(c *gin.Context, err error) {
	switch err {
	case ErrUserNotFound:
		platform.Fail(c, http.StatusNotFound, "USER_NOT_FOUND", "账号不存在")
	case ErrUsernameTaken:
		platform.Fail(c, http.StatusBadRequest, "USERNAME_TAKEN", "该账号已存在，请更换")
	case ErrInvalidPassword:
		platform.Fail(c, http.StatusBadRequest, "INVALID_PASSWORD", "密码至少 6 位")
	case ErrInvalidUsername:
		platform.Fail(c, http.StatusBadRequest, "INVALID_USERNAME", "请填写账号")
	case ErrInvalidRole:
		platform.Fail(c, http.StatusBadRequest, "INVALID_ROLE", "角色不合法")
	case ErrForbiddenRole:
		platform.Fail(c, http.StatusForbidden, "FORBIDDEN", "无权操作该账号")
	case ErrSelfOperation:
		platform.Fail(c, http.StatusBadRequest, "SELF_OPERATION", "不能对自己执行该操作")
	case ErrLastOwner:
		platform.Fail(c, http.StatusBadRequest, "LAST_OWNER", "必须保留至少一个所有者账号")
	default:
		platform.Fail(c, http.StatusInternalServerError, "USER_OP_FAILED", "操作失败，请重试")
	}
}
