package changelog

import "strconv"

// ChangeLog 对应 change_log 表。
type ChangeLog struct {
	ID         int64   `json:"id"`
	OrgID      int64   `json:"orgId"`
	EntityType string  `json:"entityType"`
	EntityID   int64   `json:"entityId"`
	Action     string  `json:"action"`
	Field      *string `json:"field"`
	OldValue   *string `json:"oldValue"`
	NewValue   *string `json:"newValue"`
	ChangedAt  string  `json:"changedAt"`
}

// OpLogEffect 单条影响明细（前端 OperationLogDialog 消费）。
type OpLogEffect struct {
	Entity   string  `json:"entity"`
	EntityID int64   `json:"entityId,omitempty"`
	Field    *string `json:"field,omitempty"`
	OldValue *string `json:"oldValue,omitempty"`
	NewValue *string `json:"newValue,omitempty"`
	Desc     string  `json:"desc"`
}

// OpLog 操作日志条目：0..1 条 change_log 映射为 1 个条目 + 1 个 effect。
type OpLog struct {
	ID        int64         `json:"id"`
	Time      string        `json:"time"`
	Operation string        `json:"operation"`
	Summary   string        `json:"summary"`
	Effects   []OpLogEffect `json:"effects"`
}

// entityName 实体类型 → 中文名。
func entityName(t string) string {
	switch t {
	case "transaction", "txn":
		return "流水"
	case "transfer":
		return "资金划转"
	case "fund_move":
		return "资金划转"
	case "category":
		return "科目"
	case "party":
		return "往来单位"
	case "receivable":
		return "应收单"
	case "receipt":
		return "收款单"
	case "contract":
		return "合同附件"
	case "reinvest_allocation":
		return "再投资去向"
	case "user":
		return "账号"
	case "org":
		return "组织"
	default:
		if t == "" {
			return "数据"
		}
		return t
	}
}

// actionName 操作 → 中文名。
func actionName(a string) string {
	switch a {
	case "create":
		return "新建"
	case "update":
		return "修改"
	case "void":
		return "作废"
	case "unvoid":
		return "恢复"
	case "delete":
		return "删除"
	default:
		if a == "" {
			return "操作"
		}
		return a
	}
}

// fieldName 字段名 → 中文名。
func fieldName(f string) string {
	switch f {
	case "status":
		return "状态"
	case "name":
		return "名称"
	case "type":
		return "类型"
	case "note":
		return "备注"
	case "amount_cents":
		return "金额"
	case "category_id":
		return "科目"
	case "direction":
		return "收支方向"
	case "txn_date":
		return "日期"
	case "opening_balance_cents":
		return "期初余额"
	default:
		return f
	}
}

// toOpLog 将一条 change_log 映射为操作日志条目（1 行 → 1 条目 + 1 effect）。
// business 为联表取到的中文业务描述（单位名/标题/金额等），空则回落实体名。
func toOpLog(cl ChangeLog, business string) OpLog {
	entity := entityName(cl.EntityType)
	action := actionName(cl.Action)

	var field string
	var fieldPtr *string
	if cl.Field != nil {
		field = fieldName(*cl.Field)
		fieldPtr = &field
	}

	summary := entity
	if business != "" {
		summary += "「" + business + "」"
	} else if cl.EntityID > 0 {
		summary += " #" + itoa(cl.EntityID)
	}
	summary += " " + action
	if field != "" {
		summary += "（" + field + "）"
	}

	eff := OpLogEffect{
		Entity:   entity,
		EntityID: cl.EntityID,
		Field:    fieldPtr,
		OldValue: cl.OldValue,
		NewValue: cl.NewValue,
		Desc:     summary,
	}
	if cl.Field == nil && cl.OldValue == nil && cl.NewValue == nil {
		eff.Desc = entity + action
		if business != "" {
			eff.Desc = summary
		}
	}

	return OpLog{
		ID:        cl.ID,
		Time:      cl.ChangedAt,
		Operation: action,
		Summary:   summary,
		Effects:   []OpLogEffect{eff},
	}
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
