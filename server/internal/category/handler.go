package category

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"jititaizhang/server/internal/auth"
	"jititaizhang/server/internal/changelog"
	"jititaizhang/server/internal/platform"
)

// Handler 处理科目相关 HTTP 请求。
type Handler struct {
	repo *Repo
	clog *changelog.Repo
}

// NewHandler 创建 Handler。
func NewHandler(db *sql.DB) *Handler {
	return &Handler{repo: NewRepo(db), clog: changelog.NewRepo(db)}
}

// Register 挂载路由（与 auth.Handler.Register 同风格，供 main 统一接线）。
func (h *Handler) Register(r gin.IRouter) {
	r.GET("/api/categories", h.ListCategories)
	r.POST("/api/categories", h.CreateCategory)
	r.PUT("/api/categories/:id", h.UpdateCategory)
	r.DELETE("/api/categories/:id", h.DeleteCategory)
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

// buildTree 将扁平科目列表构建为树结构，并计算余额。
func (h *Handler) buildTree(cats []*Category) []*Category {
	// 按 parent_id 分组
	children := make(map[int64][]*Category)
	for _, c := range cats {
		if c.Level == 2 && c.ParentID != nil {
			children[*c.ParentID] = append(children[*c.ParentID], c)
		}
	}

	var roots []*Category
	for _, c := range cats {
		if c.Level == 1 {
			c.Children = children[c.ID]
			roots = append(roots, c)
		}
	}

	// 计算每个二级科目的余额和流水数，以及一级科目余额（子科目之和）。
	// 普通科目走 CalcBalance（收支+转账），资产科目走 AssetBalance（投资−收回，D10）。
	for _, root := range roots {
		var l1Balance int64
		for _, child := range root.Children {
			var bal int64
			if child.Kind == "asset" {
				bal, _ = h.repo.AssetBalance(child.ID)
			} else {
				bal, _ = h.repo.CalcBalance(child.ID)
			}
			child.BalanceCents = &bal
			l1Balance += bal
			count, _ := h.repo.CountTransactions(child.ID)
			child.TxnCount = &count
		}
		root.BalanceCents = &l1Balance
	}

	return roots
}

// ListCategories 返回科目树（仅当前组织）。
// GET /api/categories
func (h *Handler) ListCategories(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}
	cats, err := h.repo.FindAll(orgID)
	if err != nil {
		h.internal(c, "查询科目失败")
		return
	}
	if cats == nil {
		cats = []*Category{}
	}

	tree := h.buildTree(cats)
	if tree == nil {
		tree = []*Category{}
	}
	platform.SuccessResponse(c, tree)
}

// CreateCategory 新建科目。
// POST /api/categories
func (h *Handler) CreateCategory(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}

	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "参数不合法: " + err.Error(),
		})
		return
	}

	kind := "normal"
	if req.Kind != nil {
		kind = *req.Kind
	}

	// 校验
	if req.Level == 2 && req.ParentID == nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "二级科目必须指定所属一级科目",
		})
		return
	}
	if req.Level == 1 {
		req.ParentID = nil
	} else {
		// 二级科目：父科目必须存在、为一级、且属于当前组织
		parent, err := h.repo.FindByID(*req.ParentID)
		if err != nil {
			h.internal(c, "服务暂时不可用")
			return
		}
		if parent == nil || parent.Level != 1 {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "CATEGORY_PARENT_INVALID", Message: "父科目不存在或不是一级科目",
			})
			return
		}
		if parent.OrgID != orgID {
			platform.ErrResponse(c, http.StatusNotFound, platform.ErrCategoryNotFound)
			return
		}
	}

	// 资产型校验（D10）：仅二级、仅余粮、不勾稽、无期初
	if kind == "asset" {
		if req.Level != 2 {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "ASSET_LEVEL", Message: "资产型科目只能是二级科目（资产金额挂在其一级分组下）",
			})
			return
		}
		if req.BalanceType != "residual" {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "ASSET_BALANCE_TYPE", Message: "资产型科目余额类型固定为「余粮型（存量）」",
			})
			return
		}
		if req.OpeningBalanceCents != 0 {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "ASSET_NO_OPENING", Message: "资产型科目没有期初余额（余额由资金划转产生）",
			})
			return
		}
		if req.IncludeInReconciliation != nil && *req.IncludeInReconciliation {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "ASSET_NO_RECONCILE", Message: "资产型科目不参与资金勾稽",
			})
			return
		}
	}

	// 检查重名
	dup, err := h.repo.IsNameDup(orgID, req.Name, req.ParentID, 0)
	if err != nil {
		h.internal(c, "服务暂时不可用")
		return
	}
	if dup {
		platform.ErrResponse(c, http.StatusConflict, platform.ErrCategoryNameDup)
		return
	}

	// 检查花费型 + 勾稽
	includeInc := false
	if req.IncludeInReconciliation != nil {
		includeInc = *req.IncludeInReconciliation
	}
	if includeInc && req.BalanceType == "spending" {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code:    "INVALID_REQUEST",
			Message: "只有余粮型科目能参与资金勾稽",
		})
		return
	}

	// 一级科目是分组容器：余额 = 子科目之和（见 buildTree），不支持期初余额与参与勾稽
	if req.Level == 1 {
		if req.OpeningBalanceCents != 0 {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "LEVEL1_NO_OPENING", Message: "一级科目为分组容器，不支持设置期初余额",
			})
			return
		}
		if includeInc {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "LEVEL1_NO_RECONCILE", Message: "一级科目为分组容器，不支持参与资金勾稽",
			})
			return
		}
	}

	cat := &Category{
		OrgID:                   orgID,
		Name:                    req.Name,
		Level:                   req.Level,
		ParentID:                req.ParentID,
		Status:                  "active",
		BalanceType:             req.BalanceType,
		Kind:                    kind,
		OpeningBalanceCents:     req.OpeningBalanceCents,
		IncludeInReconciliation: includeInc,
		SortOrder:               req.SortOrder,
	}

	created, err := h.repo.Create(cat)
	if err != nil {
		h.internal(c, "创建科目失败")
		return
	}

	platform.SuccessResponse(c, created)
}

