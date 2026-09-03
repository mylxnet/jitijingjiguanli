// Package export 提供数据导出：流水明细 / 收支汇总 / 科目余额表，支持 CSV 与 xlsx。
// 契约见 03-design §4.3 接口 12（GET /api/export）与 PRD F5、决策 D8。
package export

import (
	"fmt"
	"strings"

	"jititaizhang/server/internal/category"
	"jititaizhang/server/internal/summary"
	"jititaizhang/server/internal/transaction"
)

// Content 导出内容类型。
type Content string

const (
	ContentTransactions Content = "transactions"   // 收支流水明细
	ContentSummary      Content = "summary"        // 区间收支汇总（分级小计 + 资金构成）
	ContentBalanceSheet Content = "balance_sheet"  // 科目余额表（D8）
)

// Format 导出格式。
type Format string

const (
	FormatCSV  Format = "csv"
	FormatXLSX Format = "xlsx"
)

// ExportQuery 导出筛选参数（与流水列表接口同口径）。
type ExportQuery struct {
	From          string
	To            string
	CategoryID    *int64
	Keyword       string
	MinAmount     *int64
	MaxAmount     *int64
	IncludeVoided bool
}

// xlCell 单元格值：string 为文本，float64 为数字（金额列，xlsx 中可求和）。
type xlCell any

// xlRow 一行。bold 用于分组行/合计行；top 用于合计行上方分隔线。
type xlRow struct {
	Cells []xlCell
	Bold  bool
	Top   bool
}

// xlSheet 一张工作表。
type xlSheet struct {
	Name     string   // 工作表名
	Widths   []float64
	NumCols  []int  // 数字右对齐 + #,##0.00 格式的列索引
	Header   []string
	Rows     []xlRow // 数据行（含分组小计行）
	FootRows []xlRow // 表尾行（资金构成 / 合计）
	// SheetTitle 工作表首行大标题（如「XX 年 X 月 收支汇总」），为空则从表头开始
	Title string
}

// money 分转元（float64 尾数足够本项目金额量级）。
func money(cents int64) float64 { return float64(cents) / 100 }

// renderer 汇聚渲染所需的数据仓库。
type renderer struct {
	sum *summary.Repo
	txn *transaction.Repo
	cat *category.Repo
}

// catNames 科目全名解析：id -> "一级/二级"（二级科目挂在父下）。
type catNames struct {
	byID  map[int64]*category.Category
	byL1  map[int64][]*category.Category // 一级 -> 其下二级（保持 sort 顺序）
}

func loadCatNames(cats []*category.Category) *catNames {
	n := &catNames{byID: make(map[int64]*category.Category), byL1: make(map[int64][]*category.Category)}
	for _, c := range cats {
		n.byID[c.ID] = c
	}
	// 按 sort_order, id 稳定排序子级
	for _, c := range cats {
		if c.Level == 2 && c.ParentID != nil {
			n.byL1[*c.ParentID] = append(n.byL1[*c.ParentID], c)
		}
	}
	return n
}

// pathOf 返回二级科目的「一级 / 二级」全名。
func (n *catNames) pathOf(catID int64) string {
	c, ok := n.byID[catID]
	if !ok {
		return fmt.Sprintf("科目#%d", catID)
	}
	if c.Level == 1 {
		return c.Name
	}
	if c.ParentID != nil {
		if p, ok := n.byID[*c.ParentID]; ok {
			return p.Name + " / " + c.Name
		}
	}
	return c.Name
}

// ===== 三种内容的数据渲染 =====

// txnSheet 收支流水明细。
func (r *renderer) txnSheet(q ExportQuery) (*xlSheet, error) {
	// 一次性全量拉取（页面按 200 条分页，导出不走分页语义；规模上限可接受）
	txns, _, err := r.txn.List(q.From, q.To, q.CategoryID, q.Keyword, q.MinAmount, q.MaxAmount, q.IncludeVoided, 1, 1000000)
	if err != nil {
		return nil, fmt.Errorf("查询流水失败: %w", err)
	}
	cats, err := r.cat.FindAll()
	if err != nil {
		return nil, fmt.Errorf("查询科目失败: %w", err)
	}
	names := loadCatNames(cats)

	incSum, expSum, err := r.txn.GetSummary(q.From, q.To, q.CategoryID, q.Keyword, q.MinAmount, q.MaxAmount, q.IncludeVoided)
	if err != nil {
		return nil, fmt.Errorf("统计收支失败: %w", err)
	}

	s := &xlSheet{
		Name:    "收支流水",
		Widths:  []float64{12, 34, 26, 14, 14, 8},
		NumCols: []int{3, 4},
		Header:  []string{"日期", "摘要", "科目", "收入（元）", "支出（元）", "状态"},
	}
	for _, t := range txns {
		var inc, exp xlCell
		if t.Direction == "income" {
			inc = money(t.AmountCents)
		} else {
			exp = money(t.AmountCents)
		}
		status := "正常"
		if t.Status == "voided" {
			status = "已作废"
		}
		note := ""
		if t.Note != nil {
			note = *t.Note
		}
		s.Rows = append(s.Rows, xlRow{Cells: []xlCell{t.TxnDate, note, names.pathOf(t.CategoryID), inc, exp, status}})
	}
	periodTitle := ""
	if q.From != "" || q.To != "" {
		periodTitle = q.From + " ~ " + q.To
	}
	s.Title = "收支流水" + withPeriod(periodTitle)
	s.FootRows = []xlRow{
		{Cells: []xlCell{"合计", "", "", money(incSum), money(expSum), ""}, Bold: true, Top: true},
	}
	return s, nil
}

