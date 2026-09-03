// Package summary 提供汇总查询：资金构成、科目余额树、区间收支小计。
//
// v0.4 口径（到账才算收益）：
//   - 银行存款 = 银行期初 + Σ收 − Σ支 ± 资金划转（全年 normal）
//   - 资产类合计 = Σ(资产二级余额)（投资 − 收回）
//   - 权益类合计 = Σ(权益二级余额)（各科目收入−支出+转账净额），即净资产
//   - 往来应收欠款不并入本汇总（单独在往来查看）
package summary

import (
	"database/sql"
	"fmt"

	"jititaizhang/server/internal/category"
	"jititaizhang/server/internal/settings"
)

// Repo 封装汇总查询 SQL。
type Repo struct {
	db      *sql.DB
	setting *settings.Repo
	catRepo *category.Repo
}

// NewRepo 创建 Repo。
func NewRepo(db *sql.DB) *Repo {
	return &Repo{
		db:      db,
		setting: settings.NewRepo(db),
		catRepo: category.NewRepo(db),
	}
}

// catPeriodStat 是某科目在区间内的收支发生额统计。
type catPeriodStat struct {
	income  int64
	expense int64
	txn     int64
}

// GetSummary 计算某组织汇总。from/to 空串表示不限。
func (r *Repo) GetSummary(orgID int64, from, to string) (*SummaryResponse, error) {
	set, err := r.setting.Get(orgID)
	if err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}

	incomeTotal, expenseTotal, err := r.calcIncomeExpense(orgID, from, to)
	if err != nil {
		return nil, fmt.Errorf("计算收支合计失败: %w", err)
	}

	capital, err := r.calcCapital(orgID, set.BankOpeningBalanceCents)
	if err != nil {
		return nil, fmt.Errorf("计算资金构成失败: %w", err)
	}

	categories, err := r.calcCategoryTree(orgID, from, to)
	if err != nil {
		return nil, fmt.Errorf("计算科目余额失败: %w", err)
	}

	return &SummaryResponse{
		IncomeTotal:  incomeTotal,
		ExpenseTotal: expenseTotal,
		Balance:      incomeTotal - expenseTotal,
		Capital:      capital,
		Categories:   categories,
	}, nil
}

// calcIncomeExpense 计算某组织区间内收支合计（normal）。
func (r *Repo) calcIncomeExpense(orgID int64, from, to string) (int64, int64, error) {
	where := "WHERE org_id = ? AND status = 'normal'"
	args := []any{orgID}
	if from != "" {
		where += " AND txn_date >= ?"
		args = append(args, from)
	}
	if to != "" {
		where += " AND txn_date <= ?"
		args = append(args, to)
	}

	var income, expense int64
	err := r.db.QueryRow(`SELECT
		COALESCE(SUM(CASE WHEN direction='income' THEN amount_cents ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN direction='expense' THEN amount_cents ELSE 0 END),0)
		FROM txn `+where, args...).Scan(&income, &expense)
	if err != nil {
		return 0, 0, fmt.Errorf("汇总收支失败: %w", err)
	}
	return income, expense, nil
}

// calcCapital 计算资金构成（v0.4）。
func (r *Repo) calcCapital(orgID int64, bankOpening int64) (*Capital, error) {
	var totalIncome, totalExpense int64
	if err := r.db.QueryRow(`SELECT
		COALESCE(SUM(CASE WHEN direction='income' THEN amount_cents ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN direction='expense' THEN amount_cents ELSE 0 END),0)
		FROM txn WHERE org_id = ? AND status='normal'`, orgID,
	).Scan(&totalIncome, &totalExpense); err != nil {
		return nil, fmt.Errorf("汇总全年收支失败: %w", err)
	}

	moveDelta, err := r.catRepo.BankDelta(orgID)
	if err != nil {
		return nil, fmt.Errorf("汇总资金划转影响失败: %w", err)
	}
	bank := bankOpening + totalIncome - totalExpense + moveDelta

	cats, err := r.catRepo.FindAll(orgID)
	if err != nil {
		return nil, fmt.Errorf("查询科目失败: %w", err)
	}
	var assetTotal, equityTotal int64
	for _, cat := range cats {
		if cat.Level != 2 {
			continue
		}
		var bal int64
		if cat.Kind == "asset" {
			bal, err = r.catRepo.AssetBalance(cat.ID)
		} else {
			bal, err = r.catRepo.CalcBalance(cat.ID)
		}
		if err != nil {
			return nil, fmt.Errorf("计算科目 %d 余额失败: %w", cat.ID, err)
		}
		if cat.Kind == "asset" {
			assetTotal += bal
		} else {
			equityTotal += bal
		}
	}

	c := &Capital{
		BankBalanceCents: bank,
		AssetTotalCents:  assetTotal,
		EquityTotalCents: equityTotal,
	}
	if equityTotal < 0 {
		c.Warning = "权益类科目合计为负，请检查收入/支出是否记错方向"
	}
	return c, nil
}