// UpdateCategory 更新科目（重命名 / 停用 / 启用 / 改期初余额）。
// PUT /api/categories/:id
func (h *Handler) UpdateCategory(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "科目 ID 不合法",
		})
		return
	}

	cat, err := h.repo.FindByID(id)
	if err != nil {
		h.internal(c, "服务暂时不可用")
		return
	}
	if cat == nil || cat.OrgID != orgID {
		platform.ErrResponse(c, http.StatusNotFound, platform.ErrCategoryNotFound)
		return
	}

	var req UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "参数不合法",
		})
		return
	}

	// 资产型科目固定规则：不改类型/期初/勾稽（余额由资金划转产生）
	if cat.Kind == "asset" &&
		(req.BalanceType != nil || req.OpeningBalanceCents != nil || req.IncludeInReconciliation != nil) {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code:    "ASSET_FIXED",
			Message: "资产型科目的类型/期初/勾稽为固定值，不支持修改",
		})
		return
	}

	updates := make(map[string]any)

	// 一级科目为分组容器：不允许期初余额/参与勾稽变更（语义见 CreateCategory）
	if cat.Level == 1 {
		if req.OpeningBalanceCents != nil {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "LEVEL1_NO_OPENING", Message: "一级科目为分组容器，不支持设置期初余额",
			})
			return
		}
		if req.IncludeInReconciliation != nil && *req.IncludeInReconciliation {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "LEVEL1_NO_RECONCILE", Message: "一级科目为分组容器，不支持参与资金勾稽",
			})
			return
		}
	}

	if req.Name != nil {
		// 检查重名
		dup, err := h.repo.IsNameDup(orgID, *req.Name, cat.ParentID, id)
		if err != nil {
			h.internal(c, "服务暂时不可用")
			return
		}
		if dup {
			platform.ErrResponse(c, http.StatusConflict, platform.ErrCategoryNameDup)
			return
		}
		updates["name"] = *req.Name
	}

	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if req.OpeningBalanceCents != nil {
		updates["opening_balance_cents"] = *req.OpeningBalanceCents
	}

	if req.BalanceType != nil {
		// 已有流水时不允许修改类型
		if cat.Level == 2 {
			count, err := h.repo.CountTransactions(id)
			if err == nil && count > 0 {
				platform.ErrResponse(c, http.StatusConflict, &platform.AppError{
					Code:    "CATEGORY_HAS_TXN",
					Message: "该科目已有流水记录，不允许修改余额类型",
				})
				return
			}
		}
		updates["balance_type"] = *req.BalanceType
	}

	if req.IncludeInReconciliation != nil {
		// 花费型科目不能参与勾稽
		bt := cat.BalanceType
		if req.BalanceType != nil {
			bt = *req.BalanceType
		}
		if *req.IncludeInReconciliation && bt == "spending" {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code:    "INVALID_REQUEST",
				Message: "只有余粮型科目能参与资金勾稽",
			})
			return
		}
		inc := 0
		if *req.IncludeInReconciliation {
			inc = 1
		}
		updates["include_in_reconciliation"] = inc
	}

	if err := h.repo.Update(id, orgID, updates); err != nil {
		h.internal(c, "更新科目失败")
		return
	}

	// 留痕（D0）：重命名 / 停用启用 / 改期初 / 改类型 / 改勾稽都记入 change_log
	if v, ok := updates["name"]; ok {
		_ = h.clog.LogUpdateField(orgID, "category", id, "name", cat.Name, v.(string))
	}
	if v, ok := updates["status"]; ok {
		_ = h.clog.LogUpdateField(orgID, "category", id, "status", cat.Status, v.(string))
	}
	if v, ok := updates["opening_balance_cents"]; ok {
		_ = h.clog.LogUpdateField(orgID, "category", id, "opening_balance_cents",
			strconv.FormatInt(cat.OpeningBalanceCents, 10), strconv.FormatInt(v.(int64), 10))
	}
	if v, ok := updates["balance_type"]; ok {
		_ = h.clog.LogUpdateField(orgID, "category", id, "balance_type", cat.BalanceType, v.(string))
	}
	if v, ok := updates["include_in_reconciliation"]; ok {
		oldS := "false"
		if cat.IncludeInReconciliation {
			oldS = "true"
		}
		newS := "false"
		if v.(int) == 1 {
			newS = "true"
		}
		_ = h.clog.LogUpdateField(orgID, "category", id, "include_in_reconciliation", oldS, newS)
	}

	updated, _ := h.repo.FindByID(id)
	platform.SuccessResponse(c, updated)
}

