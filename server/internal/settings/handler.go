package settings

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/platform"
)

// Handler 处理系统配置相关 HTTP 请求。
type Handler struct {
	repo *Repo
}

// NewHandler 创建 Handler（与全库约定一致：只接 db）。
func NewHandler(db *sql.DB) *Handler {
	return &Handler{repo: NewRepo(db)}
}

// Register 挂载路由。
func (h *Handler) Register(r gin.IRouter) {
	r.GET("/api/settings", h.GetSettings)
	r.PUT("/api/settings", h.UpdateSettings)
}

// GetSettings 读取系统配置。
// GET /api/settings
func (h *Handler) GetSettings(c *gin.Context) {
	s, err := h.repo.Get()
	if err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "读取配置失败",
		})
		return
	}
	platform.SuccessResponse(c, s)
}

// UpdateSettings 修改系统配置。
// PUT /api/settings
func (h *Handler) UpdateSettings(c *gin.Context) {
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "参数不合法",
		})
		return
	}

	if req.BankOpeningBalanceCents != nil {
		if *req.BankOpeningBalanceCents < 0 {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "INVALID_REQUEST", Message: "期初余额不能为负数",
			})
			return
		}
		if err := h.repo.Upsert("bank_opening_balance_cents",
			fmt.Sprintf("%d", *req.BankOpeningBalanceCents)); err != nil {
			platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
				Code: "INTERNAL_ERROR", Message: "保存配置失败",
			})
			return
		}
	}

	s, err := h.repo.Get()
	if err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "读取配置失败",
		})
		return
	}
	platform.SuccessResponse(c, s)
}