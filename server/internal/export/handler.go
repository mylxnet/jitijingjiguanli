package export

import (
	"database/sql"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/category"
	"jititaizhang/server/internal/platform"
	"jititaizhang/server/internal/summary"
	"jititaizhang/server/internal/transaction"
)

// Handler 处理导出相关 HTTP 请求。
type Handler struct {
	renderer *renderer
}

// NewHandler 创建 Handler（与全库约定一致：只接 db，内部自建依赖 repo）。
func NewHandler(db *sql.DB) *Handler {
	return &Handler{
		renderer: &renderer{
			sum: summary.NewRepo(db),
			txn: transaction.NewRepo(db),
			cat: category.NewRepo(db),
		},
	}
}

// Register 挂载路由。
func (h *Handler) Register(r gin.IRouter) {
	r.GET("/api/export", h.Export)
}

// Export GET /api/export?content=&format=&from=&to=&categoryId=&keyword=&minAmount=&maxAmount=&includeVoided=
func (h *Handler) Export(c *gin.Context) {
	content := Content(c.Query("content"))
	format := Format(c.DefaultQuery("format", "xlsx"))

	if content != ContentTransactions && content != ContentSummary && content != ContentBalanceSheet {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_CONTENT", Message: "导出内容不合法（transactions / summary / balance_sheet）",
		})
		return
	}
	if format != FormatCSV && format != FormatXLSX {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_FORMAT", Message: "导出格式不合法（csv / xlsx）",
		})
		return
	}

	q := ExportQuery{
		From:          c.Query("from"),
		To:            c.Query("to"),
		Keyword:       c.Query("keyword"),
		IncludeVoided: c.Query("includeVoided") == "true",
	}
	if cid := c.Query("categoryId"); cid != "" {
		if id, err := strconv.ParseInt(cid, 10, 64); err == nil {
			q.CategoryID = &id
		}
	}
	if v := c.Query("minAmount"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			q.MinAmount = &n
		}
	}
	if v := c.Query("maxAmount"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			q.MaxAmount = &n
		}
	}

	// 渲染目标工作表
	var sheet *xlSheet
	switch content {
	case ContentTransactions:
		s, err := h.renderer.txnSheet(q)
		if err != nil {
			h.fail500(c, "导出流水失败")
			return
		}
		sheet = s
	case ContentSummary:
		s, err := h.renderer.summarySheet(q)
		if err != nil {
			h.fail500(c, "导出收支汇总失败")
			return
		}
		sheet = s
	case ContentBalanceSheet:
		s, err := h.renderer.balanceSheet(q)
		if err != nil {
			h.fail500(c, "导出科目余额表失败")
			return
		}
		sheet = s
	}

	// 响应头
	nameCn := map[Content]string{ContentTransactions: "收支流水", ContentSummary: "收支汇总", ContentBalanceSheet: "科目余额表"}[content]
	ext := string(format)
	stamp := time.Now().Format("20060102")
	filename := fmt.Sprintf("集体台账-%s-%s.%s", nameCn, stamp, ext)
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": filename})

	switch format {
	case FormatCSV:
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", disposition)
		if err := writeCSV(c.Writer, sheet); err != nil {
			return
		}
	case FormatXLSX:
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", disposition)
		if err := writeXLSX(c.Writer, sheet); err != nil {
			return
		}
	}
}

func (h *Handler) fail500(c *gin.Context, msg string) {
	platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
		Code: "INTERNAL_ERROR", Message: msg + "，请重试",
	})
}
