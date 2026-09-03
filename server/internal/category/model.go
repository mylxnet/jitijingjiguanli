// Package category 提供科目管理功能。
package category

import "time"

// Category 对应 category 表。
type Category struct {
	ID                       int64     `json:"id"`
	Name                     string    `json:"name"`
	Level                    int       `json:"level"`
	ParentID                 *int64    `json:"parentId"`
	Status                   string    `json:"status"`
	BalanceType              string    `json:"balanceType"`
	OpeningBalanceCents      int64     `json:"openingBalanceCents"`
	IncludeInReconciliation  bool      `json:"includeInReconciliation"`
	SortOrder                int       `json:"sortOrder"`
	CreatedAt                time.Time `json:"createdAt"`
	UpdatedAt                time.Time `json:"updatedAt"`
	// 计算字段（不存库）
	BalanceCents *int64        `json:"balanceCents,omitempty"`
	TxnCount     *int          `json:"txnCount,omitempty"`
	Children     []*Category   `json:"children,omitempty"`
}

// CreateCategoryRequest 新建科目请求。
type CreateCategoryRequest struct {
	Name                    string `json:"name" binding:"required"`
	Level                   int    `json:"level" binding:"required,oneof=1 2"`
	ParentID                *int64 `json:"parentId"`
	BalanceType             string `json:"balanceType" binding:"required,oneof=residual spending"`
	OpeningBalanceCents     int64  `json:"openingBalanceCents"`
	IncludeInReconciliation *bool  `json:"includeInReconciliation"`
	SortOrder               int    `json:"sortOrder"`
}

// UpdateCategoryRequest 更新科目请求。
type UpdateCategoryRequest struct {
	Name                    *string `json:"name"`
	Status                  *string `json:"status" binding:"omitempty,oneof=active inactive"`
	OpeningBalanceCents     *int64  `json:"openingBalanceCents"`
	BalanceType             *string `json:"balanceType" binding:"omitempty,oneof=residual spending"`
	IncludeInReconciliation *bool   `json:"includeInReconciliation"`
}