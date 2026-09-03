package changelog

// ChangeLog 对应 change_log 表。
type ChangeLog struct {
	ID          int64   `json:"id"`
	EntityType  string  `json:"entityType"`
	EntityID    int64   `json:"entityId"`
	Action      string  `json:"action"`
	Field       *string `json:"field"`
	OldValue    *string `json:"oldValue"`
	NewValue    *string `json:"newValue"`
	ChangedAt   string  `json:"changedAt"`
}
