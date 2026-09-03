// Package settings 提供系统配置管理（银行存款期初余额等）。
package settings

// Settings 系统配置响应。
type Settings struct {
	BankOpeningBalanceCents int64 `json:"bankOpeningBalanceCents"`
}

// UpdateSettingsRequest 修改系统配置请求。
type UpdateSettingsRequest struct {
	BankOpeningBalanceCents *int64 `json:"bankOpeningBalanceCents"`
}