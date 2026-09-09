package receivable

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
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
	r.POST("/api/receivables/batch", h.BatchAccrue)
	r.GET("/api/receivables/:id", h.GetReceivableDetail)
	r.PUT("/api/receivables/:id/void", h.VoidReceivable)
	r.POST("/api/receivables/:id/receipts", h.CreateReceipt)
	r.PUT("/api/receipts/:id", h.VoidReceipt)

	r.GET("/api/recv-standards", h.ListStandards)
	r.GET("/api/recv-standards/preview", h.PreviewAccrue)
	r.POST("/api/recv-standards", h.SaveStandard)
	r.PUT("/api/recv-standards/:id", h.ToggleStandard)
	r.POST("/api/recv-standards/accrue", h.AccrueByStandards)

	r.GET("/api/parties/:id/allocations", h.ListAllocations)
	r.POST("/api/parties/:id/allocations", h.CreateAllocation)
	r.DELETE("/api/allocations/:id", h.DeleteAllocation)

	r.POST("/api/receipts", h.CreateReceiptByReceivable)
	r.POST("/api/party-collect", h.CollectByParty)

	r.GET("/api/distributions-532", h.ListDistributions532)
	r.POST("/api/distributions-532", h.SaveDistribution532)
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

// ---------- 往来单位 ----------

// ListParties 往来单位列表（含欠款合计）。
// GET /api/parties?keyword=
func (h *Handler) ListParties(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	keyword := c.Query("keyword")

	items, err := h.repo.ListParties(orgID, keyword)
	if err != nil {
		h.internal(c, "查询往来单位失败")
		return
	}
	if items == nil {
		items = []Party{}
	}
	platform.SuccessResponse(c, items)
}

// CreateParty 新建往来单位。
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
			Code: "INVALID_REQUEST", Message: "请填写单位名称",
		})
		return
	}
	ptype := trimSpace(req.Type)
	if ptype == "" && len(req.Types) > 0 {
		ptype = trimSpace(req.Types[0]) // types 优先级高于 type（v0.7 多选数组）
	}
	if ptype == "" {
		ptype = "flow"
	}
	if !validPartyType(ptype) {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "单位类型不合法",
		})
		return
	}
	if req.AreaMu < 0 {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "流转面积不能为负数",
		})
		return
	}

	// 重名校验：同类型精确重名直接拒绝；模糊相近重名给出候选，交由用户改名称
	if exact, dupes := h.repo.FindDuplicate(orgID, req.Name, ptype); exact {
		platform.ErrResponse(c, http.StatusConflict, &platform.AppError{
			Code: "DUPLICATE_NAME", Message: "该类型下已存在同名单位",
		})
		return
	} else if len(dupes) > 0 {
		platform.ErrResponse(c, http.StatusConflict, &platform.AppError{
			Code: "DUPLICATE_NAME", Message: "存在相近重名单位，请修改名称后重试", Details: dupes,
		})
		return
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}
	// 费用/年收益解析：显式总额优先；未提供时按原始量（亩数×每单价 / 本金×收益率）兜底计算。
	// 仅计算为正才写入（0 不建单位同名费用、不写标准）。
	landFee := req.ExpectedLandFeeCents
	if landFee <= 0 && req.LandMu > 0 && req.LandFeePerMuCents > 0 {
		landFee = int64(math.Round(req.LandMu * float64(req.LandFeePerMuCents)))
	}
	mgmtFee := req.ExpectedMgmtFeeCents
	if mgmtFee <= 0 && req.LandMu > 0 && req.MgmtFeePerMuCents > 0 {
		mgmtFee = int64(math.Round(req.LandMu * float64(req.MgmtFeePerMuCents)))
	}
	retFee := req.ExpectedReturnCents
	if retFee <= 0 && req.InvestAmountCents > 0 && req.ReturnRateBps > 0 {
		retFee = req.InvestAmountCents * int64(req.ReturnRateBps) / 10000
	}
	p := &Party{
		OrgID: orgID, Name: req.Name, Type: ptype,
		ContactPhone: trimSpace(req.ContactPhone), AreaMu: req.AreaMu, Note: note,
		InvestAmountCents:    req.InvestAmountCents,
		ReturnRateBps:        req.ReturnRateBps,
		ExpectedReturnCents:  retFee,
		LandMu:               req.LandMu,
		LandFeePerMuCents:    req.LandFeePerMuCents,
		ExpectedLandFeeCents: landFee,
		MgmtFeePerMuCents:    req.MgmtFeePerMuCents,
		ExpectedMgmtFeeCents: mgmtFee,
	}
	created, err := h.repo.CreateParty(p)
	if err != nil {
		h.internal(c, "新建往来单位失败")
		return
	}
	// 新建单位带正费用/年收益 → 同步写入计提标准（供年度计提预览与结转）
	var landPtr, mgmtPtr, retPtr *int64
	if landFee > 0 {
		v := landFee
		landPtr = &v
	}
	if mgmtFee > 0 {
		v := mgmtFee
		mgmtPtr = &v
	}
	if retFee > 0 {
		v := retFee
		retPtr = &v
	}
	if err := h.applyFeeStandards(orgID, created.ID, ptype, landPtr, mgmtPtr, retPtr); err != nil {
		h.internal(c, "同步计提标准失败")
		return
	}
	h.clRepo.LogCreate(orgID, "party", created.ID)
	platform.SuccessResponse(c, created)
}

