// Package receivable 提供应收/往来功能（D11）：
// party 往来单位、receivable 应收单、receipt 收款核销（现金入账 cash / 抵销 offset）。
package receivable

import "time"

// Party 对应 party 表（往来单位；v0.3.6+ 需求：只有往来单位，无农户类型）。
type Party struct {
	ID               int64     `json:"id"`
	OrgID            int64     `json:"orgId"`
	Name             string    `json:"name"`
	Note             *string   `json:"note"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	OutstandingCents int64     `json:"outstandingCents"` // 欠款合计 = Σ(未核销应收余额，open)，列表/详情用
}

// Receivable 对应 receivable 表（应收单）。
type Receivable struct {
	ID               int64     `json:"id"`
	OrgID            int64     `json:"orgId"`
	PartyID          int64     `json:"partyId"`
	PartyName        string    `json:"partyName"` // join party.name，供列表/详情展示
	RecvYear         int       `json:"recvYear"`  // 归属年度（批量计提结转用，防重）
	RecvKind         string    `json:"recvKind"`  // rent 流转费 / dividend 投资收益 / other 其他
	Title            string    `json:"title"`
	AmountCents      int64     `json:"amountCents"`
	IncomeCategoryID *int64    `json:"incomeCategoryId"`
	Status           string    `json:"status"` // open / closed
	Note             *string   `json:"note"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	PaidCents        int64     `json:"paidCents"`        // 已核销（normal receipts 合计）
	OutstandingCents int64     `json:"outstandingCents"` // 未收 = amount - paid
}

// Receivable 创建请求（v0.4 支持年度批量计提）。
type CreateReceivableRequest struct {
	PartyID          int64  `json:"partyId" binding:"required"`
	RecvYear         int    `json:"recvYear"` // 0 = 当前年
	RecvKind         string `json:"recvKind" binding:"required,oneof=rent dividend other"`
	Title            string `json:"title" binding:"required"`
	AmountCents      int64  `json:"amountCents"`
	IncomeCategoryID *int64 `json:"incomeCategoryId"`
	Note             string `json:"note"`
}

// BatchAccrueRequest 批量计提应收：同一年度一批（可多单位/多类别）。
type BatchAccrueRequest struct {
	RecvYear int               `json:"recvYear"`
	Title    string            `json:"title" binding:"required"`
	Items    []BatchAccrueItem `json:"items" binding:"required,min=1"`
}

// BatchAccrueItem 批量计提明细行。
type BatchAccrueItem struct {
	PartyID     int64  `json:"partyId" binding:"required"`
	RecvKind    string `json:"recvKind" binding:"required,oneof=rent dividend other"`
	AmountCents int64  `json:"amountCents"`
}

// BatchAccrueResult 批量计提结果（created=新增 / skipped=同年同类已存在跳过）。
type BatchAccrueResult struct {
	Created int `json:"created"`
	Skipped int `json:"skipped"`
}

// AccrualStandard 流转费年度计提标准（预存金额，10月一键结转）。
type AccrualStandard struct {
	ID          int64     `json:"id"`
	OrgID       int64     `json:"orgId"`
	PartyID     int64     `json:"partyId"`
	PartyName   string    `json:"partyName"`
	RecvKind    string    `json:"recvKind"` // rent 等
	AmountCents int64     `json:"amountCents"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// AccrualStandardRequest 保存标准。
type AccrualStandardRequest struct {
	PartyID     int64  `json:"partyId" binding:"required"`
	RecvKind    string `json:"recvKind" binding:"required,oneof=rent dividend other"`
	AmountCents int64  `json:"amountCents"`
	Active      *bool  `json:"active"`
}

// AccrueResult 一键结转结果。
type AccrueResult struct {
	Created int `json:"created"`
	Skipped int `json:"skipped"`
}

// Receipt 对应 receipt 表（核销记录）。
type Receipt struct {
	ID           int64     `json:"id"`
	OrgID        int64     `json:"orgId"`
	ReceivableID int64     `json:"receivableId"`
	AmountCents  int64     `json:"amountCents"`
	ReceiptDate  string    `json:"receiptDate"`
	Method       string    `json:"method"` // cash 现金（生成银行收入流水）/ offset 抵销（关联发放支出流水）
	TxnID        *int64    `json:"txnId"`
	Note         *string   `json:"note"`
	Status       string    `json:"status"` // normal / voided
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// CreatePartyRequest 新建往来单位。
type CreatePartyRequest struct {
	Name string `json:"name" binding:"required"`
	Note string `json:"note"`
}

// UpdatePartyRequest 更新往来单位（名称/备注）。
type UpdatePartyRequest struct {
	Name *string `json:"name"`
	Note *string `json:"note"`
}

// CreateReceiptRequest 收款核销。
// method=cash：现金收款，自动生成银行收入流水（科目 = 应收单预设 incomeCategoryId，未预设时用 categoryId 必传）；
// method=offset：抵销，关联一条已存在的发放支出流水（txnId 必传），不产生现金流水。
type CreateReceiptRequest struct {
	AmountCents int64  `json:"amountCents"`
	ReceiptDate string `json:"receiptDate" binding:"required"`
	Method      string `json:"method" binding:"required,oneof=cash offset"`
	CategoryID  *int64 `json:"categoryId"`
	TxnID       *int64 `json:"txnId"`
	Note        string `json:"note"`
}

// ReceivableListResponse 应收单列表响应。
type ReceivableListResponse struct {
	Items []Receivable `json:"items"`
	Total int          `json:"total"`
}

// ReceivableDetail 应收单详情（含核销记录）。
type ReceivableDetail struct {
	Receivable Receivable `json:"receivable"`
	Receipts   []Receipt  `json:"receipts"`
}
