package receivable

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/auth"
	"jititaizhang/server/internal/category"
	"jititaizhang/server/internal/changelog"
	"jititaizhang/server/internal/platform"
)

// dateLayout 核销日期格式（与表内 TEXT 约定一致）。
const dateLayout = "2006-01-02"

func validDate(s string) bool {
	_, err := time.Parse(dateLayout, s)
	return err == nil
}

// Handler 处理应收/往来相关 HTTP 请求（D11）。
type Handler struct {
	repo    *Repo
	catRepo *category.Repo
	clRepo  *changelog.Repo
}

// NewHandler 创建 Handler。
func NewHandler(db *sql.DB) *Handler {
	return &Handler{
		repo:    NewRepo(db),
		catRepo: category.NewRepo(db),
		clRepo:  changelog.NewRepo(db),
	}
}

// Register 挂载路由。
func (h *Handler) Register(r gin.IRouter) {
	r.GET("/api/parties", h.ListParties)
	r.POST("/api/parties", h.CreateParty)
	r.PUT("/api/parties/:id", h.UpdateParty)

	r.GET("/api/receivables", h.ListReceivables)
	r.POST("/api/receivables", h.CreateReceivable)
	r.GET("/api/receivables/:id", h.GetReceivableDetail)
	r.POST("/api/receivables/:id/receipts", h.CreateReceipt)
	r.PUT("/api/receipts/:id", h.VoidReceipt)
}

func (h *Handler) unauthorized(c *gin.Context) {
	platform.ErrResponse(c, http.StatusUnauthorized, &platform.AppError{
		Code: "UNAUTHORIZED", Message: "未登录或登录已过期",
	})
}

func (h *Handler) internal(c *gin.Context, msg string) {
	platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
		Code: "INTERNAL_ERROR", Message: msg,
	})
}

// ---------- 往来对象 ----------

// ListParties 往来对象列表（含欠款合计）。
// GET /api/parties?kind=&keyword=
func (h *Handler) ListParties(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	kind := c.Query("kind")
	keyword := c.Query("keyword")

	items, err := h.repo.ListParties(orgID, kind, keyword)
	if err != nil {
		h.internal(c, "查询往来对象失败")
		return
	}
	if items == nil {
		items = []Party{}
	}
	platform.SuccessResponse(c, items)
}

// CreateParty 新建往来对象。
// POST /api/parties
func (h *Handler) CreateParty(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}

	var req CreatePartyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "参数不合法",
		})
		return
	}
	req.Name = trimSpace(req.Name)
	if req.Name == "" {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "请填写对象名称",
		})
		return
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}
	p := &Party{OrgID: orgID, Name: req.Name, Kind: req.Kind, Note: note}
	created, err := h.repo.CreateParty(p)
	if err != nil {
		h.internal(c, "新建往来对象失败")
		return
	}
	h.clRepo.LogCreate(orgID, "party", created.ID)
	platform.SuccessResponse(c, created)
}

// UpdateParty 更新往来对象（名称/类别/备注）。
// PUT /api/parties/:id
func (h *Handler) UpdateParty(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "往来对象 ID 不合法",
		})
		return
	}

	p, err := h.repo.FindPartyByID(id)
	if err != nil {
		h.internal(c, "服务暂时不可用")
		return
	}
	if p == nil || p.OrgID != orgID {
		platform.ErrResponse(c, http.StatusNotFound, &platform.AppError{
			Code: "PARTY_NOT_FOUND", Message: "往来对象不存在",
		})
		return
	}

	var req UpdatePartyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "参数不合法",
		})
		return
	}

	updates := make(map[string]any)
	if req.Name != nil {
		name := trimSpace(*req.Name)
		if name == "" {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "INVALID_REQUEST", Message: "请填写对象名称",
			})
			return
		}
		updates["name"] = name
	}
	if req.Kind != nil {
		updates["kind"] = *req.Kind
	}
	if req.Note != nil {
		if *req.Note == "" {
			updates["note"] = nil
		} else {
			updates["note"] = *req.Note
		}
	}

	if err := h.repo.UpdateParty(id, orgID, updates); err != nil {
		h.internal(c, "更新往来对象失败")
		return
	}

	if v, ok := updates["name"]; ok {
		_ = h.clRepo.LogUpdateField(orgID, "party", id, "name", p.Name, v.(string))
	}
	if v, ok := updates["kind"]; ok {
		_ = h.clRepo.LogUpdateField(orgID, "party", id, "kind", p.Kind, v.(string))
	}
	if v, ok := updates["note"]; ok {
		oldNote := ""
		if p.Note != nil {
			oldNote = *p.Note
		}
		newNote := ""
		if v != nil {
			newNote = v.(string)
		}
		_ = h.clRepo.LogUpdateField(orgID, "party", id, "note", oldNote, newNote)
	}

	updated, _ := h.repo.FindPartyByID(id)
	platform.SuccessResponse(c, updated)
}

