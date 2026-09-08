package transaction

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/auth"
	"jititaizhang/server/internal/category"
	"jititaizhang/server/internal/changelog"
	"jititaizhang/server/internal/platform"
)

// dateLayout 流水业务日期格式（与表内 TEXT 约定一致）。
const dateLayout = "2006-01-02"

func validDate(s string) bool {
	_, err := time.Parse(dateLayout, s)
	return err == nil
}

// Handler 处理流水相关 HTTP 请求。
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
	r.GET("/api/transactions", h.ListTransactions)
	r.POST("/api/transactions", h.CreateTransaction)
	r.PUT("/api/transactions/:id", h.UpdateTransaction)
}

// CreateTransaction 记一笔。
// POST /api/transactions
func (h *Handler) CreateTransaction(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}

	var req CreateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "参数不合法",
		})
		return
	}

	// 校验金额
	if req.AmountCents <= 0 {
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

	// 校验科目
	cat, err := h.catRepo.FindByID(req.CategoryID)
	if err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "服务暂时不可用",
		})
		return
	}
	if cat == nil {
		platform.ErrResponse(c, http.StatusBadRequest, platform.ErrCategoryNotFound)
		return
	}
	if cat.Level != 2 {
		platform.ErrResponse(c, http.StatusBadRequest, platform.ErrCategoryLevel2Only)
		return
	}
	if cat.Status != "active" {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "CATEGORY_INACTIVE", Message: "该科目已停用，请选择其他科目",
		})
		return
	}
	if cat.OrgID != orgID {
		platform.ErrResponse(c, http.StatusNotFound, platform.ErrCategoryNotFound)
		return
	}
	if cat.Kind == "asset" {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "CATEGORY_IS_ASSET", Message: "资产型科目不能直接记收/支，请使用「资金划转」",
		})
		return
	}

	// 校验方向
	if req.Direction != "income" && req.Direction != "expense" {
		platform.ErrResponse(c, http.StatusBadRequest, platform.ErrInvalidDirection)
		return
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}
	txn := &Transaction{
		OrgID:       orgID,
		TxnDate:     req.TxnDate,
		Direction:   req.Direction,
		AmountCents: req.AmountCents,
		CategoryID:  req.CategoryID,
		Note:        note,
	}

	created, err := h.repo.Create(txn)
	if err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "保存流水失败",
		})
		return
	}

	// 记录创建日志
	h.clRepo.LogCreate(orgID, "transaction", created.ID)

	platform.SuccessResponse(c, created)
}

func (h *Handler) unauthorized(c *gin.Context) {
	platform.ErrResponse(c, http.StatusUnauthorized, &platform.AppError{
		Code: "UNAUTHORIZED", Message: "未登录或登录已过期",
	})
}

// ListTransactions 流水列表 + 筛选。
// GET /api/transactions
func (h *Handler) ListTransactions(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	from := c.Query("from")
	to := c.Query("to")
	keyword := c.Query("keyword")
	direction := c.Query("direction")
	includeVoided := c.Query("includeVoided") == "true"

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

	var minAmount, maxAmount *int64
	if min := c.Query("minAmount"); min != "" {
		if v, err := strconv.ParseInt(min, 10, 64); err == nil {
			minAmount = &v
		}
	}
	if max := c.Query("maxAmount"); max != "" {
		if v, err := strconv.ParseInt(max, 10, 64); err == nil {
			maxAmount = &v
		}
	}

	items, total, err := h.repo.List(orgID, from, to, categoryID, keyword, direction, minAmount, maxAmount, includeVoided, page, pageSize)
	if err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "查询流水失败",
		})
		return
	}

	incomeTotal, expenseTotal, err := h.repo.GetSummary(orgID, from, to, categoryID, keyword, direction, minAmount, maxAmount, includeVoided)
	if err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "计算汇总失败",
		})
		return
	}

	if items == nil {
		items = []*Transaction{}
	}

	resp := ListTransactionsResponse{
		Items: toSlice(items),
		Total: total,
	}
	resp.Summary.IncomeTotal = incomeTotal
	resp.Summary.ExpenseTotal = expenseTotal
	resp.Summary.Balance = incomeTotal - expenseTotal

	platform.SuccessResponse(c, resp)
}