// applyFeeStandards 把单位费用/年收益同步到计提标准，pointer 语义：
//   - nil      → 该项本次未提供，不处理
//   - 值 > 0   → upsert 标准并启用（flow→rent/service；invest/reinvest→dividend）
//   - 值 == 0  → 停用对应标准（费用清零时避免标准残留）
func (h *Handler) applyFeeStandards(orgID, partyID int64, ptype string, land, mgmt, ret *int64) error {
	type target struct {
		kind string
		v    *int64
	}
	var list []target
	if ptype == "flow" {
		list = append(list, target{"rent", land}, target{"service", mgmt})
	}
	if ptype == "invest" || ptype == "reinvest" {
		list = append(list, target{"dividend", ret})
	}
	for _, t := range list {
		if t.v == nil {
			continue
		}
		if *t.v > 0 {
			if _, err := h.repo.UpsertStandard(orgID, &AccrualStandard{
				PartyID: partyID, RecvKind: t.kind, AmountCents: *t.v, Active: true,
			}); err != nil {
				return err
			}
			continue
		}
		if err := h.repo.SetPartyStandardActive(orgID, partyID, t.kind, false); err != nil {
			return err
		}
	}
	return nil
}

// UpdateParty 更新往来单位（名称/备注）。
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
			Code: "INVALID_REQUEST", Message: "往来单位 ID 不合法",
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
			Code: "PARTY_NOT_FOUND", Message: "往来单位不存在",
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
	// 名称/类型不可编辑：仅允许同名同值（忽略），否则拒绝
	if req.Name != nil && trimSpace(*req.Name) != p.Name {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "NAME_IMMUTABLE", Message: "单位名称不可编辑",
		})
		return
	}
	if req.Type != nil && trimSpace(*req.Type) != p.Type {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "TYPE_IMMUTABLE", Message: "单位类型不可编辑",
		})
		return
	}
	if req.ContactPhone != nil {
		updates["contact_phone"] = trimSpace(*req.ContactPhone)
	}
	if req.AreaMu != nil {
		if *req.AreaMu < 0 {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "INVALID_REQUEST", Message: "流转面积不能为负数",
			})
			return
		}
		updates["area_mu"] = *req.AreaMu
	}
	if req.Note != nil {
		if *req.Note == "" {
			updates["note"] = nil
		} else {
			updates["note"] = *req.Note
		}
	}
	// 年度数据（v0.9+ 已落库）
	if req.InvestAmountCents != nil {
		updates["invest_amount_cents"] = *req.InvestAmountCents
	}
	if req.ReturnRateBps != nil {
		updates["return_rate_bps"] = *req.ReturnRateBps
	}
	if req.ExpectedReturnCents != nil {
		updates["expected_return_cents"] = *req.ExpectedReturnCents
	}
	if req.LandMu != nil {
		updates["land_mu"] = *req.LandMu
	}
	if req.LandFeePerMuCents != nil {
		updates["land_fee_per_mu_cents"] = *req.LandFeePerMuCents
	}
	if req.ExpectedLandFeeCents != nil {
		updates["expected_land_fee_cents"] = *req.ExpectedLandFeeCents
	}
	if req.MgmtFeePerMuCents != nil {
		updates["mgmt_fee_per_mu_cents"] = *req.MgmtFeePerMuCents
	}
	if req.ExpectedMgmtFeeCents != nil {
		updates["expected_mgmt_fee_cents"] = *req.ExpectedMgmtFeeCents
	}

	// ---- 费用/年收益 → 计提标准解析：显式总额优先；未显式时按原始量（亩×每单价 / 本金×收益率）兜底计算 ----
	var landPtr, mgmtPtr, retPtr *int64
	if req.ExpectedLandFeeCents != nil {
		v := *req.ExpectedLandFeeCents
		landPtr = &v
	} else if req.LandMu != nil || req.LandFeePerMuCents != nil {
		mu := p.LandMu
		if req.LandMu != nil {
			mu = *req.LandMu
		}
		fee := p.LandFeePerMuCents
		if req.LandFeePerMuCents != nil {
			fee = *req.LandFeePerMuCents
		}
		v := int64(math.Round(mu * float64(fee)))
		updates["expected_land_fee_cents"] = v
		landPtr = &v
	}
	if req.ExpectedMgmtFeeCents != nil {
		v := *req.ExpectedMgmtFeeCents
		mgmtPtr = &v
	} else if req.MgmtFeePerMuCents != nil {
		mu := p.LandMu
		if req.LandMu != nil {
			mu = *req.LandMu
		}
		fee := p.MgmtFeePerMuCents
		if req.MgmtFeePerMuCents != nil {
			fee = *req.MgmtFeePerMuCents
		}
		v := int64(math.Round(mu * float64(fee)))
		updates["expected_mgmt_fee_cents"] = v
		mgmtPtr = &v
	}
	if req.ExpectedReturnCents != nil {
		v := *req.ExpectedReturnCents
		retPtr = &v
	} else if req.InvestAmountCents != nil || req.ReturnRateBps != nil {
		amt := p.InvestAmountCents
		if req.InvestAmountCents != nil {
			amt = *req.InvestAmountCents
		}
		bps := p.ReturnRateBps
		if req.ReturnRateBps != nil {
			bps = *req.ReturnRateBps
		}
		v := amt * int64(bps) / 10000
		updates["expected_return_cents"] = v
		retPtr = &v
	}

	if err := h.repo.UpdateParty(id, orgID, updates); err != nil {
		h.internal(c, "更新往来单位失败")
		return
	}

	// 同步计提标准：>0 启用/更新；0 停用对应类别
	if err := h.applyFeeStandards(orgID, id, p.Type, landPtr, mgmtPtr, retPtr); err != nil {
		h.internal(c, "同步计提标准失败")
		return
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
	year, _ := strconv.Atoi(c.Query("year"))
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

	items, total, err := h.repo.ListReceivables(orgID, partyID, year, kind, status, page, pageSize)
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
		if cat == nil || cat.OrgID != orgID || cat.Level != 2 || cat.Status != "active" || cat.Kind != "equity" {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "INVALID_INCOME_CATEGORY", Message: "收款入账科目不合法，需为本组织启用中的普通二级科目",
			})
			return
		}
		incomeCatID = req.IncomeCategoryID
	}

	year := req.RecvYear
	if year == 0 {
		year = time.Now().Year()
	}

	// 年度性费用（流转费/管理费）同一单位同一类别同一年度只允许一张应收单，
	// 与批量计提语义一致；防止手工补录历年欠款时重复建单。
	if req.RecvKind == "rent" || req.RecvKind == "service" {
		dup, err := h.repo.ReceivableExists(orgID, req.PartyID, year, req.RecvKind)
		if err != nil {
			h.internal(c, "查重失败")
			return
		}
		if dup {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code:    "DUPLICATE_RECEIVABLE",
				Message: fmt.Sprintf("该单位 %d 年度已登记过同类应收，请勿重复登记（如需改金额请作废原单后重录）", year),
			})
			return
		}
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}
	rec := &Receivable{
		OrgID: orgID, PartyID: req.PartyID, RecvYear: year, RecvKind: req.RecvKind, Title: req.Title,
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

// BatchAccrue 批量计提应收（同年度一批：单位+类别+金额）。
// POST /api/receivables/batch
func (h *Handler) BatchAccrue(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	var req BatchAccrueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{Code: "INVALID_REQUEST", Message: "参数不合法"})
		return
	}
	year := req.RecvYear
	if year == 0 {
		year = time.Now().Year()
	}
	for _, it := range req.Items {
		if it.AmountCents <= 0 {
			platform.ErrResponse(c, http.StatusBadRequest, platform.ErrInvalidAmount)
			return
		}
		party, err := h.repo.FindPartyByID(it.PartyID)
		if err != nil {
			h.internal(c, "服务暂时不可用")
			return
		}
		if party == nil || party.OrgID != orgID {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{Code: "PARTY_NOT_FOUND", Message: "往来单位不存在"})
			return
		}
	}
	result, err := h.repo.BatchCreateReceivables(orgID, year, trimSpace(req.Title), req.Items)
	if err != nil {
		h.internal(c, "批量计提失败")
		return
	}
	_ = h.clRepo.LogCreate(orgID, "receivable_batch", int64(year))
	platform.SuccessResponse(c, result)
}

