// Package category 提供科目管理（v0.4：二级类型 = 资产 asset / 权益 equity）。
package category

import "time"

// Category 对应 category 表。
type Category struct {
	ID        int64     `json:"id"`
	OrgID     int64     `json:"orgId"`
	Name      string    `json:"name"`
	Level     int       `json:"level"`
	ParentID  *int64    `json:"parentId"`
	Status    string    `json:"status"`
	Kind      string    `json:"kind"` // asset 资产 / equity 权益（v0.4）
	Opening   int64     `json:"openingBalanceCents"`
	Preset    bool      `json:"preset"`
	SortOrder int       `json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// 计算字段（不存库）
	BalanceCents *int64      `json:"balanceCents,omitempty"`
	TxnCount     *int        `json:"txnCount,omitempty"`
	Children     []*Category `json:"children,omitempty"`
}

// CreateCategoryRequest 新建科目请求。
type CreateCategoryRequest struct {
	Name  string `json:"name" binding:"required"`
	Level int    `json:"level" binding:"required,oneof=1 2"`
	// ParentID 二级科目必填
	ParentID *int64 `json:"parentId"`
	// Kind 二级科目必填：asset / equity；一级为分组容器不填（默认 equity）
	Kind                *string `json:"kind" binding:"omitempty,oneof=asset equity"`
	OpeningBalanceCents int64   `json:"openingBalanceCents"`
	SortOrder           int     `json:"sortOrder"`
}

// UpdateCategoryRequest 更新科目（kind 不可变；名称/状态/期初）。
type UpdateCategoryRequest struct {
	Name                *string `json:"name"`
	Status              *string `json:"status" binding:"omitempty,oneof=active inactive"`
	OpeningBalanceCents *int64  `json:"openingBalanceCents"`
}