// UpdateTransaction 编辑流水 / 作废 / 撤销作废。
// PUT /api/transactions/:id
func (h *Handler) UpdateTransaction(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "流水 ID 不合法",
		})
		return
	}

	txn, err := h.repo.FindByID(id)
	if err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "服务暂时不可用",
		})
		return
	}
	if txn == nil || txn.OrgID != orgID {
		platform.ErrResponse(c, http.StatusNotFound, platform.ErrTransactionNotFound)
		return
	}

	var req struct {
		Date        *string `json:"date"`
		Direction   *string `json:"direction"`
		AmountCents *int64  `json:"amountCents"`
		CategoryID  *int64  `json:"categoryId"`
		Note        *string `json:"note"`
		Status      *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "参数不合法",
		})
		return
	}

	// 对要修改的字段做与 Create 一致的校验（避免直接撞 DB CHECK 返回 500）
	if req.Date != nil && !validDate(*req.Date) {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_DATE", Message: "日期格式不合法，应为 YYYY-MM-DD",
		})
		return
	}
	if req.Direction != nil && *req.Direction != "income" && *req.Direction != "expense" {
		platform.ErrResponse(c, http.StatusBadRequest, platform.ErrInvalidDirection)
		return
	}
	if req.AmountCents != nil && *req.AmountCents <= 0 {
		platform.ErrResponse(c, http.StatusBadRequest, platform.ErrInvalidAmount)
		return
	}
	if req.Status != nil && *req.Status != "normal" && *req.Status != "voided" {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_STATUS", Message: "流水状态不合法",
		})
		return
	}
	if req.CategoryID != nil && *req.CategoryID != txn.CategoryID {
		cat, err := h.catRepo.FindByID(*req.CategoryID)
		if err != nil {
			platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
				Code: "INTERNAL_ERROR", Message: "服务暂时不可用",
			})
			return
		}
		if cat == nil {
			platform.ErrResponse(c, http.StatusBadRequest, platform.ErrCategoryNotFound)
			return
		}
		if cat.Level != 2 {
			platform.ErrResponse(c, http.StatusBadRequest, platform.ErrCategoryLevel2Only)
			return
		}
		if cat.Status != "active" {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "CATEGORY_INACTIVE", Message: "该科目已停用，请选择其他科目",
			})
			return
		}
		if cat.OrgID != orgID {
			platform.ErrResponse(c, http.StatusNotFound, platform.ErrCategoryNotFound)
			return
		}
		if cat.Kind == "asset" {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "CATEGORY_IS_ASSET", Message: "资产型科目不能直接记收/支，请使用「资金划转」",
			})
			return
		}
	}

	updates := make(map[string]any)
	if req.Date != nil {
		updates["txn_date"] = *req.Date
	}
	if req.Direction != nil {
		updates["direction"] = *req.Direction
	}
	if req.AmountCents != nil {
		updates["amount_cents"] = *req.AmountCents
	}
	if req.CategoryID != nil {
		updates["category_id"] = *req.CategoryID
	}
	if req.Note != nil {
		updates["note"] = *req.Note
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if err := h.repo.Update(id, orgID, updates); err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "更新流水失败",
		})
		return
	}

	// 记录变更日志
	if req.Status != nil && *req.Status != txn.Status {
		action := "void"
		if *req.Status == "normal" {
			action = "unvoid"
		}
		field := "status"
		old, new := txn.Status, *req.Status
		_ = h.clRepo.LogChange(orgID, "transaction", id, action, &field, &old, &new)
	}
	if req.Date != nil && *req.Date != txn.TxnDate {
		h.clRepo.LogUpdateField(orgID, "transaction", id, "txn_date", txn.TxnDate, *req.Date)
	}
	if req.Direction != nil && *req.Direction != txn.Direction {
		h.clRepo.LogUpdateField(orgID, "transaction", id, "direction", txn.Direction, *req.Direction)
	}
	if req.AmountCents != nil && *req.AmountCents != txn.AmountCents {
		oldStr := strconv.FormatInt(txn.AmountCents, 10)
		newStr := strconv.FormatInt(*req.AmountCents, 10)
		h.clRepo.LogUpdateField(orgID, "transaction", id, "amount_cents", oldStr, newStr)
	}
	if req.CategoryID != nil && *req.CategoryID != txn.CategoryID {
		oldStr := strconv.FormatInt(txn.CategoryID, 10)
		newStr := strconv.FormatInt(*req.CategoryID, 10)
		h.clRepo.LogUpdateField(orgID, "transaction", id, "category_id", oldStr, newStr)
	}
	if req.Note != nil {
		oldNote := ""
		if txn.Note != nil {
			oldNote = *txn.Note
		}
		if *req.Note != oldNote {
			h.clRepo.LogUpdateField(orgID, "transaction", id, "note", oldNote, *req.Note)
		}
	}

	updated, _ := h.repo.FindByID(id)
	platform.SuccessResponse(c, updated)
}

func toSlice(items []*Transaction) []Transaction {
	result := make([]Transaction, len(items))
	for i, item := range items {
		result[i] = *item
	}
	return result
}