// ListStandards 计提标准列表。
// GET /api/recv-standards?kind=rent
func (h *Handler) ListStandards(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	items, err := h.repo.ListStandards(orgID, c.Query("kind"))
	if err != nil {
		h.internal(c, "查询计提标准失败")
		return
	}
	if items == nil {
		items = []AccrualStandard{}
	}
	platform.SuccessResponse(c, items)
}

// PreviewAccrue 预览年度计提：自动按单位基本信息带出建议金额（投资→投资收益/流转→流转费+管理费）。
// GET /api/recv-standards/preview?year=
func (h *Handler) PreviewAccrue(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	year, _ := strconv.Atoi(c.Query("year"))
	if year == 0 {
		year = time.Now().Year()
	}
	result, err := h.repo.PreviewAccrueAuto(orgID, year)
	if err != nil {
		h.internal(c, "生成年度计提预览失败")
		return
	}
	platform.SuccessResponse(c, result)
}

// SaveStandard 保存计提标准（同单位+类别更新）。
// POST /api/recv-standards
func (h *Handler) SaveStandard(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	var req AccrualStandardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{Code: "INVALID_REQUEST", Message: "参数不合法"})
		return
	}
	if req.AmountCents <= 0 {
		platform.ErrResponse(c, http.StatusBadRequest, platform.ErrInvalidAmount)
		return
	}
	party, err := h.repo.FindPartyByID(req.PartyID)
	if err != nil {
		h.internal(c, "服务暂时不可用")
		return
	}
	if party == nil || party.OrgID != orgID {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{Code: "PARTY_NOT_FOUND", Message: "往来单位不存在"})
		return
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	saved, err := h.repo.UpsertStandard(orgID, &AccrualStandard{
		PartyID: req.PartyID, RecvKind: req.RecvKind, AmountCents: req.AmountCents, Active: active,
	})
	if err != nil {
		h.internal(c, "保存计提标准失败")
		return
	}
	platform.SuccessResponse(c, saved)
}

