// Package transaction 提供流水管理功能。
package transaction

import "time"

// Transaction 对应 txn 表。
type Transaction struct {
	ID          int64     `json:"id"`
	TxnDate     string    `json:"txnDate"`
	Direction   string    `json:"direction"`
	AmountCents int64     `json:"amountCents"`
	CategoryID  int64     `json:"categoryId"`
	Note        *string   `json:"note"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// CreateTransactionRequest 记一笔请求。
type CreateTransactionRequest struct {
	TxnDate     string `json:"txnDate" binding:"required"`
	Direction   string `json:"direction" binding:"required,oneof=income expense"`
	AmountCents int64  `json:"amountCents" binding:"required"`
	CategoryID  int64  `json:"categoryId" binding:"required"`
	Note        string `json:"note"`
}

// ListTransactionsResponse 流水列表响应。
type ListTransactionsResponse struct {
	Items   []Transaction `json:"items"`
	Total   int           `json:"total"`
	Summary struct {
		IncomeTotal  int64 `json:"incomeTotal"`
		ExpenseTotal int64 `json:"expenseTotal"`
		Balance      int64 `json:"balance"`
	} `json:"summary"`
}