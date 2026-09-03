// Package transfer 提供科目间转账功能（1 转出 → N 转入，见需求文档 D7）。
package transfer

import "time"

// Transfer 对应 transfer 表。
type Transfer struct {
	ID                int64     `json:"id"`
	OrgID             int64     `json:"orgId"`
	TxnDate           string    `json:"txnDate"`
	SourceCategoryID  int64     `json:"sourceCategoryId"`
	SourceAmountCents int64     `json:"sourceAmountCents"`
	Note              *string   `json:"note"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
	Legs              []Leg     `json:"legs,omitempty"`
}

// Leg 对应 transfer_leg 表。
type Leg struct {
	ID           int64 `json:"id"`
	TransferID   int64 `json:"transferId"`
	CategoryID   int64 `json:"categoryId"`
	AmountCents  int64 `json:"amountCents"`
}

// CreateTransferRequest 创建转账请求。
type CreateTransferRequest struct {
	TxnDate           string          `json:"txnDate" binding:"required"`
	SourceCategoryID  int64           `json:"sourceCategoryId" binding:"required"`
	SourceAmountCents int64           `json:"sourceAmountCents" binding:"required"`
	Note              string          `json:"note"`
	Legs              []CreateLegReq  `json:"legs" binding:"required,min=1"`
}

// CreateLegReq 创建转入明细请求。
type CreateLegReq struct {
	CategoryID  int64 `json:"categoryId" binding:"required"`
	AmountCents int64 `json:"amountCents" binding:"required"`
}

// TransferListResponse 转账列表响应。
type TransferListResponse struct {
	Items []Transfer `json:"items"`
	Total int        `json:"total"`
}