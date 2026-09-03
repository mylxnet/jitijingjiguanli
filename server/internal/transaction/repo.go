package transaction

import (
	"database/sql"
	"fmt"

	"jititaizhang/server/internal/platform"
)

// Repo 封装 txn 表的 SQL 查询。
type Repo struct {
	db *sql.DB
}

// NewRepo 创建 Repo。
func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// Create 创建一笔流水（归属组织 orgID 取自 t.OrgID）。
func (r *Repo) Create(t *Transaction) (*Transaction, error) {
	now := platform.Now()
	res, err := r.db.Exec(
		`INSERT INTO txn(org_id, txn_date, direction, amount_cents, category_id, note, status, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, 'normal', ?, ?)`,
		t.OrgID, t.TxnDate, t.Direction, t.AmountCents, t.CategoryID, t.Note, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("创建流水失败: %w", err)
	}
	id, _ := res.LastInsertId()
	t.ID = id
	t.Status = "normal"
	t.CreatedAt = now
	t.UpdatedAt = now
	return t, nil
}

// FindByID 按 ID 查询流水（调用方需校验 OrgID 归属）。
func (r *Repo) FindByID(id int64) (*Transaction, error) {
	t := &Transaction{}
	err := r.db.QueryRow(
		`SELECT id, org_id, txn_date, direction, amount_cents, category_id, note, status, created_at, updated_at
		 FROM txn WHERE id = ?`, id,
	).Scan(&t.ID, &t.OrgID, &t.TxnDate, &t.Direction, &t.AmountCents, &t.CategoryID, &t.Note, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询流水失败: %w", err)
	}
	return t, nil
}

// List 按日期倒序查询某组织的流水列表（v0.3 多组织隔离）。
func (r *Repo) List(orgID int64, from, to string, categoryID *int64, keyword string, minAmount, maxAmount *int64, includeVoided bool, page, pageSize int) ([]*Transaction, int, error) {
	where := "WHERE org_id = ?"
	var args []any
	args = append(args, orgID)

	if from != "" {
		where += " AND txn_date >= ?"
		args = append(args, from)
	}
	if to != "" {
		where += " AND txn_date <= ?"
		args = append(args, to)
	}
	if categoryID != nil {
		where += " AND category_id = ?"
		args = append(args, *categoryID)
	}
	if keyword != "" {
		where += " AND (note LIKE ? OR note LIKE ?)"
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}
	if minAmount != nil {
		where += " AND amount_cents >= ?"
		args = append(args, *minAmount)
	}
	if maxAmount != nil {
		where += " AND amount_cents <= ?"
		args = append(args, *maxAmount)
	}
	if !includeVoided {
		where += " AND status = 'normal'"
	}

	// 查询总数
	var total int
	countQuery := "SELECT COUNT(*) FROM txn " + where
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计流水数失败: %w", err)
	}

	// 查询列表
	offset := (page - 1) * pageSize
	listQuery := "SELECT id, org_id, txn_date, direction, amount_cents, category_id, note, status, created_at, updated_at FROM txn " +
		where + " ORDER BY txn_date DESC, id DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, offset)

	rows, err := r.db.Query(listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询流水列表失败: %w", err)
	}
	defer rows.Close()

	var items []*Transaction
	for rows.Next() {
		t := &Transaction{}
		if err := rows.Scan(&t.ID, &t.OrgID, &t.TxnDate, &t.Direction, &t.AmountCents, &t.CategoryID, &t.Note, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("扫描流水行失败: %w", err)
		}
		items = append(items, t)
	}
	return items, total, rows.Err()
}

// GetSummary 计算某组织当前筛选条件下的收支合计。
// 与 List 口径一致：includeVoided=false 时只统计 normal；=true 时含作废流水。
func (r *Repo) GetSummary(orgID int64, from, to string, categoryID *int64, keyword string, minAmount, maxAmount *int64, includeVoided bool) (incomeTotal, expenseTotal int64, err error) {
	where := "WHERE org_id = ?"
	var args []any
	args = append(args, orgID)
	if !includeVoided {
		where += " AND status = 'normal'"
	}
	if from != "" {
		where += " AND txn_date >= ?"
		args = append(args, from)
	}
	if to != "" {
		where += " AND txn_date <= ?"
		args = append(args, to)
	}
	if categoryID != nil {
		where += " AND category_id = ?"
		args = append(args, *categoryID)
	}
	if keyword != "" {
		where += " AND note LIKE ?"
		args = append(args, "%"+keyword+"%")
	}
	if minAmount != nil {
		where += " AND amount_cents >= ?"
		args = append(args, *minAmount)
	}
	if maxAmount != nil {
		where += " AND amount_cents <= ?"
		args = append(args, *maxAmount)
	}

	query := "SELECT COALESCE(SUM(CASE WHEN direction='income' THEN amount_cents ELSE 0 END), 0), " +
		"COALESCE(SUM(CASE WHEN direction='expense' THEN amount_cents ELSE 0 END), 0) FROM txn " + where
	err = r.db.QueryRow(query, args...).Scan(&incomeTotal, &expenseTotal)
	if err != nil {
		return 0, 0, fmt.Errorf("计算收支合计失败: %w", err)
	}
	return
}

// Update 更新流水字段（限定本组织，防跨组织改写）。
func (r *Repo) Update(id, orgID int64, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	updates["updated_at"] = platform.Now()

	var setClauses []string
	var args []any
	for k, v := range updates {
		setClauses = append(setClauses, k+" = ?")
		args = append(args, v)
	}
	args = append(args, id, orgID)

	query := fmt.Sprintf("UPDATE txn SET %s WHERE id = ? AND org_id = ?", joinClauses(setClauses))
	_, err := r.db.Exec(query, args...)
	return err
}

func joinClauses(clauses []string) string {
	result := clauses[0]
	for i := 1; i < len(clauses); i++ {
		result += ", " + clauses[i]
	}
	return result
}
