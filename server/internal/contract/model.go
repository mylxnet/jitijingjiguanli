// Package contract 提供合同/附件管理（D12）：
// 按往来单位（party）挂载合同文件，文件内容以 BLOB 存 SQLite，单文件备份天然一致。
package contract

import "time"

// Contract 对应 contract 表。
type Contract struct {
	ID            int64     `json:"id"`
	OrgID         int64     `json:"orgId"`
	PartyID       int64     `json:"partyId"`
	FileName      string    `json:"fileName"`
	FileSize      int64     `json:"fileSize"`
	MimeType      string    `json:"mimeType"`
	ContractTitle string    `json:"contractTitle"`
	ContractDate  *string   `json:"contractDate"` // nullable，前端暂未开通
	ExpiresAt     *string   `json:"expiresAt"`    // nullable，前端暂未开通
	FileData      string    `json:"fileData,omitempty"` // 完整 data URL，仅详情/下载返回
	FileBytes     []byte    `json:"-"`                   // 已解码原始字节（仅 repo/内部传递，不序列化）
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// CreateContractRequest 新建合同/附件。
type CreateContractRequest struct {
	PartyID       int64   `json:"partyId" binding:"required"`
	FileName      string  `json:"fileName" binding:"required"`
	FileSize      int64   `json:"fileSize"`
	MimeType      string  `json:"mimeType"`
	ContractTitle string  `json:"contractTitle"`
	ExpiresAt     *string `json:"expiresAt"` // 可选「合同期至时间」，留空/缺省 = 无到期
	FileData      string  `json:"fileData" binding:"required"` // base64 data URL
}

// UpdateContractExpiryRequest 修改合同「合同期至时间」。ExpiresAt 为 nil/空串 = 清除到期。
type UpdateContractExpiryRequest struct {
	ExpiresAt *string `json:"expiresAt"`
}

// ExpiringItem 到期合同清单项（关联往来单位，用于「合同到期」提醒）。
type ExpiringItem struct {
	PartyID       int64  `json:"partyId"`
	PartyName     string `json:"partyName"`
	Type          string `json:"type"`
	ContractID    int64  `json:"contractId"`
	ContractTitle string `json:"contractTitle"`
	FileName      string `json:"fileName"`
	ExpiresAt     string `json:"expiresAt"`
	HasExpired    bool   `json:"hasExpired"`
	DaysUntil     int64  `json:"daysUntil"`
}