package fundmove

import (
	"database/sql"
	"fmt"
	"time"

	"jititaizhang/server/internal/platform"
)

// Repo 封装 fund_move 表的 SQL 查询。
type Repo struct {
	db *sql.DB
}

// NewRepo 创建 Repo。
func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// Create 新建一笔资金划转（单行写入，无需多表事务）。
func (r *Repo) Create(m *FundMove) (*FundMove, error) {
	now := platform.Now()
	res, err := r.db.Exec(
		`INSERT INTO fund_move(org_id, move_date, kind, asset_category_id, amount_cents, note, status, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, 'normal', ?, ?)`,
		m.OrgID, m.MoveDate, m.Kind, m.AssetCategoryID, m.AmountCents, m.Note, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("创建资金划转失败: %w", err)
	}
	id, _ := res.LastInsertId()
	m.ID = id
	m.Status = "normal"
	m.CreatedAt = now
	m.UpdatedAt = now
	return m, nil
}

// FindByID 按 ID 查询资金划转（调用方需校验 OrgID 归属）。
func (r *Repo) FindByID(id int64) (*FundMove, error) {
	m := &FundMove{}
	err := r.db.QueryRow(
		`SELECT id, org_id, move_date, kind, asset_category_id, amount_cents, note, status, created_at, updated_at
		 FROM fund_move WHERE id = ?`, id,
	).Scan(&m.ID, &m.OrgID, &m.MoveDate, &m.Kind, &m.AssetCategoryID, &m.AmountCents, &m.Note, &m.Status, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询资金划转失败: %w", err)
	}
	return m, nil
}

// List 查询某组织资金划转列表（日期/类型过滤，日期倒序分页）。
func (r *Repo) List(orgID int64, from, to, kind string, page, pageSize int) ([]FundMove, int, error) {
	where := "WHERE org_id = ?"
	var args []any
	args = append(args, orgID)

	if from != "" {
		where += " AND move_date >= ?"
		args = append(args, from)
	}
	if to != "" {
		where += " AND move_date <= ?"
		args = append(args, to)
	}
	if kind == "invest" || kind == "recover" {
		where += " AND kind = ?"
		args = append(args, kind)
	}

	var total int
	if err := r.db.QueryRow("SELECT COUNT(*) FROM fund_move "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计资金划转数失败: %w", err)
	}

	offset := (page - 1) * pageSize
	listArgs := append(args, pageSize, offset)
	rows, err := r.db.Query(
		`SELECT id, org_id, move_date, kind, asset_category_id, amount_cents, note, status, created_at, updated_at
		 FROM fund_move `+where+` ORDER BY move_date DESC, id DESC LIMIT ? OFFSET ?`, listArgs...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("查询资金划转列表失败: %w", err)
	}
	defer rows.Close()

	var items []FundMove
	for rows.Next() {
		m := FundMove{}
		if err := rows.Scan(&m.ID, &m.OrgID, &m.MoveDate, &m.Kind, &m.AssetCategoryID, &m.AmountCents, &m.Note, &m.Status, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("扫描资金划转行失败: %w", err)
		}
		items = append(items, m)
	}
	return items, total, rows.Err()
}

// UpdateStatus 更新资金划转状态（作废/撤销，限定本组织）。
func (r *Repo) UpdateStatus(id, orgID int64, status string, updatedAt time.Time) error {
	_, err := r.db.Exec(`UPDATE fund_move SET status = ?, updated_at = ? WHERE id = ? AND org_id = ?`, status, updatedAt, id, orgID)
	return err
}