// ---------- 应收单 ----------

// ListReceivables 应收单列表。
// GET /api/receivables?partyId=&kind=&status=&page=
func (h *Handler) ListReceivables(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}

	var partyID *int64
	if pid := c.Query("partyId"); pid != "" {
		if id, err := strconv.ParseInt(pid, 10, 64); err == nil {
			partyID = &id
		}
	}
	kind := c.Query("kind")
	status := c.Query("status")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	items, total, err := h.repo.ListReceivables(orgID, partyID, kind, status, page, pageSize)
	if err != nil {
		h.internal(c, "查询应收单失败")
		return
	}
	if items == nil {
		items = []Receivable{}
	}
	platform.SuccessResponse(c, ReceivableListResponse{Items: items, Total: total})
}

// CreateReceivable 登记应收单。
// POST /api/receivables
func (h *Handler) CreateReceivable(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}

	var req CreateReceivableRequest
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
	req.Title = trimSpace(req.Title)
	if req.Title == "" {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "请填写应收事由",
		})
		return
	}

	party, err := h.repo.FindPartyByID(req.PartyID)
	if err != nil {
		h.internal(c, "服务暂时不可用")
		return
	}
	if party == nil || party.OrgID != orgID {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "PARTY_NOT_FOUND", Message: "往来对象不存在",
		})
		return
	}

	// 预设收款入账科目（可选）：须为本组织启用中的普通二级
	var incomeCatID *int64
	if req.IncomeCategoryID != nil {
		cat, err := h.catRepo.FindByID(*req.IncomeCategoryID)
		if err != nil {
			h.internal(c, "服务暂时不可用")
			return
		}
		if cat == nil || cat.OrgID != orgID || cat.Level != 2 || cat.Status != "active" || cat.Kind != "normal" {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "INVALID_INCOME_CATEGORY", Message: "收款入账科目不合法，需为本组织启用中的普通二级科目",
			})
			return
		}
		incomeCatID = req.IncomeCategoryID
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}
	rec := &Receivable{
		OrgID: orgID, PartyID: req.PartyID, RecvKind: req.RecvKind, Title: req.Title,
		AmountCents: req.AmountCents, IncomeCategoryID: incomeCatID, Note: note,
	}
	created, err := h.repo.CreateReceivable(rec)
	if err != nil {
		h.internal(c, "登记应收单失败")
		return
	}
	h.clRepo.LogCreate(orgID, "receivable", created.ID)
	platform.SuccessResponse(c, created)
}

// GetReceivableDetail 应收单详情（含核销记录）。
// GET /api/receivables/:id
func (h *Handler) GetReceivableDetail(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "应收单 ID 不合法",
		})
		return
	}

	detail, err := h.repo.GetReceivableDetail(orgID, id)
	if err != nil {
		h.internal(c, "服务暂时不可用")
		return
	}
	if detail == nil {
		platform.ErrResponse(c, http.StatusNotFound, &platform.AppError{
			Code: "RECEIVABLE_NOT_FOUND", Message: "应收单不存在",
		})
		return
	}
	platform.SuccessResponse(c, detail)
}

// ---------- 核销 ----------

