// Package summary 提供汇总查询：资金构成、科目余额、收支小计。
package summary

// Capital 资金构成（见 D6）。
type Capital struct {
	BankBalanceCents    int64 `json:"bankBalanceCents"`    // 银行存款余额 = 期初 + 收 - 支
	EarmarkedCents      int64 `json:"earmarkedCents"`      // 专项资金合计 = Σ(参与勾稽科目余额)
	UnallocatedCents    int64 `json:"unallocatedCents"`    // 未分配 = bank - earmarked
	Warning             string `json:"warning,omitempty"`    // 警告（未分配为负时）
}

// CategorySummary 单个科目汇总。
type CategorySummary struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	Level             int    `json:"level"`
	ParentID          *int64 `json:"parentId,omitempty"`
	BalanceType       string `json:"balanceType"` // residual / spending
	OpeningBalanceCents int64 `json:"openingBalanceCents"`
	IncludeInReconciliation bool `json:"includeInReconciliation"`
	CurrentBalanceCents int64 `json:"currentBalanceCents"` // 当前余额（实时计算）
	TxnCount           int   `json:"txnCount"`           // 流水笔数
	IncomeCents        int64 `json:"incomeCents"`        // 区间内收入
	ExpenseCents       int64 `json:"expenseCents"`       // 区间内支出
	Children           []*CategorySummary `json:"children,omitempty"`
}

// SummaryResponse 汇总响应。
type SummaryResponse struct {
	IncomeTotal  int64              `json:"incomeTotal"`  // 区间总收入
	ExpenseTotal int64              `json:"expenseTotal"` // 区间总支出
	Balance      int64              `json:"balance"`      // 结余 = 收入 - 支出
	Capital      *Capital           `json:"capital"`      // 资金构成
	Categories   []*CategorySummary `json:"categories"`   // 一级科目列表（含子科目）
}