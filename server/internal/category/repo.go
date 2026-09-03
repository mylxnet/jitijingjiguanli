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

const catCols = `id, org_id, name, level, parent_id, status, balance_type, kind,
	opening_balance_cents, include_in_reconciliation, preset,
	sort_order, created_at, updated_at`

func scanCat(row interface{ Scan(...any) error }) (*Category, error) {
	c := &Category{}
	var inc, preset int
	err := row.Scan(
		&c.ID, &c.OrgID, &c.Name, &c.Level, &c.ParentID, &c.Status,
		&c.BalanceType, &c.Kind, &c.OpeningBalanceCents, &inc, &preset,
		&c.SortOrder, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	c.IncludeInReconciliation = inc == 1
	c.Preset = preset == 1
	return c, nil
}

// FindAll 查询某组织的全部科目（含资产型），按 sort_order 排序。
func (r *Repo) FindAll(orgID int64) ([]*Category, error) {
	rows, err := r.db.Query(
		`SELECT `+catCols+` FROM category WHERE org_id = ? ORDER BY sort_order, id`, orgID)
	if err != nil {
		return nil, fmt.Errorf("查询科目列表失败: %w", err)
	}
	defer rows.Close()

	var cats []*Category
	for rows.Next() {
		c, err := scanCat(rows)
		if err != nil {
			return nil, fmt.Errorf("扫描科目行失败: %w", err)
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

// FindByID 按 ID 查询科目（调用方需校验 OrgID 归属）。
func (r *Repo) FindByID(id int64) (*Category, error) {
	row := r.db.QueryRow(`SELECT `+catCols+` FROM category WHERE id = ?`, id)
	c, err := scanCat(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询科目失败: %w", err)
	}
	return c, nil
}

// FindActiveLevel2 查询某组织启用中的普通二级科目（收支/转账可挂）。
func (r *Repo) FindActiveLevel2(orgID int64) ([]*Category, error) {
	rows, err := r.db.Query(
		`SELECT `+catCols+` FROM category
		 WHERE org_id = ? AND level = 2 AND status = 'active' AND kind = 'normal'
		 ORDER BY sort_order, id`, orgID)
	if err != nil {
		return nil, fmt.Errorf("查询启用中二级科目失败: %w", err)
	}
	defer rows.Close()

	var cats []*Category
	for rows.Next() {
		c, err := scanCat(rows)
		if err != nil {
			return nil, fmt.Errorf("扫描科目行失败: %w", err)
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

// FindAssetLevel2 查询某组织启用中的资产型二级科目（资金划转可挂，见 D10）。
func (r *Repo) FindAssetLevel2(orgID int64) ([]*Category, error) {
	rows, err := r.db.Query(
		`SELECT `+catCols+` FROM category
		 WHERE org_id = ? AND level = 2 AND status = 'active' AND kind = 'asset'
		 ORDER BY sort_order, id`, orgID)
	if err != nil {
		return nil, fmt.Errorf("查询资产科目失败: %w", err)
	}
	defer rows.Close()

	var cats []*Category
	for rows.Next() {
		c, err := scanCat(rows)
		if err != nil {
			return nil, fmt.Errorf("扫描资产科目失败: %w", err)
		}
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
	preset := 0
	if c.Preset {
		preset = 1
	}
	res, err := r.db.Exec(
		`INSERT INTO category(org_id, name, level, parent_id, status, balance_type, kind,
			opening_balance_cents, include_in_reconciliation, preset,
			sort_order, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.OrgID, c.Name, c.Level, c.ParentID, c.Status, c.BalanceType, c.Kind,
		c.OpeningBalanceCents, inc, preset, c.SortOrder, now, now,
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

// Update 更新科目字段（限定本组织，防跨组织改写）。
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

	query := fmt.Sprintf("UPDATE category SET %s WHERE id = ? AND org_id = ?", joinClauses(setClauses))
	_, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("更新科目失败: %w", err)
	}
	return nil
}

// Delete 物理删除科目（限定本组织）。
func (r *Repo) Delete(id, orgID int64) error {
	_, err := r.db.Exec(`DELETE FROM category WHERE id = ? AND org_id = ?`, id, orgID)
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

// CountFundMoves 统计引用该科目的资金划转数（资产科目删除保护，D10/D0）。
func (r *Repo) CountFundMoves(categoryID int64) (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM fund_move WHERE asset_category_id = ?`, categoryID).Scan(&n)
	return n, err
}

// IsNameDup 检查同组织同级是否重名。
func (r *Repo) IsNameDup(orgID int64, name string, parentID *int64, excludeID int64) (bool, error) {
	var n int
	var err error
	if parentID == nil {
		err = r.db.QueryRow(
			`SELECT COUNT(*) FROM category WHERE org_id = ? AND name = ? AND parent_id IS NULL AND id != ?`,
			orgID, name, excludeID,
		).Scan(&n)
	} else {
		err = r.db.QueryRow(
			`SELECT COUNT(*) FROM category WHERE org_id = ? AND name = ? AND parent_id = ? AND id != ?`,
			orgID, name, *parentID, excludeID,
		).Scan(&n)
	}
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// CalcBalance 计算科目当前余额（实时聚合，不落库）。
// 普通科目：期初 + 收支（方向按类型）+ 转入 − 转出；资产科目不走此函数（余额见 fund_move 聚合）。
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

// AssetBalance 计算资产科目余额 = Σ投资 − Σ收回（D10；fund_move 聚合）。
func (r *Repo) AssetBalance(catID int64) (int64, error) {
	var out, in int64
	if err := r.db.QueryRow(
		`SELECT COALESCE(SUM(CASE WHEN kind='invest' THEN amount_cents ELSE 0 END), 0),
		        COALESCE(SUM(CASE WHEN kind='recover' THEN amount_cents ELSE 0 END), 0)
		 FROM fund_move WHERE asset_category_id = ? AND status = 'normal'`, catID,
	).Scan(&out, &in); err != nil {
		return 0, fmt.Errorf("聚合资产余额失败: %w", err)
	}
	return out - in, nil
}

// BankDelta 资金划转对银行存款的净影响 = Σ收回 − Σ投资（供资金构成口径，D10）。
func (r *Repo) BankDelta(orgID int64) (int64, error) {
	var delta int64
	if err := r.db.QueryRow(
		`SELECT COALESCE(SUM(CASE WHEN kind='recover' THEN amount_cents
		                          WHEN kind='invest' THEN -amount_cents ELSE 0 END), 0)
		 FROM fund_move WHERE org_id = ? AND status = 'normal'`, orgID,
	).Scan(&delta); err != nil {
		return 0, fmt.Errorf("聚合资金划转影响失败: %w", err)
	}
	return delta, nil
}

func joinClauses(clauses []string) string {
	result := clauses[0]
	for i := 1; i < len(clauses); i++ {
		result += ", " + clauses[i]
	}
	return result
}
