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
	r.POST("/api/auth/logout", h.Logout)
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
