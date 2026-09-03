package transfer

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/category"
	"jititaizhang/server/internal/changelog"
	"jititaizhang/server/internal/platform"
)

// dateLayout 转账业务日期格式（与表内 TEXT 约定一致）。
const dateLayout = "2006-01-02"

func validDate(s string) bool {
	_, err := time.Parse(dateLayout, s)
	return err == nil
}

// Handler 处理转账相关 HTTP 请求。
type Handler struct {
	repo    *Repo
	catRepo *category.Repo
	clRepo  *changelog.Repo
}

// NewHandler 创建 Handler（与全库约定一致：只接 db，内部自建依赖 repo）。
func NewHandler(db *sql.DB) *Handler {
	return &Handler{
		repo:    NewRepo(db),
		catRepo: category.NewRepo(db),
		clRepo:  changelog.NewRepo(db),
	}
}

// Register 挂载路由。
func (h *Handler) Register(r gin.IRouter) {
	r.GET("/api/transfers", h.ListTransfers)
	r.POST("/api/transfers", h.CreateTransfer)
	r.PUT("/api/transfers/:id", h.UpdateTransfer)
}

// CreateTransfer 创建一笔转账。
// POST /api/transfers
func (h *Handler) CreateTransfer(c *gin.Context) {
	var req CreateTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "参数不合法",
		})
		return
	}

	// 校验金额（R2 前提）
	if req.SourceAmountCents <= 0 {
		platform.ErrResponse(c, http.StatusBadRequest, platform.ErrInvalidAmount)
		return
	}
	// 校验日期格式
	if !validDate(req.TxnDate) {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_DATE", Message: "日期格式不合法，应为 YYYY-MM-DD",
		})
		return
	}

	// 校验转出科目（须为启用中的二级科目；花费型允许作转出方，用于结转清零）
	src, err := h.catRepo.FindByID(req.SourceCategoryID)
	if err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "服务暂时不可用",
		})
		return
	}
	if src == nil {
		platform.ErrResponse(c, http.StatusBadRequest, platform.ErrCategoryNotFound)
		return
	}
	if src.Level != 2 {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "TRANSFER_SOURCE_NOT_LEAF", Message: "转出科目必须为二级科目",
		})
		return
	}
	if src.Status != "active" {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "CATEGORY_INACTIVE", Message: "转出科目已停用",
		})
		return
	}

	// 校验 R2：转出总额 = Σ转入总额
	var totalLegs int64
	for _, leg := range req.Legs {
		if leg.AmountCents <= 0 {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "INVALID_AMOUNT", Message: "转入金额必须大于 0",
			})
			return
		}
		totalLegs += leg.AmountCents
	}
	if totalLegs != req.SourceAmountCents {
		diff := req.SourceAmountCents - totalLegs
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code:    "TRANSFER_AMOUNT_MISMATCH",
			Message: fmt.Sprintf("转出金额 (%.2f) 与转入合计 (%.2f) 不相等，差额 %.2f",
				float64(req.SourceAmountCents)/100, float64(totalLegs)/100, float64(diff)/100),
		})
		return
	}

	// 校验转入科目（R4：花费型不得作转入方）+ 检查是否与转出相同 / 重复
	seen := make(map[int64]bool)
	for _, leg := range req.Legs {
		if leg.CategoryID == req.SourceCategoryID {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "TRANSFER_SAME_CATEGORY", Message: "转入科目不能与转出科目相同",
			})
			return
		}
		if seen[leg.CategoryID] {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "TRANSFER_DUPLICATE_LEG", Message: "同一科目不能有多条转入明细",
			})
			return
		}
		seen[leg.CategoryID] = true

		dst, err := h.catRepo.FindByID(leg.CategoryID)
		if err != nil {
			platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
				Code: "INTERNAL_ERROR", Message: "服务暂时不可用",
			})
			return
		}
		if dst == nil {
			platform.ErrResponse(c, http.StatusBadRequest, platform.ErrCategoryNotFound)
			return
		}
		if dst.Level != 2 {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "TRANSFER_LEG_NOT_LEAF", Message: "转入科目必须为二级科目",
			})
			return
		}
		if dst.Status != "active" {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "CATEGORY_INACTIVE", Message: "转入科目已停用",
			})
			return
		}
		if dst.BalanceType == "spending" {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code:    "TRANSFER_SPENDING_LEG",
				Message: "花费型科目不能作为转入方",
			})
			return
		}
	}

	// 校验 R3：转出金额 ≤ 转出科目当前余额（余额口径复用 category.CalcBalance，含收支与未作废转账）
	currentBal, err := h.catRepo.CalcBalance(req.SourceCategoryID)
	if err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "服务暂时不可用",
		})
		return
	}
	if req.SourceAmountCents > currentBal {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code:    "TRANSFER_INSUFFICIENT_BALANCE",
			Message: fmt.Sprintf("余额不足，当前余额 %.2f", float64(currentBal)/100),
		})
		return
	}

	// 创建
	legs := make([]Leg, len(req.Legs))
	for i, l := range req.Legs {
		legs[i] = Leg{CategoryID: l.CategoryID, AmountCents: l.AmountCents}
	}
	var note *string
	if req.Note != "" {
		note = &req.Note
	}
	t := &Transfer{
		TxnDate:           req.TxnDate,
		SourceCategoryID:  req.SourceCategoryID,
		SourceAmountCents: req.SourceAmountCents,
		Note:              note,
		Legs:              legs,
	}

	created, err := h.repo.Create(t)
	if err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "创建转账失败",
		})
		return
	}

	// 记录创建日志
	h.clRepo.LogCreate("transfer", created.ID)

	platform.SuccessResponse(c, created)
}