// summarySheet 区间收支汇总（科目分级小计 + 资金构成）。
func (r *renderer) summarySheet(q ExportQuery) (*xlSheet, error) {
	res, err := r.sum.GetSummary(q.From, q.To)
	if err != nil {
		return nil, fmt.Errorf("查询汇总失败: %w", err)
	}

	s := &xlSheet{
		Name:    "收支汇总",
		Widths:  []float64{40, 16, 16, 16},
		NumCols: []int{1, 2, 3},
		Header:  []string{"科目", "收入（元）", "支出（元）", "结余（元）"},
	}

	// 一级行 = 小计（加粗），二级行 = 明细
	for _, l1 := range res.Categories {
		s.Rows = append(s.Rows, xlRow{
			Cells: []xlCell{l1.Name, money(l1.IncomeCents), money(l1.ExpenseCents), money(l1.IncomeCents - l1.ExpenseCents)},
			Bold:  true,
		})
		for _, l2 := range l1.Children {
			s.Rows = append(s.Rows, xlRow{
				Cells: []xlCell{"　" + l2.Name, money(l2.IncomeCents), money(l2.ExpenseCents), money(l2.IncomeCents - l2.ExpenseCents)},
			})
		}
	}
	s.FootRows = []xlRow{
		{Cells: []xlCell{"合计", money(res.IncomeTotal), money(res.ExpenseTotal), money(res.Balance)}, Bold: true, Top: true},
	}
	// 资金构成表尾（D6）
	if res.Capital != nil {
		s.FootRows = append(s.FootRows,
			xlRow{Cells: []xlCell{"银行存款余额", money(res.Capital.BankBalanceCents), "", ""}},
			xlRow{Cells: []xlCell{"专项资金合计", money(res.Capital.EarmarkedCents), "", ""}},
			xlRow{Cells: []xlCell{"未分配资金", money(res.Capital.UnallocatedCents), "", ""}},
		)
		if res.Capital.Warning != "" {
			s.FootRows = append(s.FootRows, xlRow{Cells: []xlCell{"⚠ " + res.Capital.Warning, "", "", ""}})
		}
	}
	s.Title = "收支汇总" + withPeriod(periodOf(q))
	return s, nil
}

// balanceSheet 科目余额表（D8）：按一级分组列二级的期初/当前余额，一级合计，表尾资金构成。
func (r *renderer) balanceSheet(q ExportQuery) (*xlSheet, error) {
	res, err := r.sum.GetSummary("", "")
	if err != nil {
		return nil, fmt.Errorf("查询科目余额失败: %w", err)
	}

	s := &xlSheet{
		Name:    "科目余额表",
		Widths:  []float64{40, 16, 16, 10},
		NumCols: []int{1, 2},
		Header:  []string{"科目", "期初余额（元）", "当前余额（元）", "类型"},
	}
	// 按一级分组：先一级合计行，再其下二级行
	for _, l1 := range res.Categories {
		var l1Open int64
		rows := []xlRow{}
		for _, l2 := range l1.Children {
			l1Open += l2.OpeningBalanceCents
			typeName := "余粮"
			if l2.BalanceType == "spending" {
				typeName = "花费"
			}
			rows = append(rows, xlRow{Cells: []xlCell{
				"　" + l2.Name, money(l2.OpeningBalanceCents), money(l2.CurrentBalanceCents), typeName,
			}})
		}
		// 一级自身余额 = Σ子级（余额型科目树口径，见 D6）
		s.Rows = append(s.Rows, xlRow{
			Cells: []xlCell{l1.Name, money(l1.OpeningBalanceCents), money(l1.CurrentBalanceCents), ""},
			Bold:  true,
		})
		_ = l1Open
		s.Rows = append(s.Rows, rows...)
	}
	s.FootRows = []xlRow{}
	if res.Capital != nil {
		s.FootRows = append(s.FootRows,
			xlRow{Cells: []xlCell{"银行存款余额", money(res.Capital.BankBalanceCents), "", ""}, Bold: true, Top: true},
			xlRow{Cells: []xlCell{"专项资金合计", money(res.Capital.EarmarkedCents), "", ""}},
			xlRow{Cells: []xlCell{"未分配资金", money(res.Capital.UnallocatedCents), "", ""}},
		)
	}
	s.Title = "科目余额表"
	return s, nil
}

func withPeriod(p string) string {
	if p == "" {
		return ""
	}
	return "（" + p + "）"
}

func periodOf(q ExportQuery) string {
	if q.From == "" && q.To == "" {
		return ""
	}
	if q.From == q.To {
		return q.From
	}
	return strings.TrimSpace(q.From + " ~ " + q.To)
}