// calcCategoryTree 计算科目余额树（当前余额为全年累计；发生额为区间内）。
func (r *Repo) calcCategoryTree(orgID int64, from, to string) ([]*CategorySummary, error) {
	cats, err := r.catRepo.FindAll(orgID)
	if err != nil {
		return nil, err
	}

	stats, err := r.periodStats(orgID, from, to)
	if err != nil {
		return nil, err
	}

	summaries := make([]*CategorySummary, 0, len(cats))
	byID := make(map[int64]*CategorySummary)

	for _, cat := range cats {
		var bal int64
		if cat.Kind == "asset" {
			bal, err = r.catRepo.AssetBalance(cat.ID)
		} else {
			bal, err = r.catRepo.CalcBalance(cat.ID)
		}
		if err != nil {
			return nil, fmt.Errorf("计算科目 %d 余额失败: %w", cat.ID, err)
		}
		st := stats[cat.ID]
		if st == nil {
			st = &catPeriodStat{}
		}
		s := &CategorySummary{
			ID:                  cat.ID,
			Name:                cat.Name,
			Level:               cat.Level,
			ParentID:            cat.ParentID,
			Kind:                cat.Kind,
			CurrentBalanceCents: bal,
			IncomeCents:         st.income,
			ExpenseCents:        st.expense,
			TxnCount:            int(st.txn),
		}
		byID[cat.ID] = s
		if cat.Level == 1 {
			summaries = append(summaries, s)
		}
	}

	for _, cat := range cats {
		if cat.Level == 2 && cat.ParentID != nil {
			if parent, ok := byID[*cat.ParentID]; ok {
				parent.Children = append(parent.Children, byID[cat.ID])
			}
		}
	}

	for _, root := range summaries {
		for _, child := range root.Children {
			root.CurrentBalanceCents += child.CurrentBalanceCents
			root.IncomeCents += child.IncomeCents
			root.ExpenseCents += child.ExpenseCents
			root.TxnCount += child.TxnCount
		}
	}

	return summaries, nil
}

// periodStats 汇总某组织各二级科目在区间内的发生额（一次 GROUP BY 完成）。
func (r *Repo) periodStats(orgID int64, from, to string) (map[int64]*catPeriodStat, error) {
	where := "WHERE org_id = ? AND status='normal'"
	args := []any{orgID}
	if from != "" {
		where += " AND txn_date >= ?"
		args = append(args, from)
	}
	if to != "" {
		where += " AND txn_date <= ?"
		args = append(args, to)
	}

	rows, err := r.db.Query(`SELECT category_id,
		COALESCE(SUM(CASE WHEN direction='income' THEN amount_cents ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN direction='expense' THEN amount_cents ELSE 0 END),0),
		COUNT(*) FROM txn `+where+` GROUP BY category_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("汇总科目发生额失败: %w", err)
	}
	defer rows.Close()

	stats := make(map[int64]*catPeriodStat)
	for rows.Next() {
		var catID int64
		st := &catPeriodStat{}
		if err := rows.Scan(&catID, &st.income, &st.expense, &st.txn); err != nil {
			return nil, fmt.Errorf("扫描科目发生额失败: %w", err)
		}
		stats[catID] = st
	}
	return stats, rows.Err()
}
