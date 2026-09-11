// Package receivable 提供应收/往来功能（D11）：
// party 往来单位、receivable 应收单、receipt 收款核销（现金入账 cash / 抵销 offset）。
package receivable

import "time"

// Party 对应 party 表（往来单位；v0.7 起支持多选类型 + 投资/流转专属字段）。
type Party struct {
	ID                int64     `json:"id"`
	OrgID             int64     `json:"orgId"`
	Name              string    `json:"name"`
	Type              string    `json:"type"`               // 单值（兼容旧前端，取 types[0]）
	Types             []string  `json:"types"`              // 多选数组（v0.7）
	ContactPhone      string    `json:"contactPhone"`       // 联系电话
	AreaMu            float64   `json:"areaMu"`             // 流转面积（亩，流转企业用）
	Note              *string   `json:"note"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
	OutstandingCents  int64     `json:"outstandingCents"`   // 欠款合计
	Deletable         bool      `json:"deletable"`          // 是否可删除（无欠款 + 科目余额0 + 未被引用）
	DeleteBlockReason string    `json:"deleteBlockReason"`  // 不可删除原因（可删时为空）

	// --- 投资/再投资专属字段（invest / reinvest 类型用）---
	InvestAmountCents    int64  `json:"investAmountCents"`    // 投资本金
	ReturnRateBps        int    `json:"returnRateBps"`        // 收益率基点（500 = 5.00%）
	ExpectedReturnCents  int64  `json:"expectedReturnCents"`  // 年收益（自动算=本金×收益率/10000，可手动改）

	// --- 土地流转专属字段（flow 类型用）---
	LandMu                   float64 `json:"landMu"`                   // 流转亩数
	LandFeePerMuCents        int64   `json:"landFeePerMuCents"`        // 每亩年流转费
	ExpectedLandFeeCents     int64   `json:"expectedLandFeeCents"`     // 总流转费（自动=亩数×每亩费，可改）
	MgmtFeePerMuCents        int64   `json:"mgmtFeePerMuCents"`        // 每亩年管理费
	ExpectedMgmtFeeCents     int64   `json:"expectedMgmtFeeCents"`     // 总管理费（自动=亩数×每亩管理费，可改）
}

// validPartyType 校验单位类型是否合法。
func validPartyType(t string) bool {
	return t == "flow" || t == "invest" || t == "reinvest" || t == "other"
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
	PaidCents         int64   `json:"paidCents"`         // 已核销合计（全部 normal receipts，含坏账）
	WriteoffCents     int64   `json:"writeoffCents"`     // 其中坏账核销（method=writeoff）合计
	WriteoffDate      *string `json:"writeoffDate"`      // 坏账核销日期（存在坏账时才有）
	WriteoffNote      *string `json:"writeoffNote"`      // 坏账原因（存在坏账时才有）
	WriteoffReceiptID *int64  `json:"writeoffReceiptId"` // 坏账核销记录 id（供撤销）
	OutstandingCents  int64   `json:"outstandingCents"`  // 未收 = amount - paid
}

// Receivable 创建请求（v0.4 支持年度批量计提）。
type CreateReceivableRequest struct {
	PartyID          int64  `json:"partyId" binding:"required"`
	RecvYear         int    `json:"recvYear"` // 0 = 当前年
	RecvKind         string `json:"recvKind" binding:"required,oneof=rent dividend service reinvest_dividend other"`
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
	RecvKind    string `json:"recvKind" binding:"required,oneof=rent dividend service reinvest_dividend other"`
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
	RecvKind    string `json:"recvKind" binding:"required,oneof=rent dividend service reinvest_dividend other"`
	AmountCents int64  `json:"amountCents"`
	Active      *bool  `json:"active"`
}

// AccrueResult 一键结转结果。
type AccrueResult struct {
	Created int `json:"created"`
	Skipped int `json:"skipped"`
}

// PreviewAccrueItem 年度结转预览行（按单位年度标准 + 是否存在同年同类应收单）。
type PreviewAccrueItem struct {
	Kind        string `json:"kind"`        // rent / dividend
	Title       string `json:"title"`       // 将生成的事由（如 2026年度土地流转费）
	PartyID     int64  `json:"partyId"`
	PartyName   string `json:"partyName"`
	AmountCents int64  `json:"amountCents"`
	Exists      bool   `json:"exists"` // true=同年同类已存在，确认结转时会跳过
}

// PreviewAccrueResult 预览结果。
type PreviewAccrueResult struct {
	Year  int                 `json:"year"`
	Items []PreviewAccrueItem `json:"items"`
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
	Name         string   `json:"name" binding:"required"`
	Type         string   `json:"type"`  // 空 = flow
	Types        []string `json:"types"` // 多选数组，优先级高于 type
	ContactPhone string   `json:"contactPhone"`
	AreaMu       float64  `json:"areaMu"`
	Note         string   `json:"note"`

	// 投资/再投资字段
	InvestAmountCents   int64 `json:"investAmountCents"`
	ReturnRateBps       int   `json:"returnRateBps"`
	ExpectedReturnCents int64 `json:"expectedReturnCents"`

	// 土地流转字段
	LandMu               float64 `json:"landMu"`
	LandFeePerMuCents    int64   `json:"landFeePerMuCents"`
	ExpectedLandFeeCents int64   `json:"expectedLandFeeCents"`
	MgmtFeePerMuCents    int64   `json:"mgmtFeePerMuCents"`
	ExpectedMgmtFeeCents int64   `json:"expectedMgmtFeeCents"`
}

// UpdatePartyRequest 更新往来单位。
type UpdatePartyRequest struct {
	Name         *string   `json:"name"`
	Type         *string   `json:"type"`
	Types        *[]string `json:"types"`
	ContactPhone *string   `json:"contactPhone"`
	AreaMu       *float64  `json:"areaMu"`
	Note         *string   `json:"note"`

	InvestAmountCents   *int64 `json:"investAmountCents"`
	ReturnRateBps       *int   `json:"returnRateBps"`
	ExpectedReturnCents *int64 `json:"expectedReturnCents"`

	LandMu               *float64 `json:"landMu"`
	LandFeePerMuCents    *int64   `json:"landFeePerMuCents"`
	ExpectedLandFeeCents *int64   `json:"expectedLandFeeCents"`
	MgmtFeePerMuCents    *int64   `json:"mgmtFeePerMuCents"`
	ExpectedMgmtFeeCents *int64   `json:"expectedMgmtFeeCents"`
}

// ReinvestAllocation 再投资去向明细（子表）。
type ReinvestAllocation struct {
	ID            int64     `json:"id"`
	PartyID       int64     `json:"partyId"`
	TargetName    string    `json:"targetName"`
	TargetPartyID *int64    `json:"targetPartyId,omitempty"` // 可选
	AmountCents   int64     `json:"amountCents"`
	Notes         *string   `json:"notes"`
	CreatedAt     time.Time `json:"createdAt"`
}

// ReinvestAllocationRequest 新增再投资去向。
type ReinvestAllocationRequest struct {
	TargetName    string `json:"targetName" binding:"required"`
	TargetPartyID *int64 `json:"targetPartyId"` // 可选
	AmountCents   int64  `json:"amountCents"`
	Notes         string `json:"notes"`
}

// CreateReceiptRequest 核销请求。
// method=cash：现金收款，自动生成银行收入流水（科目 = 应收单预设 incomeCategoryId，未预设时用 categoryId 必传）；
// method=offset：抵销，关联一条已存在的发放支出流水（txnId 必传），不产生现金流水；
// method=writeoff：坏账核销，把应收单剩余待收全额清零（不生成流水、不需科目/流水，note 必填）。
type CreateReceiptRequest struct {
	AmountCents int64  `json:"amountCents"`
	ReceiptDate string `json:"receiptDate" binding:"required"`
	Method      string `json:"method" binding:"required,oneof=cash offset writeoff"`
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

// Distribution532 532分配方案（按 org+year 唯一，一年一条）。
type Distribution532 struct {
	ID               int64     `json:"id"`
	OrgID            int64     `json:"orgId"`
	Year             int       `json:"year"`
	TotalIncomeCents int64     `json:"totalIncomeCents"` // 分配基准总收益（快照）
	ReinvestCents    int64     `json:"reinvestCents"`    // 再投资
	DividendCents    int64     `json:"dividendCents"`    // 成员分红福利
	WelfareCents     int64     `json:"welfareCents"`     // 管理公益支出
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// SaveDistribution532Request 保存/更新某年 532 分配方案。
type SaveDistribution532Request struct {
	Year             int64 `json:"year" binding:"required"`
	TotalIncomeCents int64 `json:"totalIncomeCents"`
	ReinvestCents    int64 `json:"reinvestCents"`
	DividendCents    int64 `json:"dividendCents"`
	WelfareCents     int64 `json:"welfareCents"`
}