// CreateReceipt 收款核销（cash 自动入银行收入；offset 关联支出流水抵销）。
// POST /api/receivables/:id/receipts
func (h *Handler) CreateReceipt(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}

	recID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "应收单 ID 不合法",
		})
		return
	}

	rec, err := h.repo.FindReceivableByID(recID)
	if err != nil {
		h.internal(c, "服务暂时不可用")
		return
	}
	if rec == nil || rec.OrgID != orgID {
		platform.ErrResponse(c, http.StatusNotFound, &platform.AppError{
			Code: "RECEIVABLE_NOT_FOUND", Message: "应收单不存在",
		})
		return
	}

	var req CreateReceiptRequest
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
	if !validDate(req.ReceiptDate) {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_DATE", Message: "日期格式不合法，应为 YYYY-MM-DD",
		})
		return
	}
	if rec.Status == "closed" {
		platform.ErrResponse(c, http.StatusConflict, &platform.AppError{
			Code: "RECEIVABLE_CLOSED", Message: "该应收单已结清，不能再核销",
		})
		return
	}

	// 超收预检（repo 事务内再兜底一次）
	paid, err := h.paidSum(orgID, recID)
	if err != nil {
		h.internal(c, "服务暂时不可用")
		return
	}
	if paid+req.AmountCents > rec.AmountCents {
		platform.ErrResponse(c, http.StatusConflict, &platform.AppError{
			Code: "RECEIPT_OVER_RECEIVABLE",
			Message: fmt.Sprintf("累计核销不能超过应收金额：已收 %.2f，应收 %.2f",
				float64(paid)/100, float64(rec.AmountCents)/100),
		})
		return
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	var outcome *CreateOutcome
	switch req.Method {
	case "cash":
		// 入账科目：本次请求显式指定优先（需合法）；未指定时用应收单预设；两者皆无则报错
		var catID int64
		switch {
		case req.CategoryID != nil:
			catID = *req.CategoryID
		case rec.IncomeCategoryID != nil:
			catID = *rec.IncomeCategoryID
		default:
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code:    "INCOME_CATEGORY_REQUIRED",
				Message: "现金收款需要收入入账科目（登记应收单时预设，或本次指定）",
			})
			return
		}
		okCat, err := h.validIncomeCategory(orgID, catID)
		if err != nil {
			h.internal(c, "服务暂时不可用")
			return
		}
		if !okCat {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "INVALID_INCOME_CATEGORY", Message: "收款入账科目不合法，需为本组织启用中的普通二级科目",
			})
			return
		}

		cashNote := note
		if cashNote == nil || *cashNote == "" {
			s := fmt.Sprintf("核销应收 #%d %s", rec.ID, rec.Title)
			cashNote = &s
		}
		outcome, err = h.repo.CreateCashReceipt(orgID, rec, req.AmountCents, req.ReceiptDate, catID, cashNote)
		if err != nil {
			if errors.Is(err, ErrOverReceivable) {
				platform.ErrResponse(c, http.StatusConflict, &platform.AppError{
					Code: "RECEIPT_OVER_RECEIVABLE", Message: "累计核销不能超过应收金额",
				})
				return
			}
			h.internal(c, "现金核销失败")
			return
		}
	case "offset":
		if req.TxnID == nil {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "OFFSET_TXN_REQUIRED", Message: "抵销核销需关联一条发放支出流水（txnId）",
			})
			return
		}
		okTxn, err := h.validOffsetTxn(orgID, *req.TxnID)
		if err != nil {
			h.internal(c, "服务暂时不可用")
			return
		}
		if !okTxn {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "OFFSET_TXN_INVALID", Message: "抵销流水不合法，需为本组织一笔正常状态的发放支出流水",
			})
			return
		}
		outcome, err = h.repo.CreateOffsetReceipt(orgID, rec, req.AmountCents, req.ReceiptDate, *req.TxnID, note)
		if err != nil {
			if errors.Is(err, ErrOverReceivable) {
				platform.ErrResponse(c, http.StatusConflict, &platform.AppError{
					Code: "RECEIPT_OVER_RECEIVABLE", Message: "累计核销不能超过应收金额",
				})
				return
			}
			h.internal(c, "抵销核销失败")
			return
		}
	default:
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "核销方式不合法（cash/offset）",
		})
		return
	}

	h.clRepo.LogCreate(orgID, "receipt", outcome.Receipt.ID)
	if outcome.TxnCreated != nil {
		_ = h.clRepo.LogCreate(orgID, "txn", *outcome.TxnCreated)
	}
	if outcome.ReceivableStatus != rec.Status {
		_ = h.clRepo.LogUpdateField(orgID, "receivable", rec.ID, "status", rec.Status, outcome.ReceivableStatus)
	}

	platform.SuccessResponse(c, outcome.Receipt)
}

