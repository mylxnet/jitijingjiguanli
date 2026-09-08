// Package changelog 提供变更留痕功能（D0 可追溯优先）。
package changelog

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

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

// ListRecentByOrg 查询某组织在 cutoff（UTC）之后（含）的变更日志，按时间倒序。
//
// 注意：SQLite julianday/strftime 只能解析 3 位小数秒，而 changed_at 存的是纳秒级
// RFC3339（如 "2026-09-08T10:26:33.6839633Z"），会解析成 NULL 使过滤失效。
// 因此改在 Go 侧解析；change_log.id 自增有序 = 写入时间有序，遇早于 cuttoff 的行即可提前终止。
func (r *Repo) ListRecentByOrg(orgID int64, cutoff time.Time) ([]ChangeLog, error) {
	rows, err := r.db.Query(
		`SELECT id, org_id, entity_type, entity_id, action, field, old_value, new_value, changed_at
		 FROM change_log WHERE org_id = ? ORDER BY id DESC`, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询操作日志失败: %w", err)
	}
	defer rows.Close()

	var items []ChangeLog
	for rows.Next() {
		var cl ChangeLog
		if err := rows.Scan(&cl.ID, &cl.OrgID, &cl.EntityType, &cl.EntityID, &cl.Action, &cl.Field, &cl.OldValue, &cl.NewValue, &cl.ChangedAt); err != nil {
			return nil, fmt.Errorf("扫描操作日志行失败: %w", err)
		}
		ts, ok := parseChangedAt(cl.ChangedAt)
		if !ok || ts.Before(cutoff) {
			// id 倒序 = 时间倒序，后续行只会更旧，提前结束。
			break
		}
		items = append(items, cl)
	}
	return items, rows.Err()
}

// parseChangedAt 兼容纳秒级 RFC3339 与标准 RFC3339。
func parseChangedAt(s string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
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

// Describe 联表取实体业务可读信息（仅业务部分，不含类型与动作）。
// 返回空串表示无可读字段或实体不存在。用于 operation-logs 富化为中文，让用户一眼看懂。
func (r *Repo) Describe(orgID int64, entityType string, entityID int64) string {
	switch entityType {
	case "party":
		var n string
		if r.db.QueryRow(`SELECT name FROM party WHERE org_id=? AND id=?`, orgID, entityID).Scan(&n) == nil {
			return n
		}
	case "receivable":
		var title string
		var amount int64
		if r.db.QueryRow(`SELECT COALESCE(title,''), COALESCE(amount_cents,0) FROM receivable WHERE org_id=? AND id=?`, orgID, entityID).Scan(&title, &amount) == nil {
			var pn string
			_ = r.db.QueryRow(`SELECT COALESCE((SELECT p.name FROM party p WHERE p.org_id=rec.org_id AND p.id=rec.party_id),'') FROM receivable rec WHERE rec.id=?`, entityID).Scan(&pn)
			desc := ""
			if pn != "" {
				desc = pn + " "
			}
			if title != "" {
				desc += title + " "
			}
			if amount > 0 {
				desc += formatCents(amount)
			}
			return strings.TrimSpace(desc)
		}
	case "receipt":
		var amount int64
		if r.db.QueryRow(`SELECT amount_cents FROM receipt WHERE org_id=? AND id=?`, orgID, entityID).Scan(&amount) == nil {
			return formatCents(amount)
		}
	case "contract":
		var name string
		if r.db.QueryRow(`SELECT COALESCE(NULLIF(TRIM(contract_title),''), file_name, '') FROM contract WHERE org_id=? AND id=?`, orgID, entityID).Scan(&name) == nil {
			return name
		}
	case "reinvest_allocation":
		var target string
		var amount int64
		if r.db.QueryRow(`SELECT target_name, COALESCE(amount_cents,0) FROM reinvest_allocation WHERE org_id=? AND id=?`, orgID, entityID).Scan(&target, &amount) == nil {
			desc := target
			if amount > 0 {
				if desc != "" {
					desc += " "
				}
				desc += formatCents(amount)
			}
			return desc
		}
	case "transaction", "txn":
		var cat, note string
		var amount int64
		if err := r.db.QueryRow(
			`SELECT COALESCE(c.name,''), COALESCE(t.note,''), COALESCE(t.amount_cents,0)
			 FROM txn t LEFT JOIN category c ON c.id = t.category_id
			 WHERE t.org_id=? AND t.id=?`, orgID, entityID,
		).Scan(&cat, &note, &amount); err == nil {
			desc := cat
			if note != "" {
				if desc != "" {
					desc += " "
				}
				desc += note
			}
			if amount > 0 {
				if desc != "" {
					desc += " "
				}
				desc += formatCents(amount)
			}
			return desc
		}
	case "category":
		var n string
		if r.db.QueryRow(`SELECT name FROM category WHERE org_id=? AND id=?`, orgID, entityID).Scan(&n) == nil {
			return n
		}
	case "user":
		var n string
		if r.db.QueryRow(`SELECT username FROM user WHERE id=?`, entityID).Scan(&n) == nil {
			return n
		}
	}
	return ""
}

// formatCents 金额分 → "1000.00元"。
func formatCents(c int64) string {
	if c < 0 {
		return "-" + formatCents(-c)
	}
	return fmt.Sprintf("%d.%02d元", c/100, c%100)
}
