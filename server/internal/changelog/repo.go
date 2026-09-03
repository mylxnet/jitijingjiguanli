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

// LogChange 记录一次变更（归属组织 orgID，v0.3 多组织）。
func (r *Repo) LogChange(orgID int64, entityType string, entityID int64, action string, field *string, oldValue, newValue *string) error {
	now := platform.Now()
	_, err := r.db.Exec(
		`INSERT INTO change_log(org_id, entity_type, entity_id, action, field, old_value, new_value, changed_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		orgID, entityType, entityID, action, field, oldValue, newValue, now,
	)
	if err != nil {
		return fmt.Errorf("记录变更日志失败: %w", err)
	}
	return nil
}

// ListByEntity 查询某组织下某实体的变更日志，按时间倒序。
func (r *Repo) ListByEntity(orgID int64, entityType string, entityID int64) ([]ChangeLog, error) {
	rows, err := r.db.Query(
		`SELECT id, entity_type, entity_id, action, field, old_value, new_value, changed_at
		 FROM change_log WHERE org_id = ? AND entity_type = ? AND entity_id = ?
		 ORDER BY changed_at DESC`, orgID, entityType, entityID,
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
func (r *Repo) LogCreate(orgID int64, entityType string, entityID int64) error {
	return r.LogChange(orgID, entityType, entityID, "create", nil, nil, nil)
}

// LogChangeVoid 记录作废/撤销操作（action 语义由调用方表达：void/unvoid）。
func (r *Repo) LogChangeVoid(orgID int64, entityType string, entityID int64, action, oldStatus, newStatus string) error {
	field := "status"
	old := oldStatus
	new := newStatus
	return r.LogChange(orgID, entityType, entityID, action, &field, &old, &new)
}

// LogUpdateField 记录单个字段更新。
func (r *Repo) LogUpdateField(orgID int64, entityType string, entityID int64, field string, oldValue, newValue string) error {
	return r.LogChange(orgID, entityType, entityID, "update", &field, &oldValue, &newValue)
}
