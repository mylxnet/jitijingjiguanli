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

// Register 挂载路由。
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

// buildTree 将扁平科目列表构建为树结构，并计算余额（asset=资金划转余额；equity=收支持平余额）。
func (h *Handler) buildTree(cats []*Category) []*Category {
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

// CreateCategory 新建科目（v0.4：二级类型 = 资产 asset / 权益 equity）。
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

	kind := "equity"
	if req.Kind != nil {
		kind = *req.Kind
	}

	// 一级为分组容器：不支持指定资产类型（资产/权益语义只作用于二级）
	if req.Level == 1 {
		req.ParentID = nil
		if kind == "asset" {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "ASSET_LEVEL", Message: "资产/权益类型只设置在二级科目上；一级科目用于分组",
			})
			return
		}
	} else {
		if req.ParentID == nil {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "INVALID_REQUEST", Message: "二级科目必须指定所属一级科目",
			})
			return
		}
		// 资产科目仅限二级（资金划转只挂二级）
		if kind == "asset" && req.Level != 2 {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "ASSET_LEVEL", Message: "资产科目只能是二级科目",
			})
			return
		}
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

	// 期初校验：金额非负；一级（分组容器）不允许设期初
	if req.OpeningBalanceCents < 0 {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "INVALID_OPENING", Message: "期初余额不能为负数",
		})
		return
	}
	if req.Level == 1 && req.OpeningBalanceCents != 0 {
		platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
			Code: "LEVEL1_NO_OPENING", Message: "一级科目为分组容器，不支持设置期初余额（请设在二级上）",
		})
		return
	}

	// 重名
	dup, err := h.repo.IsNameDup(orgID, req.Name, req.ParentID, 0)
	if err != nil {
		h.internal(c, "服务暂时不可用")
		return
	}
	if dup {
		platform.ErrResponse(c, http.StatusConflict, platform.ErrCategoryNameDup)
		return
	}

	cat := &Category{
		OrgID:     orgID,
		Name:      req.Name,
		Level:     req.Level,
		ParentID:  req.ParentID,
		Status:    "active",
		Kind:      kind,
		Opening:   req.OpeningBalanceCents,
		SortOrder: req.SortOrder,
	}

	created, err := h.repo.Create(cat)
	if err != nil {
		h.internal(c, "创建科目失败")
		return
	}

	platform.SuccessResponse(c, created)
}

// UpdateCategory 更新科目（重命名 / 停用 / 启用；kind 不可变）。
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

	updates := make(map[string]any)

	if req.Name != nil {
		if cat.Preset {
			platform.ErrResponse(c, http.StatusForbidden, platform.ErrCategoryPreset)
			return
		}
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
		if *req.OpeningBalanceCents < 0 {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "INVALID_OPENING", Message: "期初余额不能为负数",
			})
			return
		}
		if cat.Level == 1 {
			platform.ErrResponse(c, http.StatusBadRequest, &platform.AppError{
				Code: "LEVEL1_NO_OPENING", Message: "一级科目为分组容器，不支持设置期初余额",
			})
			return
		}
		updates["opening_balance_cents"] = *req.OpeningBalanceCents
	}

	if err := h.repo.Update(id, orgID, updates); err != nil {
		h.internal(c, "更新科目失败")
		return
	}

	if v, ok := updates["name"]; ok {
		_ = h.clog.LogUpdateField(orgID, "category", id, "name", cat.Name, v.(string))
	}
	if v, ok := updates["status"]; ok {
		_ = h.clog.LogUpdateField(orgID, "category", id, "status", cat.Status, v.(string))
	}
	if v, ok := updates["opening_balance_cents"]; ok {
		_ = h.clog.LogUpdateField(orgID, "category", id, "opening_balance_cents",
			strconv.FormatInt(cat.Opening, 10), strconv.FormatInt(v.(int64), 10))
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

	if cat.Preset {
		platform.ErrResponse(c, http.StatusForbidden, platform.ErrCategoryPreset)
		return
	}

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
	_, _ = db.Exec(`SELECT 1 FROM category LIMIT 1`)
}