// ToggleStandard 启停计提标准。
// PUT /api/recv-standards/:id
func (h *Handler) ToggleStandard(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{Code: "INVALID_REQUEST", Message: "标准 ID 不合法"})
		return
	}
	var req struct {
		Active *bool `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Active == nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{Code: "INVALID_REQUEST", Message: "参数不合法"})
		return
	}
	if err := h.repo.SetStandardActive(id, orgID, *req.Active); err != nil {
		h.internal(c, "更新计提标准失败")
		return
	}
	platform.SuccessResponse(c, gin.H{"ok": true})
}

// AccrueByStandards 按启用标准一键结转年度应收。
// POST /api/recv-standards/accrue  body {year, kind, title}
func (h *Handler) AccrueByStandards(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	var req struct {
		Year  int    `json:"year"`
		Kind  string `json:"kind" binding:"required,oneof=rent dividend service other"`
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{Code: "INVALID_REQUEST", Message: "参数不合法"})
		return
	}
	year := req.Year
	if year == 0 {
		year = time.Now().Year()
	}
	title := trimSpace(req.Title)
	if title == "" {
		switch req.Kind {
		case "rent":
			title = fmt.Sprintf("%d年度土地流转费", year)
		case "service":
			title = fmt.Sprintf("%d年度管理费", year)
		case "dividend":
			title = fmt.Sprintf("%d年度投资收益", year)
		default:
			title = fmt.Sprintf("%d年度计提", year)
		}
	}
	result, err := h.repo.AccrueFromStandards(orgID, year, req.Kind, title)
	if err != nil {
		h.internal(c, "一键结转失败")
		return
	}
	_ = h.clRepo.LogCreate(orgID, "receivable_batch", int64(year))
	platform.SuccessResponse(c, result)
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

// VoidReceivable 作废未收款应收单（允许“作废重结”；已核销的不可作废）。
// PUT /api/receivables/:id/void
func (h *Handler) VoidReceivable(c *gin.Context) {
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
	rec := detail.Receivable
	if rec.Status != "open" || rec.PaidCents > 0 {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "RECEIVABLE_HAS_PAYMENT", Message: "该应收已有核销或已结清，不能作废",
		})
		return
	}
	if err := h.repo.DeleteOpenReceivable(orgID, id); err != nil {
		h.internal(c, "作废应收单失败")
		return
	}
	_ = h.clRepo.LogUpdateField(orgID, "receivable", id, "status", "open", "voided")
	platform.SuccessResponse(c, gin.H{"ok": true})
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
		// 入账科目：本次请求显式指定优先（需合法）；未指定时用应收单预设；
		// 两者皆无则按应收类型自动定位/创建该单位同名收入二级科目入账。
		var catID int64
		autoResolved := false
		switch {
		case req.CategoryID != nil:
			catID = *req.CategoryID
		case rec.IncomeCategoryID != nil:
			catID = *rec.IncomeCategoryID
		default:
			cid, rerr := h.repo.ResolveIncomeCategory(orgID, rec.PartyID, rec.RecvKind)
			if rerr != nil {
				h.internal(c, "服务暂时不可用")
				return
			}
			if cid == 0 {
				platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
					Code:    "INCOME_CATEGORY_REQUIRED",
					Message: "现金收款需要收入入账科目（登记应收单时预设，或本次指定）",
				})
				return
			}
			catID = cid
			autoResolved = true
		}
		okCat, err := h.validIncomeCategory(orgID, catID)
		if err != nil {
			h.internal(c, "服务暂时不可用")
			return
		}
		if !okCat && !autoResolved {
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

// ---------- 再投资去向 ----------

// ListAllocations 某往来单位的再投资去向列表。
// GET /api/parties/:id/allocations
func (h *Handler) ListAllocations(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	partyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "往来单位 ID 不合法",
		})
		return
	}
	pty, err := h.repo.FindPartyByID(partyID)
	if err != nil {
		h.internal(c, "服务暂时不可用")
		return
	}
	if pty == nil || pty.OrgID != orgID {
		platform.ErrResponse(c, http.StatusNotFound, &platform.AppError{
			Code: "PARTY_NOT_FOUND", Message: "往来单位不存在",
		})
		return
	}
	items, err := h.repo.ListAllocations(orgID, partyID)
	if err != nil {
		h.internal(c, "查询再投资去向失败")
		return
	}
	if items == nil {
		items = []ReinvestAllocation{}
	}
	platform.SuccessResponse(c, items)
}

// CreateAllocation 新建再投资去向。
// POST /api/parties/:id/allocations  body {targetName, targetPartyId?, amountCents, notes?}
func (h *Handler) CreateAllocation(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	partyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "往来单位 ID 不合法",
		})
		return
	}
	pty, err := h.repo.FindPartyByID(partyID)
	if err != nil {
		h.internal(c, "服务暂时不可用")
		return
	}
	if pty == nil || pty.OrgID != orgID {
		platform.ErrResponse(c, http.StatusNotFound, &platform.AppError{
			Code: "PARTY_NOT_FOUND", Message: "往来单位不存在",
		})
		return
	}

	var req ReinvestAllocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "参数不合法",
		})
		return
	}
	if trimSpace(req.TargetName) == "" {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "去向单位不能为空",
		})
		return
	}
	if req.AmountCents <= 0 {
		platform.ErrResponse(c, http.StatusBadRequest, platform.ErrInvalidAmount)
		return
	}
	var notes *string
	if trimSpace(req.Notes) != "" {
		s := trimSpace(req.Notes)
		notes = &s
	}
	a := &ReinvestAllocation{
		PartyID:       partyID,
		TargetName:    trimSpace(req.TargetName),
		TargetPartyID: req.TargetPartyID,
		AmountCents:   req.AmountCents,
		Notes:         notes,
	}
	created, err := h.repo.CreateAllocation(orgID, a)
	if err != nil {
		h.internal(c, "新建再投资去向失败")
		return
	}
	_ = h.clRepo.LogCreate(orgID, "reinvest_allocation", created.ID)
	platform.SuccessResponse(c, created)
}

// DeleteAllocation 删除再投资去向。
// DELETE /api/allocations/:id
func (h *Handler) DeleteAllocation(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "再投资去向 ID 不合法",
		})
		return
	}
	hit, err := h.repo.DeleteAllocation(id, orgID)
	if err != nil {
		h.internal(c, "删除再投资去向失败")
		return
	}
	if !hit {
		platform.ErrResponse(c, http.StatusNotFound, &platform.AppError{
			Code: "ALLOCATION_NOT_FOUND", Message: "再投资去向不存在",
		})
		return
	}
	_ = h.clRepo.LogChangeVoid(orgID, "reinvest_allocation", id, "delete", "normal", "deleted")
	platform.SuccessResponse(c, gin.H{"ok": true})
}

// ---------- 全局收缴核销 ----------

// CreateReceiptByReceivable 全局收缴核销：按 receivableId + amountCents 直接收款。
// POST /api/receipts  body {receivableId, amountCents}
// 前端不收方式/日期/科目参数 → 恒为 cash、日期取今日、入账科目取应收单预设 incomeCategoryId。
func (h *Handler) CreateReceiptByReceivable(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}

	var req struct {
		ReceivableID int64 `json:"receivableId"`
		AmountCents  int64 `json:"amountCents"`
	}
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

	rec, err := h.repo.FindReceivableByID(req.ReceivableID)
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
	if rec.Status == "closed" {
		platform.ErrResponse(c, http.StatusConflict, &platform.AppError{
			Code: "RECEIVABLE_CLOSED", Message: "该应收单已结清，不能再核销",
		})
		return
	}

	paid, err := h.paidSum(orgID, req.ReceivableID)
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

	var catID int64
	if rec.IncomeCategoryID != nil {
		catID = *rec.IncomeCategoryID
	} else {
		cid, rerr := h.repo.ResolveIncomeCategory(orgID, rec.PartyID, rec.RecvKind)
		if rerr != nil {
			h.internal(c, "服务暂时不可用")
			return
		}
		if cid == 0 {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code:    "INCOME_CATEGORY_REQUIRED",
				Message: "现金收款需要收入入账科目（登记应收单时预设，或本次指定）",
			})
			return
		}
		catID = cid
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

	date := time.Now().Format(dateLayout)
	note := fmt.Sprintf("核销应收 #%d %s", rec.ID, rec.Title)
	outcome, err := h.repo.CreateCashReceipt(orgID, rec, req.AmountCents, date, catID, &note)
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

	_ = h.clRepo.LogCreate(orgID, "receipt", outcome.Receipt.ID)
	if outcome.TxnCreated != nil {
		_ = h.clRepo.LogCreate(orgID, "txn", *outcome.TxnCreated)
	}
	if outcome.ReceivableStatus != rec.Status {
		_ = h.clRepo.LogUpdateField(orgID, "receivable", rec.ID, "status", rec.Status, outcome.ReceivableStatus)
	}
	platform.SuccessResponse(c, outcome.Receipt)
}

// CollectByParty 整额跨单收款：一笔现金收款自动按最早年度优先摊分核销该单位该类
// 多张未结清应收单。金额超过待收合计会拒绝。
// POST /api/party-collect  body {partyId, recvKind, amountCents, receiptDate?, note?}
func (h *Handler) CollectByParty(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}

	var req struct {
		PartyID     int64  `json:"partyId" binding:"required"`
		RecvKind    string `json:"recvKind" binding:"required,oneof=rent dividend service reinvest_dividend"`
		AmountCents int64  `json:"amountCents"`
		ReceiptDate string `json:"receiptDate"`
		Note        string `json:"note"`
	}
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

	date := trimSpace(req.ReceiptDate)
	if date == "" {
		date = time.Now().Format(dateLayout)
	} else if !validDate(date) {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_DATE", Message: "收款日期格式应为 YYYY-MM-DD",
		})
		return
	}

	// 现金入账科目：自动定位/创建该单位同名收入二级（与单张核销一致）
	cid, err := h.repo.ResolveIncomeCategory(orgID, req.PartyID, req.RecvKind)
	if err != nil {
		h.internal(c, "服务暂时不可用")
		return
	}
	if cid == 0 {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INCOME_CATEGORY_REQUIRED", Message: "无法确定收款入账科目，请先为该单位建立同名收入科目",
		})
		return
	}

	var note *string
	if trimSpace(req.Note) != "" {
		s := trimSpace(req.Note)
		note = &s
	}
	outcome, err := h.repo.CollectOpenAcrossYears(orgID, req.PartyID, req.RecvKind, req.AmountCents, date, cid, note)
	if err != nil {
		var overTotal *ErrCollectOverTotal
		switch {
		case errors.Is(err, ErrNoOpenReceivable):
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "COLLECT_NO_OPEN", Message: "该单位该类别暂无可核销的应收欠款",
			})
			return
		case errors.As(err, &overTotal):
			platform.ErrResponse(c, http.StatusConflict, &platform.AppError{
				Code:    "COLLECT_OVER_TOTAL",
				Message: fmt.Sprintf("金额超过该单位待收合计 %s 元", fenText(overTotal.Total)),
			})
			return
		default:
			h.internal(c, "整额核销失败")
			return
		}
	}

	for _, it := range outcome.Items {
		_ = h.clRepo.LogCreate(orgID, "receipt", it.ReceiptID)
		if it.TxnID != nil {
			_ = h.clRepo.LogCreate(orgID, "txn", *it.TxnID)
		}
	}
	_ = h.clRepo.LogCreate(orgID, "party_collect", req.PartyID)
	platform.SuccessResponse(c, outcome)
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
	return cat != nil && cat.OrgID == orgID && cat.Level == 2 && cat.Status == "active" && cat.Kind == "equity", nil
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

// ---------- 532分配 ----------

// ListDistributions532 查询 532 分配方案。
// GET /api/distributions-532?year=N → 单年方案（无则 data=null）
// GET /api/distributions-532 → 全部年份方案数组
func (h *Handler) ListDistributions532(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	if q := c.Query("year"); q != "" {
		year, err := strconv.Atoi(q)
		if err != nil {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "INVALID_REQUEST", Message: "年度参数不合法",
			})
			return
		}
		d, err := h.repo.GetDistribution532ByYear(orgID, year)
		if err != nil {
			h.internal(c, "服务暂时不可用")
			return
		}
		if d == nil {
			platform.SuccessResponse(c, nil) // 与前端 loadDist 的 null 期望一致
			return
		}
		platform.SuccessResponse(c, d)
		return
	}

	items, err := h.repo.ListDistributions532(orgID)
	if err != nil {
		h.internal(c, "服务暂时不可用")
		return
	}
	if items == nil {
		items = []Distribution532{}
	}
	platform.SuccessResponse(c, items)
}

// SaveDistribution532 保存/更新某年 532 分配方案。
// POST /api/distributions-532
func (h *Handler) SaveDistribution532(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	var req SaveDistribution532Request
	if err := c.ShouldBindJSON(&req); err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "参数不合法",
		})
		return
	}
	if req.Year <= 0 {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "年度不合法",
		})
		return
	}
	if req.ReinvestCents < 0 || req.DividendCents < 0 || req.WelfareCents < 0 || req.TotalIncomeCents < 0 {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "金额不能为负数",
		})
		return
	}

	d := &Distribution532{
		OrgID: orgID, Year: int(req.Year),
		TotalIncomeCents: req.TotalIncomeCents,
		ReinvestCents:    req.ReinvestCents,
		DividendCents:    req.DividendCents,
		WelfareCents:     req.WelfareCents,
	}
	saved, err := h.repo.UpsertDistribution532(orgID, d)
	if err != nil {
		h.internal(c, "服务暂时不可用")
		return
	}
	h.clRepo.LogCreate(orgID, "distribution_532", saved.ID)
	platform.SuccessResponse(c, saved)
}
