// Package summary 提供汇总查询：资金构成、科目余额树、区间收支小计。
//
// 口径约定（见 03-design §5.7 / 需求 D6、D7）：
//   - 区间收支小计：仅 status='normal' 的 txn，按 from/to 过滤（空=不限）
//   - 资金构成与科目余额：全年累计（不随区间走），银行存款/勾稽科目均只计 normal
//   - 勾稽口径：include_in_reconciliation=1 的科目**不论停用与否**都参与专项资金合计
//     （停用只是不再录入，已占用的资金仍在）
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

// GetSummary 计算汇总。from/to 空串表示不限。
func (r *Repo) GetSummary(from, to string) (*SummaryResponse, error) {
	set, err := r.setting.Get()
	if err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}

	incomeTotal, expenseTotal, err := r.calcIncomeExpense(from, to)
	if err != nil {
		return nil, fmt.Errorf("计算收支合计失败: %w", err)
	}

	capital, err := r.calcCapital(set.BankOpeningBalanceCents)
	if err != nil {
		return nil, fmt.Errorf("计算资金构成失败: %w", err)
	}

	categories, err := r.calcCategoryTree(from, to)
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

// calcIncomeExpense 计算区间内收支合计。
func (r *Repo) calcIncomeExpense(from, to string) (int64, int64, error) {
	where := "WHERE status = 'normal'"
	var args []any
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

// calcCapital 计算资金构成（D6 恒等式）：
//
//	bankBalance = bankOpening + Σ收 − Σ支（全年）
//	earmarked   = Σ(参与勾稽科目余额)（含停用）
//	unallocated = bankBalance − earmarked
func (r *Repo) calcCapital(bankOpening int64) (*Capital, error) {
	var totalIncome, totalExpense int64
	err := r.db.QueryRow(`SELECT
		COALESCE(SUM(CASE WHEN direction='income' THEN amount_cents ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN direction='expense' THEN amount_cents ELSE 0 END),0)
		FROM txn WHERE status='normal'`).Scan(&totalIncome, &totalExpense)
	if err != nil {
		return nil, fmt.Errorf("汇总全年收支失败: %w", err)
	}

	bank := bankOpening + totalIncome - totalExpense

	// 先收齐勾稽科目 id 并关闭游标，再逐个算余额
	// （Open 设置了 MaxOpenConns(1)，游标未关时嵌套查询会死锁）
	rows, err := r.db.Query(`SELECT id FROM category WHERE include_in_reconciliation = 1`)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var catID int64
		if err := rows.Scan(&catID); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, catID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var earmarked int64
	for _, catID := range ids {
		bal, err := r.catRepo.CalcBalance(catID)
		if err != nil {
			return nil, fmt.Errorf("计算勾稽科目余额失败: %w", err)
		}
		earmarked += bal
	}

	c := &Capital{BankBalanceCents: bank, EarmarkedCents: earmarked}
	c.UnallocatedCents = bank - earmarked
	if c.UnallocatedCents < 0 {
		c.Warning = fmt.Sprintf("专项资金合计 (%d.%02d) 已超过银行存款余额 (%d.%02d)，请检查科目期初余额设置",
			earmarked/100, earmarked%100, bank/100, bank%100)
	}
	return c, nil
}

// calcCategoryTree 计算科目余额树（当前余额为全年累计；发生额为区间内）：
// 一级行的余额与发生额为子科目之和。
func (r *Repo) calcCategoryTree(from, to string) ([]*CategorySummary, error) {
	cats, err := r.catRepo.FindAll()
	if err != nil {
		return nil, err
	}

	stats, err := r.periodStats(from, to)
	if err != nil {
		return nil, err
	}

	summaries := make([]*CategorySummary, 0, len(cats))
	byID := make(map[int64]*CategorySummary)

	for _, cat := range cats {
		bal, err := r.catRepo.CalcBalance(cat.ID)
		if err != nil {
			return nil, fmt.Errorf("计算科目 %d 余额失败: %w", cat.ID, err)
		}
		st := stats[cat.ID]
		if st == nil {
			st = &catPeriodStat{} // 无流水科目 → 零发生额
		}
		s := &CategorySummary{
			ID:                      cat.ID,
			Name:                    cat.Name,
			Level:                   cat.Level,
			ParentID:                cat.ParentID,
			BalanceType:             cat.BalanceType,
			OpeningBalanceCents:     cat.OpeningBalanceCents,
			IncludeInReconciliation: cat.IncludeInReconciliation,
			CurrentBalanceCents:     bal,
			IncomeCents:             st.income,
			ExpenseCents:            st.expense,
			TxnCount:                int(st.txn),
		}
		byID[cat.ID] = s
		if cat.Level == 1 {
			summaries = append(summaries, s)
		}
	}

	// 挂载二级到一级
	for _, cat := range cats {
		if cat.Level == 2 && cat.ParentID != nil {
			if parent, ok := byID[*cat.ParentID]; ok {
				parent.Children = append(parent.Children, byID[cat.ID])
			}
		}
	}

	// 一级 = 子科目之和（含可能挂在自身的零额）
	for _, root := range summaries {
		for _, child := range root.Children {
			root.CurrentBalanceCents += child.CurrentBalanceCents
			root.OpeningBalanceCents += child.OpeningBalanceCents
			root.IncomeCents += child.IncomeCents
			root.ExpenseCents += child.ExpenseCents
			root.TxnCount += child.TxnCount
		}
	}

	return summaries, nil
}

// periodStats 汇总各二级科目在区间内的发生额（一次 GROUP BY 完成）。
func (r *Repo) periodStats(from, to string) (map[int64]*catPeriodStat, error) {
	where := "WHERE status='normal'"
	var args []any
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