// VoidReceipt 作废核销（cash 核销连带作废其银行收入流水，应收单退回未结清）。
// PUT /api/receipts/:id
func (h *Handler) VoidReceipt(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}

	receiptID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "核销记录 ID 不合法",
		})
		return
	}

	// 作废前快照（状态流转与留痕用）
	before, err := h.repo.FindReceiptByID(receiptID)
	if err != nil {
		h.internal(c, "服务暂时不可用")
		return
	}
	if before == nil || before.OrgID != orgID {
		platform.ErrResponse(c, http.StatusNotFound, &platform.AppError{
			Code: "RECEIPT_NOT_FOUND", Message: "核销记录不存在",
		})
		return
	}
	recBefore, err := h.repo.FindReceivableByID(before.ReceivableID)
	if err != nil {
		h.internal(c, "服务暂时不可用")
		return
	}

	outcome, err := h.repo.VoidReceipt(orgID, receiptID)
	if err != nil {
		h.internal(c, "作废核销失败")
		return
	}
	if outcome == nil {
		platform.ErrResponse(c, http.StatusNotFound, &platform.AppError{
			Code: "RECEIPT_NOT_FOUND", Message: "核销记录不存在",
		})
		return
	}

	if outcome.Receipt.Status == "voided" && outcome.ReceivableStatus == "" {
		// 已作废，幂等返回（不再重复留痕）
		platform.SuccessResponse(c, outcome.Receipt)
		return
	}

	_ = h.clRepo.LogChangeVoid(orgID, "receipt", receiptID, "void", before.Status, outcome.Receipt.Status)
	if outcome.TxnVoided != nil {
		_ = h.clRepo.LogChangeVoid(orgID, "txn", *outcome.TxnVoided, "void", "normal", "voided")
	}

	// 应收单状态变化留痕（closed→open 等）
	if recBefore != nil && recBefore.OrgID == orgID && recBefore.Status != outcome.ReceivableStatus && outcome.ReceivableStatus != "" {
		_ = h.clRepo.LogUpdateField(orgID, "receivable", before.ReceivableID, "status", recBefore.Status, outcome.ReceivableStatus)
	}

	platform.SuccessResponse(c, outcome.Receipt)
}

// ---------- 校验助手 ----------

// paidSum 统计应收单当前已核销金额。
func (h *Handler) paidSum(orgID, receivableID int64) (int64, error) {
	var paid int64
	if err := h.repo.db.QueryRow(
		`SELECT COALESCE(SUM(amount_cents), 0) FROM receipt WHERE org_id = ? AND receivable_id = ? AND status = 'normal'`,
		orgID, receivableID,
	).Scan(&paid); err != nil {
		return 0, fmt.Errorf("统计已核销金额失败: %w", err)
	}
	return paid, nil
}

// validIncomeCategory 校验现金核销入账科目。
func (h *Handler) validIncomeCategory(orgID, catID int64) (bool, error) {
	cat, err := h.catRepo.FindByID(catID)
	if err != nil {
		return false, err
	}
	return cat != nil && cat.OrgID == orgID && cat.Level == 2 && cat.Status == "active" && cat.Kind == "normal", nil
}

// validOffsetTxn 校验抵销所关联的支出流水（发放应付款）。
func (h *Handler) validOffsetTxn(orgID, txnID int64) (bool, error) {
	var org int64
	var direction, status string
	err := h.repo.db.QueryRow(
		`SELECT org_id, direction, status FROM txn WHERE id = ?`, txnID,
	).Scan(&org, &direction, &status)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("查询流水失败: %w", err)
	}
	return org == orgID && direction == "expense" && status == "normal", nil
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
