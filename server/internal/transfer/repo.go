package transfer

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"jititaizhang/server/internal/platform"
)

// Repo 封装 transfer 与 transfer_leg 表的 SQL 查询。
type Repo struct {
	db *sql.DB
}

// NewRepo 创建 Repo。
func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// Create 在事务内创建一笔转账（transfer + legs 原子写入）。
func (r *Repo) Create(t *Transfer) (*Transfer, error) {
	now := platform.Now()

	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`INSERT INTO transfer(org_id, txn_date, source_category_id, source_amount_cents, note, status, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, 'normal', ?, ?)`,
		t.OrgID, t.TxnDate, t.SourceCategoryID, t.SourceAmountCents, t.Note, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("创建转账失败: %w", err)
	}

	id, _ := res.LastInsertId()
	t.ID = id
	t.Status = "normal"
	t.CreatedAt = now
	t.UpdatedAt = now

	// 插入 legs
	for i := range t.Legs {
		leg := &t.Legs[i]
		res, err := tx.Exec(
			`INSERT INTO transfer_leg(org_id, transfer_id, category_id, amount_cents) VALUES(?, ?, ?, ?)`,
			t.OrgID, id, leg.CategoryID, leg.AmountCents,
		)
		if err != nil {
			return nil, fmt.Errorf("创建转入明细失败: %w", err)
		}
		legID, _ := res.LastInsertId()
		leg.ID = legID
		leg.TransferID = id
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交转账事务失败: %w", err)
	}
	return t, nil
}

// FindByID 查询转账（含 legs）。调用方需校验 OrgID。
func (r *Repo) FindByID(id int64) (*Transfer, error) {
	t := &Transfer{}
	err := r.db.QueryRow(
		`SELECT id, org_id, txn_date, source_category_id, source_amount_cents, note, status, created_at, updated_at
		 FROM transfer WHERE id = ?`, id,
	).Scan(&t.ID, &t.OrgID, &t.TxnDate, &t.SourceCategoryID, &t.SourceAmountCents, &t.Note, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询转账失败: %w", err)
	}

	// 加载 legs
	legs, err := r.FindLegsByTransferID(id)
	if err != nil {
		return nil, err
	}
	t.Legs = legs
	return t, nil
}

// FindLegsByTransferID 查询转账的转入明细。
func (r *Repo) FindLegsByTransferID(transferID int64) ([]Leg, error) {
	rows, err := r.db.Query(
		`SELECT id, transfer_id, category_id, amount_cents FROM transfer_leg WHERE transfer_id = ?`, transferID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询转入明细失败: %w", err)
	}
	defer rows.Close()

	var legs []Leg
	for rows.Next() {
		l := Leg{}
		if err := rows.Scan(&l.ID, &l.TransferID, &l.CategoryID, &l.AmountCents); err != nil {
			return nil, fmt.Errorf("扫描转入明细失败: %w", err)
		}
		legs = append(legs, l)
	}
	return legs, rows.Err()
}

// List 查询某组织转账记录列表。
func (r *Repo) List(orgID int64, from, to string, categoryID *int64, page, pageSize int) ([]*Transfer, int, error) {
	where := "WHERE t.org_id = ?"
	var args []any
	args = append(args, orgID)

	if from != "" {
		where += " AND t.txn_date >= ?"
		args = append(args, from)
	}
	if to != "" {
		where += " AND t.txn_date <= ?"
		args = append(args, to)
	}
	if categoryID != nil {
		where += " AND (t.source_category_id = ? OR EXISTS (SELECT 1 FROM transfer_leg WHERE transfer_id = t.id AND category_id = ?))"
		args = append(args, *categoryID, *categoryID)
	}

	// 总数
	var total int
	countQuery := "SELECT COUNT(*) FROM transfer t " + where
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计转账数失败: %w", err)
	}

	// 列表
	offset := (page - 1) * pageSize
	listQuery := "SELECT t.id, t.org_id, t.txn_date, t.source_category_id, t.source_amount_cents, t.note, t.status, t.created_at, t.updated_at FROM transfer t " +
		where + " ORDER BY t.txn_date DESC, t.id DESC LIMIT ? OFFSET ?"
	listArgs := append(args, pageSize, offset)

	rows, err := r.db.Query(listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询转账列表失败: %w", err)
	}
	defer rows.Close()

	var items []*Transfer
	var ids []int64
	for rows.Next() {
		t := &Transfer{}
		if err := rows.Scan(&t.ID, &t.OrgID, &t.TxnDate, &t.SourceCategoryID, &t.SourceAmountCents, &t.Note, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("扫描转账行失败: %w", err)
		}
		items = append(items, t)
		ids = append(ids, t.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// 一次查出本页全部 legs，避免 N+1 与错误被静默吞掉
	legsByTransfer, err := r.findLegsByTransferIDs(ids)
	if err != nil {
		return nil, 0, err
	}
	for _, t := range items {
		t.Legs = legsByTransfer[t.ID]
	}
	return items, total, nil
}

// UpdateStatus 更新转账状态（作废/撤销，限定本组织）。
func (r *Repo) UpdateStatus(id, orgID int64, status string, updatedAt time.Time) error {
	_, err := r.db.Exec(`UPDATE transfer SET status = ?, updated_at = ? WHERE id = ? AND org_id = ?`, status, updatedAt, id, orgID)
	return err
}

// findLegsByTransferIDs 一次查出多笔转账的全部转入明细（按 transfer_id 分组）。
func (r *Repo) findLegsByTransferIDs(ids []int64) (map[int64][]Leg, error) {
	result := make(map[int64][]Leg)
	if len(ids) == 0 {
		return result, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := r.db.Query(
		`SELECT id, transfer_id, category_id, amount_cents FROM transfer_leg WHERE transfer_id IN (`+placeholders+`)`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("查询转入明细失败: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		l := Leg{}
		if err := rows.Scan(&l.ID, &l.TransferID, &l.CategoryID, &l.AmountCents); err != nil {
			return nil, fmt.Errorf("扫描转入明细失败: %w", err)
		}
		result[l.TransferID] = append(result[l.TransferID], l)
	}
	return result, rows.Err()
}