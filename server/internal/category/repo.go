package category

import (
	"database/sql"
	"fmt"

	"jititaizhang/server/internal/platform"
)

// Repo 封装 category 表的 SQL 查询。
type Repo struct {
	db *sql.DB
}

// NewRepo 创建 Repo。
func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// FindAll 查询所有科目，按 sort_order 排序。
func (r *Repo) FindAll() ([]*Category, error) {
	rows, err := r.db.Query(
		`SELECT id, name, level, parent_id, status, balance_type,
		        opening_balance_cents, include_in_reconciliation,
		        sort_order, created_at, updated_at
		 FROM category ORDER BY sort_order, id`)
	if err != nil {
		return nil, fmt.Errorf("查询科目列表失败: %w", err)
	}
	defer rows.Close()

	var cats []*Category
	for rows.Next() {
		c := &Category{}
		var inc int
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Level, &c.ParentID, &c.Status,
			&c.BalanceType, &c.OpeningBalanceCents, &inc,
			&c.SortOrder, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描科目行失败: %w", err)
		}
		c.IncludeInReconciliation = inc == 1
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

// FindByID 按 ID 查询科目。
func (r *Repo) FindByID(id int64) (*Category, error) {
	c := &Category{}
	var inc int
	err := r.db.QueryRow(
		`SELECT id, name, level, parent_id, status, balance_type,
		        opening_balance_cents, include_in_reconciliation,
		        sort_order, created_at, updated_at
		 FROM category WHERE id = ?`, id,
	).Scan(&c.ID, &c.Name, &c.Level, &c.ParentID, &c.Status,
		&c.BalanceType, &c.OpeningBalanceCents, &inc,
		&c.SortOrder, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询科目失败: %w", err)
	}
	c.IncludeInReconciliation = inc == 1
	return c, nil
}

// FindActiveLevel2 查询所有启用中的二级科目。
func (r *Repo) FindActiveLevel2() ([]*Category, error) {
	rows, err := r.db.Query(
		`SELECT id, name, level, parent_id, status, balance_type,
		        opening_balance_cents, include_in_reconciliation,
		        sort_order, created_at, updated_at
		 FROM category WHERE level = 2 AND status = 'active' ORDER BY sort_order, id`)
	if err != nil {
		return nil, fmt.Errorf("查询启用中二级科目失败: %w", err)
	}
	defer rows.Close()

	var cats []*Category
	for rows.Next() {
		c := &Category{}
		var inc int
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Level, &c.ParentID, &c.Status,
			&c.BalanceType, &c.OpeningBalanceCents, &inc,
			&c.SortOrder, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描科目行失败: %w", err)
		}
		c.IncludeInReconciliation = inc == 1
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

// Create 新建科目。
func (r *Repo) Create(c *Category) (*Category, error) {
	now := platform.Now()
	inc := 0
	if c.IncludeInReconciliation {
		inc = 1
	}
	res, err := r.db.Exec(
		`INSERT INTO category(name, level, parent_id, status, balance_type,
		                      opening_balance_cents, include_in_reconciliation,
		                      sort_order, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Name, c.Level, c.ParentID, c.Status, c.BalanceType,
		c.OpeningBalanceCents, inc, c.SortOrder, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("新建科目失败: %w", err)
	}
	id, _ := res.LastInsertId()
	c.ID = id
	c.CreatedAt = now
	c.UpdatedAt = now
	return c, nil
}

// Update 更新科目字段。
func (r *Repo) Update(id int64, updates map[string]any) error {
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
	args = append(args, id)

	query := fmt.Sprintf("UPDATE category SET %s WHERE id = ?", joinClauses(setClauses))
	_, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("更新科目失败: %w", err)
	}
	return nil
}

// Delete 物理删除科目。
func (r *Repo) Delete(id int64) error {
	_, err := r.db.Exec(`DELETE FROM category WHERE id = ?`, id)
	return err
}

// CountChildren 统计一级科目下的二级科目数。
func (r *Repo) CountChildren(parentID int64) (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM category WHERE parent_id = ?`, parentID).Scan(&n)
	return n, err
}

// CountTransactions 统计引用该科目的流水数。
func (r *Repo) CountTransactions(categoryID int64) (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM txn WHERE category_id = ?`, categoryID).Scan(&n)
	return n, err
}

// IsNameDup 检查同级是否重名。
func (r *Repo) IsNameDup(name string, parentID *int64, excludeID int64) (bool, error) {
	var n int
	var err error
	if parentID == nil {
		// 一级科目：parent_id IS NULL
		err = r.db.QueryRow(
			`SELECT COUNT(*) FROM category WHERE name = ? AND parent_id IS NULL AND id != ?`,
			name, excludeID,
		).Scan(&n)
	} else {
		err = r.db.QueryRow(
			`SELECT COUNT(*) FROM category WHERE name = ? AND parent_id = ? AND id != ?`,
			name, *parentID, excludeID,
		).Scan(&n)
	}
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// CalcBalance 计算科目当前余额（实时聚合，不落库）。
func (r *Repo) CalcBalance(catID int64) (int64, error) {
	var opening int64
	var balanceType string
	err := r.db.QueryRow(`SELECT opening_balance_cents, balance_type FROM category WHERE id = ?`, catID).
		Scan(&opening, &balanceType)
	if err != nil {
		return 0, err
	}

	// 收支
	var inc, exp int64
	if err := r.db.QueryRow(`SELECT COALESCE(SUM(CASE WHEN direction='income' THEN amount_cents ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN direction='expense' THEN amount_cents ELSE 0 END), 0)
		FROM txn WHERE category_id = ? AND status = 'normal'`, catID).Scan(&inc, &exp); err != nil {
		return 0, fmt.Errorf("聚合收支失败: %w", err)
	}

	var bal int64
	if balanceType == "residual" {
		bal = opening + inc - exp
	} else {
		bal = opening + exp - inc
	}

	// 转出
	var out int64
	if err := r.db.QueryRow(`SELECT COALESCE(SUM(source_amount_cents), 0) FROM transfer WHERE source_category_id = ? AND status = 'normal'`, catID).Scan(&out); err != nil {
		return 0, fmt.Errorf("聚合转出失败: %w", err)
	}
	bal -= out

	// 转入
	var in int64
	if err := r.db.QueryRow(`SELECT COALESCE(SUM(leg.amount_cents), 0) FROM transfer_leg leg
		JOIN transfer t ON leg.transfer_id = t.id
		WHERE leg.category_id = ? AND t.status = 'normal'`, catID).Scan(&in); err != nil {
		return 0, fmt.Errorf("聚合转入失败: %w", err)
	}
	bal += in

	return bal, nil
}

func joinClauses(clauses []string) string {
	result := clauses[0]
	for i := 1; i < len(clauses); i++ {
		result += ", " + clauses[i]
	}
	return result
}