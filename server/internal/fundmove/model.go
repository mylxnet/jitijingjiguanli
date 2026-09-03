// Package fundmove 提供资金划转功能（D10）：
// 投资（invest）= 银行存款 → 资产科目；收回（recover）= 资产科目 → 银行存款。
package fundmove

import "time"

// FundMove 对应 fund_move 表。
type FundMove struct {
	ID              int64     `json:"id"`
	OrgID           int64     `json:"orgId"`
	MoveDate        string    `json:"moveDate"`
	Kind            string    `json:"kind"` // invest 投资 / recover 收回
	AssetCategoryID int64     `json:"assetCategoryId"`
	AmountCents     int64     `json:"amountCents"`
	Note            *string   `json:"note"`
	Status          string    `json:"status"` // normal / voided
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// CreateFundMoveRequest 创建资金划转请求。
type CreateFundMoveRequest struct {
	MoveDate        string `json:"moveDate" binding:"required"`
	Kind            string `json:"kind" binding:"required,oneof=invest recover"`
	AssetCategoryID int64  `json:"assetCategoryId" binding:"required"`
	AmountCents     int64  `json:"amountCents"`
	Note            string `json:"note"`
}

// FundMoveListResponse 资金划转列表响应。
type FundMoveListResponse struct {
	Items []FundMove `json:"items"`
	Total int        `json:"total"`
}
