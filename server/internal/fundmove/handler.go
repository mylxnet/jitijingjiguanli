package fundmove

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/auth"
	"jititaizhang/server/internal/category"
	"jititaizhang/server/internal/changelog"
	"jititaizhang/server/internal/platform"
	"jititaizhang/server/internal/settings"
)

// dateLayout 资金划转业务日期格式（与表内 TEXT 约定一致）。
const dateLayout = "2006-01-02"

func validDate(s string) bool {
	_, err := time.Parse(dateLayout, s)
	return err == nil
}

// Handler 处理资金划转相关 HTTP 请求。
type Handler struct {
	repo    *Repo
	catRepo *category.Repo
	setRepo *settings.Repo
	clRepo  *changelog.Repo
}

// NewHandler 创建 Handler（与全库约定一致：只接 db，内部自建依赖 repo）。
func NewHandler(db *sql.DB) *Handler {
	return &Handler{
		repo:    NewRepo(db),
		catRepo: category.NewRepo(db),
		setRepo: settings.NewRepo(db),
		clRepo:  changelog.NewRepo(db),
	}
}

// Register 挂载路由。
func (h *Handler) Register(r gin.IRouter) {
	r.GET("/api/fund-moves", h.ListFundMoves)
	r.POST("/api/fund-moves", h.CreateFundMove)
	r.PUT("/api/fund-moves/:id", h.UpdateFundMove)
}

// bankBalance 计算银行存款当前余额（D10 口径）：
//
//	bank = 期初 + Σ(收 − 支，全年 normal) ± 资金划转净额（Σ收回 − Σ投资）
func (h *Handler) bankBalance(orgID int64) (int64, error) {
	set, err := h.setRepo.Get(orgID)
	if err != nil {
		return 0, err
	}
	var txnNet int64
	if err := h.repo.db.QueryRow(
		`SELECT COALESCE(SUM(CASE WHEN direction='income' THEN amount_cents ELSE -amount_cents END), 0)
		 FROM txn WHERE org_id = ? AND status = 'normal'`, orgID,
	).Scan(&txnNet); err != nil {
		return 0, fmt.Errorf("聚合收支净额失败: %w", err)
	}
	delta, err := h.catRepo.BankDelta(orgID)
	if err != nil {
		return 0, err
	}
	return set.BankOpeningBalanceCents + txnNet + delta, nil
}

// CreateFundMove 创建一笔资金划转（投资/收回）。
// POST /api/fund-moves
func (h *Handler) CreateFundMove(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}

	var req CreateFundMoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "参数不合法",
		})
		return
	}
	if req.AmountCents <= 0 {
		platform.ErrResponse(c, http.StatusBadRequest, platform.ErrInvalidAmount)
		return
	}
	if !validDate(req.MoveDate) {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_DATE", Message: "日期格式不合法，应为 YYYY-MM-DD",
		})
		return
	}
	if req.Kind != "invest" && req.Kind != "recover" {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_KIND", Message: "划转方向不合法，应为 invest 投资 / recover 收回",
		})
		return
	}

	// 校验资产科目：须为本组织启用中的资产型二级科目
	asset, err := h.catRepo.FindByID(req.AssetCategoryID)
	if err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "服务暂时不可用",
		})
		return
	}
	if asset == nil {
		platform.ErrResponse(c, http.StatusBadRequest, platform.ErrCategoryNotFound)
		return
	}
	if asset.OrgID != orgID {
		platform.ErrResponse(c, http.StatusNotFound, platform.ErrCategoryNotFound)
		return
	}
	if asset.Level != 2 {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "FUND_MOVE_NOT_ASSET", Message: "资金划转必须挂资产型二级科目",
		})
		return
	}
	if asset.Kind != "asset" {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "FUND_MOVE_NOT_ASSET", Message: "资金划转仅用于资产型科目（对外投资）；普通科目请用收支或科目间转账",
		})
		return
	}
	if asset.Status != "active" {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "CATEGORY_INACTIVE", Message: "资产科目已停用",
		})
		return
	}

	// 投资：金额 ≤ 当前银行存款；收回：金额 ≤ 该资产科目在外余额（防资产余额为负）
	if req.Kind == "invest" {
		bank, err := h.bankBalance(orgID)
		if err != nil {
			platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
				Code: "INTERNAL_ERROR", Message: "服务暂时不可用",
			})
			return
		}
		if req.AmountCents > bank {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code:    "FUND_MOVE_INSUFFICIENT_BANK",
				Message: fmt.Sprintf("银行存款余额不足，当前余额 %.2f", float64(bank)/100),
			})
			return
		}
	} else {
		outstanding, err := h.catRepo.AssetBalance(req.AssetCategoryID)
		if err != nil {
			platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
				Code: "INTERNAL_ERROR", Message: "服务暂时不可用",
			})
			return
		}
		if req.AmountCents > outstanding {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code:    "FUND_MOVE_INSUFFICIENT_ASSET",
				Message: fmt.Sprintf("收回金额超过该资产科目在外金额，当前在外 %.2f", float64(outstanding)/100),
			})
			return
		}
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}
	m := &FundMove{
		OrgID:           orgID,
		MoveDate:        req.MoveDate,
		Kind:            req.Kind,
		AssetCategoryID: req.AssetCategoryID,
		AmountCents:     req.AmountCents,
		Note:            note,
	}

	created, err := h.repo.Create(m)
	if err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "创建资金划转失败",
		})
		return
	}
	h.clRepo.LogCreate(orgID, "fund_move", created.ID)

	platform.SuccessResponse(c, created)
}

// ListFundMoves 查询资金划转记录列表。
// GET /api/fund-moves
func (h *Handler) ListFundMoves(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	from := c.Query("from")
	to := c.Query("to")
	kind := c.Query("kind")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	items, total, err := h.repo.List(orgID, from, to, kind, page, pageSize)
	if err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "查询资金划转记录失败",
		})
		return
	}
	if items == nil {
		items = []FundMove{}
	}

	platform.SuccessResponse(c, FundMoveListResponse{Items: items, Total: total})
}

// UpdateFundMove 作废/撤销资金划转。
// PUT /api/fund-moves/:id
func (h *Handler) UpdateFundMove(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "资金划转 ID 不合法",
		})
		return
	}

	m, err := h.repo.FindByID(id)
	if err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "服务暂时不可用",
		})
		return
	}
	if m == nil || m.OrgID != orgID {
		platform.ErrResponse(c, http.StatusNotFound, &platform.AppError{
			Code: "FUND_MOVE_NOT_FOUND", Message: "资金划转不存在",
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

	if err := h.repo.UpdateStatus(id, orgID, *req.Status, platform.Now()); err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "更新资金划转状态失败",
		})
		return
	}

	if *req.Status != m.Status {
		action := "void"
		if *req.Status == "normal" {
			action = "unvoid"
		}
		h.clRepo.LogChangeVoid(orgID, "fund_move", id, action, m.Status, *req.Status)
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

func (h *Handler) unauthorized(c *gin.Context) {
	platform.ErrResponse(c, http.StatusUnauthorized, &platform.AppError{
		Code: "UNAUTHORIZED", Message: "未登录或登录已过期",
	})
}
