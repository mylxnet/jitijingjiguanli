// Package receivable 提供应收/往来功能（D11）：
// party 往来对象（农户/单位）、receivable 应收单、receipt 收款核销（现金入账 cash / 抵销 offset）。
package receivable

import "time"

// Party 对应 party 表（往来对象）。
type Party struct {
	ID               int64     `json:"id"`
	OrgID            int64     `json:"orgId"`
	Name             string    `json:"name"`
	Kind             string    `json:"kind"` // household 农户 / unit 单位
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

// CreatePartyRequest 新建往来对象。
type CreatePartyRequest struct {
	Name string `json:"name" binding:"required"`
	Kind string `json:"kind" binding:"required,oneof=household unit"`
	Note string `json:"note"`
}

// UpdatePartyRequest 更新往来对象（名称/类别/备注）。
type UpdatePartyRequest struct {
	Name *string `json:"name"`
	Kind *string `json:"kind" binding:"omitempty,oneof=household unit"`
	Note *string `json:"note"`
}

// CreateReceivableRequest 登记应收单。
type CreateReceivableRequest struct {
	PartyID          int64  `json:"partyId" binding:"required"`
	RecvKind         string `json:"recvKind" binding:"required,oneof=rent dividend other"`
	Title            string `json:"title" binding:"required"`
	AmountCents      int64  `json:"amountCents"`
	IncomeCategoryID *int64 `json:"incomeCategoryId"`
	Note             string `json:"note"`
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
