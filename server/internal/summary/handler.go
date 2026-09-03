package summary

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/auth"
	"jititaizhang/server/internal/platform"
)

// Handler 处理汇总查询 HTTP 请求。
type Handler struct {
	repo *Repo
}

// NewHandler 创建 Handler（与全库约定一致：只接 db）。
func NewHandler(db *sql.DB) *Handler {
	return &Handler{repo: NewRepo(db)}
}

// Register 挂载路由。
func (h *Handler) Register(r gin.IRouter) {
	r.GET("/api/summary", h.GetSummary)
}

// GetSummary 获取当前组织的汇总数据。
// GET /api/summary?from=...&to=...（空=全年）
func (h *Handler) GetSummary(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		platform.ErrResponse(c, http.StatusUnauthorized, &platform.AppError{
			Code: "UNAUTHORIZED", Message: "未登录或登录已过期",
		})
		return
	}
	resp, err := h.repo.GetSummary(orgID, c.Query("from"), c.Query("to"))
	if err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "查询汇总失败",
		})
		return
	}
	platform.SuccessResponse(c, resp)
}
