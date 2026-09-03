// Package changelog 提供变更留痕功能（D0 可追溯优先）。
package changelog

import (
	"database/sql"
	"fmt"

	"jititaizhang/server/internal/platform"
)

// Repo 封装 change_log 表的 SQL 查询。
type Repo struct {
	db *sql.DB
}

// NewRepo 创建 Repo。
func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// LogChange 记录一次变更。
func (r *Repo) LogChange(entityType string, entityID int64, action string, field *string, oldValue, newValue *string) error {
	now := platform.Now()
	_, err := r.db.Exec(
		`INSERT INTO change_log(entity_type, entity_id, action, field, old_value, new_value, changed_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?)`,
		entityType, entityID, action, field, oldValue, newValue, now,
	)
	if err != nil {
		return fmt.Errorf("记录变更日志失败: %w", err)
	}
	return nil
}

// ListByEntity 查询某个实体（transaction/transfer）的变更日志，按时间倒序。
func (r *Repo) ListByEntity(entityType string, entityID int64) ([]ChangeLog, error) {
	rows, err := r.db.Query(
		`SELECT id, entity_type, entity_id, action, field, old_value, new_value, changed_at
		 FROM change_log WHERE entity_type = ? AND entity_id = ?
		 ORDER BY changed_at DESC`, entityType, entityID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询变更日志失败: %w", err)
	}
	defer rows.Close()

	var items []ChangeLog
	for rows.Next() {
		var cl ChangeLog
		if err := rows.Scan(&cl.ID, &cl.EntityType, &cl.EntityID, &cl.Action, &cl.Field, &cl.OldValue, &cl.NewValue, &cl.ChangedAt); err != nil {
			return nil, fmt.Errorf("扫描变更日志行失败: %w", err)
		}
		items = append(items, cl)
	}
	return items, rows.Err()
}

// LogCreate 记录创建操作。
func (r *Repo) LogCreate(entityType string, entityID int64) error {
	return r.LogChange(entityType, entityID, "create", nil, nil, nil)
}

// LogVoid 记录作废操作。
func (r *Repo) LogChangeVoid(entityType string, entityID int64, oldStatus, newStatus string) error {
	field := "status"
	old := oldStatus
	new := newStatus
	return r.LogChange(entityType, entityID, "void", &field, &old, &new)
}

// LogUpdateField 记录单个字段更新。
func (r *Repo) LogUpdateField(entityType string, entityID int64, field string, oldValue, newValue string) error {
	return r.LogChange(entityType, entityID, "update", &field, &oldValue, &newValue)
}
