package transaction

import (
	"fmt"

	"jititaizhang/server/internal/platform"
)

// Leg 一次结转的转入目标。
type Leg struct {
	CategoryID  int64 `json:"categoryId"`
	AmountCents int64 `json:"amountCents"`
}

// CreateTransferRequest 创建科目间结转的请求。
// 结转=权益科目间 1 转出 → N 转入，不进收支汇总、不改银行存款。
// 可携带 TxnID 关联到来源流水，作废/恢复流水时联动置为相同状态。
type CreateTransferRequest struct {
	TxnDate          string `json:"txnDate"`
	SourceCategoryID int64  `json:"sourceCategoryId"`
	AmountCents      int64  `json:"amountCents"`
	Note             string `json:"note"`
	Legs             []Leg  `json:"legs"`
	TxnID            *int64 `json:"txnId"`
}

// CreateTransfer 原子创建科目间结转（transfer + transfer_leg），返回结转 ID。
func (r *Repo) CreateTransfer(orgID int64, req CreateTransferRequest) (int64, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("开启结转事务失败: %w", err)
	}
	defer tx.Rollback()

	now := platform.Now()
	res, err := tx.Exec(
		`INSERT INTO transfer(org_id, txn_date, source_category_id, source_amount_cents, note, status, txn_id, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, 'normal', ?, ?, ?)`,
		orgID, req.TxnDate, req.SourceCategoryID, req.AmountCents, req.Note, req.TxnID, now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("创建结转失败: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("读取结转 ID 失败: %w", err)
	}

	for _, leg := range req.Legs {
		if _, err := tx.Exec(
			`INSERT INTO transfer_leg(org_id, transfer_id, category_id, amount_cents) VALUES(?, ?, ?, ?)`,
			orgID, id, leg.CategoryID, leg.AmountCents,
		); err != nil {
			return 0, fmt.Errorf("创建结转目标失败: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("提交结转事务失败: %w", err)
	}
	return id, nil
}

// SetTransferStatusByTxn 把关联到某笔流水的所有结转置为指定状态（作废/恢复联动，limit 本组织）。
func (r *Repo) SetTransferStatusByTxn(orgID, txnID int64, status string) error {
	now := platform.Now()
	_, err := r.db.Exec(
		`UPDATE transfer SET status = ?, updated_at = ? WHERE org_id = ? AND txn_id = ?`,
		status, now, orgID, txnID,
	)
	if err != nil {
		return fmt.Errorf("更新结转状态失败: %w", err)
	}
	return nil
}