// ListTransfers 查询转账记录列表。
// GET /api/transfers
func (h *Handler) ListTransfers(c *gin.Context) {
	from := c.Query("from")
	to := c.Query("to")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	var categoryID *int64
	if cid := c.Query("categoryId"); cid != "" {
		if id, err := strconv.ParseInt(cid, 10, 64); err == nil {
			categoryID = &id
		}
	}

	items, total, err := h.repo.List(from, to, categoryID, page, pageSize)
	if err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "查询转账记录失败",
		})
		return
	}

	if items == nil {
		items = []*Transfer{}
	}

	platform.SuccessResponse(c, TransferListResponse{Items: toSlice(items), Total: total})
}

// UpdateTransfer 作废/撤销转账。
// PUT /api/transfers/:id
func (h *Handler) UpdateTransfer(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "转账 ID 不合法",
		})
		return
	}

	t, err := h.repo.FindByID(id)
	if err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "服务暂时不可用",
		})
		return
	}
	if t == nil {
		platform.ErrResponse(c, http.StatusNotFound, &platform.AppError{
			Code: "TRANSFER_NOT_FOUND", Message: "转账不存在",
		})
		return
	}

	var req struct {
		Status *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Status == nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "参数不合法",
		})
		return
	}

	if *req.Status != "normal" && *req.Status != "voided" {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_STATUS", Message: "状态值不合法",
		})
		return
	}

	if err := h.repo.UpdateStatus(id, *req.Status, platform.Now()); err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "更新转账状态失败",
		})
		return
	}

	// 记录变更日志（仅状态实际变化时）
	if *req.Status != t.Status {
		h.clRepo.LogChangeVoid("transfer", id, t.Status, *req.Status)
	}

	updated, err := h.repo.FindByID(id)
	if err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "服务暂时不可用",
		})
		return
	}
	platform.SuccessResponse(c, updated)
}

func toSlice(items []*Transfer) []Transfer {
	result := make([]Transfer, len(items))
	for i, item := range items {
		result[i] = *item
	}
	return result
}
