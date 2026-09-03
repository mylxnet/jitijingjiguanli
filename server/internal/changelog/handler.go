package changelog

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/auth"
	"jititaizhang/server/internal/platform"
)

// Handler 处理变更日志相关 HTTP 请求。
type Handler struct {
	repo *Repo
}

// NewHandler 创建 Handler（与全库约定一致：只接 db，内部自建依赖 repo）。
func NewHandler(db *sql.DB) *Handler {
	return &Handler{repo: NewRepo(db)}
}

// Register 挂载路由。
func (h *Handler) Register(r gin.IRouter) {
	r.GET("/api/changelog", h.ListChangelog)
}

// ListChangelog 查询变更日志。
// GET /api/changelog?entityType=transaction&entityId=1
func (h *Handler) ListChangelog(c *gin.Context) {
	entityType := c.Query("entityType")
	entityIDStr := c.Query("entityId")

	if entityType == "" || entityIDStr == "" {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "缺少 entityType 或 entityId 参数",
		})
		return
	}

	entityID, err := strconv.ParseInt(entityIDStr, 10, 64)
	if err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "entityId 不合法",
		})
		return
	}

	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		platform.ErrResponse(c, http.StatusUnauthorized, &platform.AppError{
			Code: "UNAUTHORIZED", Message: "未登录或登录已过期",
		})
		return
	}

	items, err := h.repo.ListByEntity(orgID, entityType, entityID)
	if err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "查询变更日志失败",
		})
		return
	}

	if items == nil {
		items = []ChangeLog{}
	}

	platform.SuccessResponse(c, items)
}