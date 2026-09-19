// Package summary 提供汇总查询：资金构成、科目余额、收支小计。
package summary

// Capital 资金构成（v0.4：资产/权益两类；到账才算收益）。
type Capital struct {
	BankBalanceCents int64  `json:"bankBalanceCents"` // 银行存款余额 = 期初 + Σ收 − Σ支 ± 资金划转
	AssetTotalCents  int64  `json:"assetTotalCents"`  // 资产类科目余额合计（对外投资等）
	EquityTotalCents int64  `json:"equityTotalCents"` // 权益类科目余额合计（净资产；收付实现）
	Warning          string `json:"warning,omitempty"`
}

// Slice 环形构成分项（name + value，value 单位分）。
type Slice struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

// Composition 看板四块环形构成（v0.21 看板改造）。
// 科目一律按名称动态解析（科目 ID 随组织注册变化），欠款按 recv_kind 分组。
type Composition struct {
	Fund    []Slice `json:"fund"`    // 资金构成：银行存款 / 长期投资 / 应收收益（土地流转费、管理费欠款不计入）。
	Invest  []Slice `json:"invest"`  // 在外投资构成：长期投资 / 再投资 / 其他（资产类科目按一级名分组）。
	Owe     []Slice `json:"owe"`     // 欠款构成：土地流转费 / 流转管理费 / 应收收益（按 recv_kind 分组 outstanding）。
	Expense []Slice `json:"expense"` // 可支出构成：公益支出科目余额。
}

// CategorySummary 单个科目汇总。
type CategorySummary struct {
	ID                  int64              `json:"id"`
	Name                string             `json:"name"`
	Level               int                `json:"level"`
	ParentID            *int64             `json:"parentId,omitempty"`
	Kind                string             `json:"kind"` // asset 资产 / equity 权益（v0.4）
	CurrentBalanceCents int64              `json:"currentBalanceCents"`
	TxnCount            int                `json:"txnCount"`
	IncomeCents         int64              `json:"incomeCents"`
	ExpenseCents        int64              `json:"expenseCents"`
	Children            []*CategorySummary `json:"children,omitempty"`
}

// SummaryResponse 汇总响应。
type SummaryResponse struct {
	IncomeTotal  int64              `json:"incomeTotal"`
	ExpenseTotal int64              `json:"expenseTotal"`
	Balance      int64              `json:"balance"`
	Capital      *Capital           `json:"capital"`
	Composition  *Composition       `json:"composition"`
	Categories   []*CategorySummary `json:"categories"`
}
