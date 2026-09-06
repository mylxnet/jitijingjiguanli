package auth

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/platform"
)

// Handler 暴露 /api/auth/* 接口。
type Handler struct {
	svc *Service
}

// NewHandler 创建认证接口处理器。
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register 挂载路由。
func (h *Handler) Register(r gin.IRouter) {
	r.POST("/api/auth/login", h.Login)
	r.POST("/api/auth/register", h.RegisterOrg)
	r.POST("/api/auth/logout", h.Logout)
	r.POST("/api/auth/reset-password", h.ResetPassword)
}

// RegisterAuthed 挂载需要登录的路由（由 main 在鉴权组内调用）。
func (h *Handler) RegisterAuthed(r gin.IRouter) {
	r.PUT("/api/auth/password", h.ChangePassword)
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}

// ChangePassword 修改当前登录账号口令（校验原口令）。
// PUT /api/auth/password
func (h *Handler) ChangePassword(c *gin.Context) {
	userID, ok := CurrentUserID(c)
	if !ok {
		platform.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "未登录或登录已过期")
		return
	}

	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platform.Fail(c, http.StatusBadRequest, "INVALID_REQUEST", "请填写原密码与新密码")
		return
	}

	err := h.svc.ChangePassword(userID, req.OldPassword, req.NewPassword)
	switch err {
	case nil:
		platform.OK(c, gin.H{"ok": true})
	case ErrOldPasswordWrong:
		platform.Fail(c, http.StatusBadRequest, "OLD_PASSWORD_WRONG", "原密码错误")
	case ErrInvalidPassword:
		platform.Fail(c, http.StatusBadRequest, "INVALID_PASSWORD", "密码至少 6 位")
	default:
		platform.Fail(c, http.StatusInternalServerError, "CHANGE_PASSWORD_FAILED", "修改密码失败，请重试")
	}
}

// RegisterOrg POST /api/auth/register —— 自助注册组织（v0.3 F8）：
// 组织名 + 管理员账号 + 密码 → 建组织/账号/预置科目并自动登录。
func (h *Handler) RegisterOrg(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platform.Fail(c, http.StatusBadRequest, "INVALID_REQUEST", "请填写组织名称、账号与密码")
		return
	}

	token, expiresAt, err := h.svc.RegisterOrg(req.OrgName, req.Username, req.Password)
	if err != nil {
		code := "REGISTER_FAILED"
		msg := "注册失败，请重试"
		switch err {
			case ErrInvalidOrgName:
				code, msg = "INVALID_ORG_NAME", "请填写组织名称"
			case ErrInvalidPassword:
				code, msg = "INVALID_PASSWORD", "密码至少 6 位"
			case ErrUsernameTaken:
				code, msg = "USERNAME_TAKEN", "该账号已存在，请更换"
			case ErrRegistrationClosed:
				code, msg = "REGISTRATION_CLOSED", "系统已注册，禁止重复注册"
			}
		platform.Fail(c, http.StatusBadRequest, code, msg)
		return
	}

	setSessionCookie(c, token, expiresAt)
	platform.OK(c, gin.H{
		"orgName":   req.OrgName,
		"user":      gin.H{"username": req.Username},
		"expiresAt": expiresAt,
	})
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login POST /api/auth/login
// 成功：写入 HttpOnly 会话 Cookie 并返回 { user: { id, username } }
// 失败：401 + 统一错误体；不区分账号不存在与密码错误，避免账号枚举。
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platform.Fail(c, http.StatusBadRequest, "INVALID_REQUEST", "请填写账号和密码")
		return
	}

	token, expiresAt, err := h.svc.Login(req.Username, req.Password)
	if err != nil {
		platform.Fail(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "账号或密码错误")
		return
	}

	setSessionCookie(c, token, expiresAt)
	platform.OK(c, gin.H{"user": gin.H{"username": req.Username}, "expiresAt": expiresAt})
}

// Logout POST /api/auth/logout —— 删除服务端会话并清除 Cookie。
func (h *Handler) Logout(c *gin.Context) {
	token, _ := c.Cookie(CookieName)
	if err := h.svc.Logout(token); err != nil {
		platform.Fail(c, http.StatusInternalServerError, "LOGOUT_FAILED", "登出失败，请重试")
		return
	}
	clearSessionCookie(c)
	platform.OK(c, gin.H{"ok": true})
}

// ResetPassword POST /api/auth/reset-password —— 重置密码为 admin888（忘记密码）。
func (h *Handler) ResetPassword(c *gin.Context) {
	username, err := h.svc.ResetPassword()
	if err != nil {
		platform.Fail(c, http.StatusNotFound, "NO_USER", "系统中没有注册用户")
		return
	}
	platform.OK(c, gin.H{"ok": true, "message": "密码已重置为 admin888，请登录后修改", "username": username})
}

// setSessionCookie 写入会话 Cookie。
// HttpOnly + SameSite=Lax；Secure 仅在显式开启 HTTPS（APP_SECURE_COOKIE=1）时设置，
// 因为局域网 HTTP 直连场景下 Secure 会导致 Cookie 不被保存。
func setSessionCookie(c *gin.Context, token string, expiresAt time.Time) {
	secure := os.Getenv("APP_SECURE_COOKIE") == "1"
	maxAge := int(time.Until(expiresAt).Seconds())
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(CookieName, token, maxAge, "/", "", secure, true)
}

func clearSessionCookie(c *gin.Context) {
	secure := os.Getenv("APP_SECURE_COOKIE") == "1"
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(CookieName, "", -1, "/", "", secure, true)
}