// DeleteCategory 删除科目（仅未被引用的）。
// DELETE /api/categories/:id
func (h *Handler) DeleteCategory(c *gin.Context) {
	orgID, ok := auth.CurrentOrgID(c)
	if !ok {
		h.unauthorized(c)
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_REQUEST", Message: "科目 ID 不合法",
		})
		return
	}

	cat, err := h.repo.FindByID(id)
	if err != nil {
		h.internal(c, "服务暂时不可用")
		return
	}
	if cat == nil || cat.OrgID != orgID {
		platform.ErrResponse(c, http.StatusNotFound, platform.ErrCategoryNotFound)
		return
	}

	// 一级科目：检查是否有子科目
	if cat.Level == 1 {
		childCount, err := h.repo.CountChildren(id)
		if err != nil {
			platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
				Code: "INTERNAL_ERROR", Message: "服务暂时不可用",
			})
			return
		}
		if childCount > 0 {
			platform.ErrResponse(c, http.StatusConflict, &platform.AppError{
				Code:    "CATEGORY_HAS_CHILD",
				Message: "该一级科目下仍有二级科目，请先处理",
			})
			return
		}
	}

	// 检查是否被流水引用
	txnCount, err := h.repo.CountTransactions(id)
	if err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "服务暂时不可用",
		})
		return
	}
	if txnCount > 0 {
		platform.ErrResponse(c, http.StatusConflict, &platform.AppError{
			Code:    "CATEGORY_IN_USE",
			Message: "该科目下仍有 " + strconv.Itoa(txnCount) + " 笔流水，无法删除。你可以将其停用",
			Details: map[string]int{"count": txnCount},
		})
		return
	}

	// 检查是否被资金划转引用（资产科目，D10）
	moveCount, err := h.repo.CountFundMoves(id)
	if err != nil {
		platform.ErrResponse(c, http.StatusInternalServerError, &platform.AppError{
			Code: "INTERNAL_ERROR", Message: "服务暂时不可用",
		})
		return
	}
	if moveCount > 0 {
		platform.ErrResponse(c, http.StatusConflict, &platform.AppError{
			Code:    "CATEGORY_IN_USE",
			Message: "该科目已有 " + strconv.Itoa(moveCount) + " 笔资金划转记录，无法删除。你可以将其停用",
			Details: map[string]int{"count": moveCount},
		})
		return
	}

	if err := h.repo.Delete(id, orgID); err != nil {
		h.internal(c, "删除科目失败")
		return
	}

	platform.SuccessResponse(c, gin.H{"ok": true})
}

// EnsureDB 确保数据库有 category 表（用于 handler 初始化时验证）。
func EnsureDB(db *sql.DB) {
	// 确保 category 表存在
	_, _ = db.Exec(`SELECT 1 FROM category LIMIT 1`)
}