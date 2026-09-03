package receivable

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"jititaizhang/server/internal/platform"
)

// ErrOverReceivable 表示累计核销超过应收金额（服务层兜底）。
var ErrOverReceivable = errors.New("累计核销不能超过应收金额")

// Repo 封装 party / receivable / receipt 三表的 SQL。
type Repo struct {
	db *sql.DB
}

// NewRepo 创建 Repo。
func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// ---------- party ----------

// CreateParty 新建往来单位。
func (r *Repo) CreateParty(p *Party) (*Party, error) {
	now := platform.Now()
	res, err := r.db.Exec(
		`INSERT INTO party(org_id, name, type, contact_phone, area_mu, note, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		p.OrgID, p.Name, p.Type, p.ContactPhone, p.AreaMu, p.Note, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("新建往来单位失败: %w", err)
	}
	id, _ := res.LastInsertId()
	p.ID = id
	p.CreatedAt = now
	p.UpdatedAt = now
	return p, nil
}

// FindPartyByID 按 ID 查询往来单位（调用方需校验 OrgID）。
func (r *Repo) FindPartyByID(id int64) (*Party, error) {
	p := &Party{}
	err := r.db.QueryRow(
		`SELECT id, org_id, name, type, contact_phone, area_mu, note, created_at, updated_at FROM party WHERE id = ?`, id,
	).Scan(&p.ID, &p.OrgID, &p.Name, &p.Type, &p.ContactPhone, &p.AreaMu, &p.Note, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询往来单位失败: %w", err)
	}
	return p, nil
}

// ListParties 查询某组织往来单位（含欠款合计 = Σ未核销应收余额）。
// keyword 非空时按名称模糊过滤。投资公司附带长期投资同名科目累计投出。
func (r *Repo) ListParties(orgID int64, keyword string) ([]Party, error) {
	where := "WHERE p.org_id = ?"
	var args []any
	args = append(args, orgID)
	if keyword != "" {
		where += " AND p.name LIKE ?"
		args = append(args, "%"+keyword+"%")
	}

	rows, err := r.db.Query(
		`SELECT p.id, p.org_id, p.name, p.type, p.contact_phone, p.area_mu, p.note, p.created_at, p.updated_at,
		        COALESCE((SELECT SUM(rec.amount_cents - COALESCE((
		            SELECT SUM(re2.amount_cents) FROM receipt re2
		            WHERE re2.org_id = p.org_id AND re2.receivable_id = rec.id AND re2.status = 'normal'
		        ), 0)) FROM receivable rec
		        WHERE rec.org_id = p.org_id AND rec.party_id = p.id AND rec.status = 'open'), 0) AS outstanding
		 FROM party p `+where+` ORDER BY p.id DESC`, args...,
	)
	if err != nil {
		return nil, fmt.Errorf("查询往来单位失败: %w", err)
	}
	defer rows.Close()

	var items []Party
	for rows.Next() {
		p := Party{}
		if err := rows.Scan(&p.ID, &p.OrgID, &p.Name, &p.Type, &p.ContactPhone, &p.AreaMu, &p.Note, &p.CreatedAt, &p.UpdatedAt, &p.OutstandingCents); err != nil {
			return nil, fmt.Errorf("扫描往来单位行失败: %w", err)
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].Type == "invest" {
			items[i].InvestAmountCents = r.partyInvestAmount(orgID, items[i].Name)
		}
	}
	return items, nil
}

// partyInvestAmount 计算某投资公司的累计投出 = 「长期投资/对外投资」组下同名普通二级科目收到的支出流水合计。
func (r *Repo) partyInvestAmount(orgID int64, name string) int64 {
	var invested int64
	err := r.db.QueryRow(
		`SELECT COALESCE(SUM(CASE WHEN t.direction = 'expense' THEN t.amount_cents
		                          WHEN t.direction = 'income' THEN -t.amount_cents ELSE 0 END), 0)
		 FROM txn t
		 JOIN category c ON t.category_id = c.id
		 JOIN category l1 ON c.parent_id = l1.id
		 WHERE t.org_id = ? AND t.status = 'normal' AND c.level = 2
		   AND c.kind = 'equity' AND c.name = ?
		   AND l1.name IN ('长期投资', '对外投资')`, orgID, name,
	).Scan(&invested)
	if err != nil {
		return 0
	}
	if invested < 0 {
		return 0
	}
	return invested
}

// UpdateParty 更新往来单位（限定本组织）。
func (r *Repo) UpdateParty(id, orgID int64, updates map[string]any) error {
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
	_, err := r.db.Exec("UPDATE party SET "+strings.Join(setClauses, ", ")+" WHERE id = ? AND org_id = ?", args...)
	if err != nil {
		return fmt.Errorf("更新往来单位失败: %w", err)
	}
	return nil
}

// ---------- receivable ----------

// CreateReceivable 登记应收单（初始 open）。
func (r *Repo) CreateReceivable(rec *Receivable) (*Receivable, error) {
	now := platform.Now()
	res, err := r.db.Exec(
		`INSERT INTO receivable(org_id, party_id, recv_year, recv_kind, title, amount_cents, income_category_id, status, note, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, 'open', ?, ?, ?)`,
		rec.OrgID, rec.PartyID, rec.RecvYear, rec.RecvKind, rec.Title, rec.AmountCents, rec.IncomeCategoryID, rec.Note, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("登记应收单失败: %w", err)
	}
	id, _ := res.LastInsertId()
	rec.ID = id
	rec.Status = "open"
	rec.CreatedAt = now
	rec.UpdatedAt = now
	return rec, nil
}

// FindReceivableByID 按 ID 查询应收单（不含统计字段，供核销校验）。
func (r *Repo) FindReceivableByID(id int64) (*Receivable, error) {
	rec := &Receivable{}
	err := r.db.QueryRow(
		`SELECT id, org_id, party_id, recv_year, recv_kind, title, amount_cents, income_category_id, status, note, created_at, updated_at
		 FROM receivable WHERE id = ?`, id,
	).Scan(&rec.ID, &rec.OrgID, &rec.PartyID, &rec.RecvYear, &rec.RecvKind, &rec.Title, &rec.AmountCents, &rec.IncomeCategoryID, &rec.Status, &rec.Note, &rec.CreatedAt, &rec.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询应收单失败: %w", err)
	}
	return rec, nil
}

// ListReceivables 查询应收单列表（join party 名称，含已收/未收）。
// partyID/kind/status 可空过滤；year=0 表示全部年度。
func (r *Repo) ListReceivables(orgID int64, partyID *int64, year int, kind, status string, page, pageSize int) ([]Receivable, int, error) {
	where := "WHERE r.org_id = ?"
	var args []any
	args = append(args, orgID)
	if partyID != nil {
		where += " AND r.party_id = ?"
		args = append(args, *partyID)
	}
	if year > 0 {
		where += " AND r.recv_year = ?"
		args = append(args, year)
	}
	if kind == "rent" || kind == "dividend" || kind == "other" {
		where += " AND r.recv_kind = ?"
		args = append(args, kind)
	}
	if status == "open" || status == "closed" {
		where += " AND r.status = ?"
		args = append(args, status)
	}

	sel := `SELECT r.id, r.org_id, r.party_id, p.name, r.recv_year, r.recv_kind, r.title, r.amount_cents, r.income_category_id,
	        r.status, r.note, r.created_at, r.updated_at,
	        COALESCE((SELECT SUM(rc.amount_cents) FROM receipt rc
	            WHERE rc.org_id = r.org_id AND rc.receivable_id = r.id AND rc.status = 'normal'), 0) AS paid
	 FROM receivable r JOIN party p ON p.id = r.party_id AND p.org_id = r.org_id `

	var total int
	if err := r.db.QueryRow("SELECT COUNT(*) FROM receivable r "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计应收单数失败: %w", err)
	}

	offset := (page - 1) * pageSize
	listArgs := append(args, pageSize, offset)
	rows, err := r.db.Query(sel+where+` ORDER BY r.id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询应收单失败: %w", err)
	}
	defer rows.Close()

	var items []Receivable
	for rows.Next() {
		rec, err := scanReceivableRow(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *rec)
	}
	return items, total, rows.Err()
}

// scanReceivableRow 扫描带 partyName 与 paid 的应收单行。
type receivableScanner interface {
	Scan(dest ...any) error
}

func scanReceivableRow(row receivableScanner) (*Receivable, error) {
	rec := &Receivable{}
	var paid int64
	err := row.Scan(
		&rec.ID, &rec.OrgID, &rec.PartyID, &rec.PartyName, &rec.RecvYear, &rec.RecvKind, &rec.Title, &rec.AmountCents,
		&rec.IncomeCategoryID, &rec.Status, &rec.Note, &rec.CreatedAt, &rec.UpdatedAt, &paid,
	)
	if err != nil {
		return nil, fmt.Errorf("扫描应收单行失败: %w", err)
	}
	rec.PaidCents = paid
	rec.OutstandingCents = rec.AmountCents - paid
	return rec, nil
}

// GetReceivableDetail 查询应收单详情（含核销记录）。
func (r *Repo) GetReceivableDetail(orgID, id int64) (*ReceivableDetail, error) {
	var rec *Receivable
	row := r.db.QueryRow(
		`SELECT r.id, r.org_id, r.party_id, p.name, r.recv_year, r.recv_kind, r.title, r.amount_cents, r.income_category_id,
		        r.status, r.note, r.created_at, r.updated_at,
		        COALESCE((SELECT SUM(rc.amount_cents) FROM receipt rc
		            WHERE rc.org_id = r.org_id AND rc.receivable_id = r.id AND rc.status = 'normal'), 0) AS paid
		 FROM receivable r JOIN party p ON p.id = r.party_id AND p.org_id = r.org_id
		 WHERE r.org_id = ? AND r.id = ?`, orgID, id,
	)
	rec, err := scanReceivableRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询应收单详情失败: %w", err)
	}

	receipts, err := r.ListReceiptsByReceivable(orgID, id)
	if err != nil {
		return nil, err
	}
	return &ReceivableDetail{Receivable: *rec, Receipts: receipts}, nil
}

// UpdateReceivableStatus 更新应收单状态（限定本组织）。
func (r *Repo) UpdateReceivableStatus(id, orgID int64, status string) error {
	_, err := r.db.Exec(`UPDATE receivable SET status = ? WHERE id = ? AND org_id = ?`, status, id, orgID)
	return err
}

// DeleteOpenReceivable 作废未收款应收单（仅 open 且无任何正常核销时允许删除，供年度结转重录）。
func (r *Repo) DeleteOpenReceivable(orgID, id int64) error {
	res, err := r.db.Exec(`DELETE FROM receivable WHERE id = ? AND org_id = ? AND status = 'open'`, id, orgID)
	if err != nil {
		return fmt.Errorf("作废应收单失败: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("应收单不存在或已结清，无法作废")
	}
	return nil
}

// ---------- receipt ----------

// ListReceiptsByReceivable 查询某应收单的全部核销记录（倒序）。
func (r *Repo) ListReceiptsByReceivable(orgID, receivableID int64) ([]Receipt, error) {
	rows, err := r.db.Query(
		`SELECT id, org_id, receivable_id, amount_cents, receipt_date, method, txn_id, note, status, created_at, updated_at
		 FROM receipt WHERE org_id = ? AND receivable_id = ? ORDER BY receipt_date DESC, id DESC`, orgID, receivableID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询核销记录失败: %w", err)
	}
	defer rows.Close()

	var items []Receipt
	for rows.Next() {
		rc := Receipt{}
		if err := rows.Scan(&rc.ID, &rc.OrgID, &rc.ReceivableID, &rc.AmountCents, &rc.ReceiptDate, &rc.Method, &rc.TxnID, &rc.Note, &rc.Status, &rc.CreatedAt, &rc.UpdatedAt); err != nil {
			return nil, fmt.Errorf("扫描核销记录失败: %w", err)
		}
		items = append(items, rc)
	}
	return items, rows.Err()
}

// FindReceiptByID 按 ID 查询核销记录（调用方需校验 OrgID）。
func (r *Repo) FindReceiptByID(id int64) (*Receipt, error) {
	rc := &Receipt{}
	err := r.db.QueryRow(
		`SELECT id, org_id, receivable_id, amount_cents, receipt_date, method, txn_id, note, status, created_at, updated_at
		 FROM receipt WHERE id = ?`, id,
	).Scan(&rc.ID, &rc.OrgID, &rc.ReceivableID, &rc.AmountCents, &rc.ReceiptDate, &rc.Method, &rc.TxnID, &rc.Note, &rc.Status, &rc.CreatedAt, &rc.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询核销记录失败: %w", err)
	}
	return rc, nil
}

// CreateOutcome 是核销创建结果（供 handler 记录留痕与状态变化）。
type CreateOutcome struct {
	Receipt          *Receipt
	ReceivableStatus string // 核销后应收单状态（open/closed）
	TxnCreated       *int64 // cash 方式自动生成的银行收入流水 id
}

// CreateCashReceipt 现金核销：单事务写入 receipt + 银行收入流水（txn），
// 累计核销恰好等于应收金额时自动把应收单置为 closed。
// receivable 须为调用方已校验归属的最新应收单。
func (r *Repo) CreateCashReceipt(orgID int64, rec *Receivable, amount int64, date string, categoryID int64, note *string) (*CreateOutcome, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	paid, err := sumPaidTx(tx, orgID, rec.ID)
	if err != nil {
		return nil, err
	}
	if paid+amount > rec.AmountCents {
		return nil, ErrOverReceivable
	}

	now := platform.Now()
	txnRes, err := tx.Exec(
		`INSERT INTO txn(org_id, txn_date, direction, amount_cents, category_id, note, status, created_at, updated_at)
		 VALUES(?, ?, 'income', ?, ?, ?, 'normal', ?, ?)`,
		orgID, date, amount, categoryID, note, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("生成银行收入流水失败: %w", err)
	}
	txnID, _ := txnRes.LastInsertId()

	res, err := tx.Exec(
		`INSERT INTO receipt(org_id, receivable_id, amount_cents, receipt_date, method, txn_id, note, status, created_at, updated_at)
		 VALUES(?, ?, ?, ?, 'cash', ?, ?, 'normal', ?, ?)`,
		orgID, rec.ID, amount, date, txnID, note, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("写入核销记录失败: %w", err)
	}
	receiptID, _ := res.LastInsertId()

	newStatus := "open"
	if paid+amount == rec.AmountCents {
		newStatus = "closed"
	}
	if newStatus != rec.Status {
		if _, err := tx.Exec(`UPDATE receivable SET status = ?, updated_at = ? WHERE id = ? AND org_id = ?`, newStatus, now, rec.ID, orgID); err != nil {
			return nil, fmt.Errorf("更新应收单状态失败: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交核销事务失败: %w", err)
	}

	rc := &Receipt{ID: receiptID, OrgID: orgID, ReceivableID: rec.ID, AmountCents: amount, ReceiptDate: date,
		Method: "cash", TxnID: &txnID, Note: note, Status: "normal", CreatedAt: now, UpdatedAt: now}
	return &CreateOutcome{Receipt: rc, ReceivableStatus: newStatus, TxnCreated: &txnID}, nil
}

// CreateOffsetReceipt 抵销核销：单事务写入 receipt（关联既有发放支出流水 txn_id），
// 累计核销恰好等于应收金额时自动把应收单置为 closed。
func (r *Repo) CreateOffsetReceipt(orgID int64, rec *Receivable, amount int64, date string, txnID int64, note *string) (*CreateOutcome, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	paid, err := sumPaidTx(tx, orgID, rec.ID)
	if err != nil {
		return nil, err
	}
	if paid+amount > rec.AmountCents {
		return nil, ErrOverReceivable
	}

	now := platform.Now()
	res, err := tx.Exec(
		`INSERT INTO receipt(org_id, receivable_id, amount_cents, receipt_date, method, txn_id, note, status, created_at, updated_at)
		 VALUES(?, ?, ?, ?, 'offset', ?, ?, 'normal', ?, ?)`,
		orgID, rec.ID, amount, date, txnID, note, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("写入核销记录失败: %w", err)
	}
	receiptID, _ := res.LastInsertId()

	newStatus := "open"
	if paid+amount == rec.AmountCents {
		newStatus = "closed"
	}
	if newStatus != rec.Status {
		if _, err := tx.Exec(`UPDATE receivable SET status = ?, updated_at = ? WHERE id = ? AND org_id = ?`, newStatus, now, rec.ID, orgID); err != nil {
			return nil, fmt.Errorf("更新应收单状态失败: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交核销事务失败: %w", err)
	}

	rc := &Receipt{ID: receiptID, OrgID: orgID, ReceivableID: rec.ID, AmountCents: amount, ReceiptDate: date,
		Method: "offset", TxnID: &txnID, Note: note, Status: "normal", CreatedAt: now, UpdatedAt: now}
	return &CreateOutcome{Receipt: rc, ReceivableStatus: newStatus}, nil
}

// VoidOutcome 是作废核销的结果（供 handler 记录留痕与状态变化）。
type VoidOutcome struct {
	Receipt          *Receipt
	ReceivableStatus string // 作废后应收单状态
	TxnVoided        *int64 // cash 核销被同步作废的银行流水 id
}

// VoidReceipt 作废核销（限定本组织）：cash 核销同步作废其银行收入流水；
// 若因此应收单未结清则状态退回 open。单事务完成。
func (r *Repo) VoidReceipt(orgID, receiptID int64) (*VoidOutcome, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	rc := &Receipt{}
	err = tx.QueryRow(
		`SELECT id, org_id, receivable_id, amount_cents, receipt_date, method, txn_id, note, status, created_at, updated_at
		 FROM receipt WHERE id = ? AND org_id = ?`, receiptID, orgID,
	).Scan(&rc.ID, &rc.OrgID, &rc.ReceivableID, &rc.AmountCents, &rc.ReceiptDate, &rc.Method, &rc.TxnID, &rc.Note, &rc.Status, &rc.CreatedAt, &rc.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询核销记录失败: %w", err)
	}
	if rc.Status == "voided" {
		_ = tx.Commit()
		return &VoidOutcome{Receipt: rc, ReceivableStatus: ""}, nil // 已作废，幂等返回
	}

	now := platform.Now()
	if _, err := tx.Exec(`UPDATE receipt SET status = 'voided', updated_at = ? WHERE id = ? AND org_id = ?`, now, receiptID, orgID); err != nil {
		return nil, fmt.Errorf("作废核销记录失败: %w", err)
	}

	// cash 核销：同步作废其自动生成的收入流水（抵销核销的 txn 是独立支出流水，不动）
	var txnVoided *int64
	if rc.Method == "cash" && rc.TxnID != nil {
		if _, err := tx.Exec(`UPDATE txn SET status = 'voided', updated_at = ? WHERE id = ? AND org_id = ?`, now, *rc.TxnID, orgID); err != nil {
			return nil, fmt.Errorf("作废银行收入流水失败: %w", err)
		}
		txnVoided = rc.TxnID
	}

	// 重新计算已核销，未结清则退回 open
	paid, err := sumPaidTx(tx, orgID, rc.ReceivableID)
	if err != nil {
		return nil, err
	}
	var receivableAmount int64
	if err := tx.QueryRow(`SELECT amount_cents FROM receivable WHERE id = ? AND org_id = ?`, rc.ReceivableID, orgID).Scan(&receivableAmount); err != nil {
		return nil, fmt.Errorf("查询应收金额失败: %w", err)
	}
	newStatus := "open"
	if paid >= receivableAmount {
		newStatus = "closed"
	}
	if _, err := tx.Exec(`UPDATE receivable SET status = ?, updated_at = ? WHERE id = ? AND org_id = ?`, newStatus, now, rc.ReceivableID, orgID); err != nil {
		return nil, fmt.Errorf("更新应收单状态失败: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交作废事务失败: %w", err)
	}
	rc.Status = "voided"
	rc.UpdatedAt = now
	return &VoidOutcome{Receipt: rc, ReceivableStatus: newStatus, TxnVoided: txnVoided}, nil
}

// ---------- 批量计提与计提标准（v0.4） ----------

// ReceivableExists 判断 单位+年度+类别 应收单是否已存在（防重）。
func (r *Repo) ReceivableExists(orgID, partyID int64, year int, kind string) (bool, error) {
	var n int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM receivable WHERE org_id=? AND party_id=? AND recv_year=? AND recv_kind=?`,
		orgID, partyID, year, kind,
	).Scan(&n)
	return n > 0, err
}

// BatchCreateReceivables 批量计提应收（同年+同类已存在则跳过）。
func (r *Repo) BatchCreateReceivables(orgID int64, year int, title string, items []BatchAccrueItem) (*BatchAccrueResult, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	result := &BatchAccrueResult{}
	now := platform.Now()
	for _, it := range items {
		var n int
		if err := tx.QueryRow(
			`SELECT COUNT(*) FROM receivable WHERE org_id=? AND party_id=? AND recv_year=? AND recv_kind=?`,
			orgID, it.PartyID, year, it.RecvKind,
		).Scan(&n); err != nil {
			return nil, fmt.Errorf("查重失败: %w", err)
		}
		if n > 0 {
			result.Skipped++
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO receivable(org_id, party_id, recv_year, recv_kind, title, amount_cents, income_category_id, status, note, created_at, updated_at)
			 VALUES(?,?,?,?,?,?,NULL,'open',NULL,?,?)`,
			orgID, it.PartyID, year, it.RecvKind, title, it.AmountCents, now, now,
		); err != nil {
			return nil, fmt.Errorf("批量计提失败: %w", err)
		}
		result.Created++
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交批量计提失败: %w", err)
	}
	return result, nil
}

// ListStandards 列出某组织计提标准（含单位名）。
func (r *Repo) ListStandards(orgID int64, kind string) ([]AccrualStandard, error) {
	where := "WHERE s.org_id = ?"
	args := []any{orgID}
	if kind == "rent" || kind == "dividend" || kind == "other" {
		where += " AND s.recv_kind = ?"
		args = append(args, kind)
	}
	rows, err := r.db.Query(`SELECT s.id, s.org_id, s.party_id, p.name, s.recv_kind, s.amount_cents, s.active,
		s.created_at, s.updated_at FROM recv_standard s JOIN party p ON p.id=s.party_id AND p.org_id=s.org_id `+where+` ORDER BY p.name`,
		args...)
	if err != nil {
		return nil, fmt.Errorf("查询计提标准失败: %w", err)
	}
	defer rows.Close()
	var items []AccrualStandard
	for rows.Next() {
		s := AccrualStandard{}
		var active int
		if err := rows.Scan(&s.ID, &s.OrgID, &s.PartyID, &s.PartyName, &s.RecvKind, &s.AmountCents, &active, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("扫描计提标准失败: %w", err)
		}
		s.Active = active == 1
		items = append(items, s)
	}
	return items, rows.Err()
}

// UpsertStandard 保存计提标准（同单位+类别唯一）。
func (r *Repo) UpsertStandard(orgID int64, s *AccrualStandard) (*AccrualStandard, error) {
	now := platform.Now()
	active := 1
	if !s.Active {
		active = 0
	}
	res, err := r.db.Exec(
		`INSERT INTO recv_standard(org_id, party_id, recv_kind, amount_cents, active, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?)
		 ON CONFLICT(org_id, party_id, recv_kind) DO UPDATE SET amount_cents=excluded.amount_cents, active=excluded.active, updated_at=excluded.updated_at`,
		orgID, s.PartyID, s.RecvKind, s.AmountCents, active, now, now)
	if err != nil {
		return nil, fmt.Errorf("保存计提标准失败: %w", err)
	}
	id, _ := res.LastInsertId()
	if id == 0 {
		if err := r.db.QueryRow(`SELECT id FROM recv_standard WHERE org_id=? AND party_id=? AND recv_kind=?`,
			orgID, s.PartyID, s.RecvKind).Scan(&id); err != nil {
			return nil, fmt.Errorf("查询计提标准失败: %w", err)
		}
	}
	s.ID = id
	s.OrgID = orgID
	return s, nil
}

// SetStandardActive 启停标准。
func (r *Repo) SetStandardActive(id, orgID int64, active bool) error {
	v := 0
	if active {
		v = 1
	}
	if _, err := r.db.Exec(`UPDATE recv_standard SET active=?, updated_at=? WHERE id=? AND org_id=?`, v, platform.Now(), id, orgID); err != nil {
		return fmt.Errorf("更新计提标准失败: %w", err)
	}
	return nil
}

// AccrueFromStandards 按启用标准一键结转年度应收（存在则跳过）。
// 只有单位类型与标准类别匹配的才结转：rent→流转企业 flow；dividend→投资公司 invest。
func (r *Repo) AccrueFromStandards(orgID int64, year int, kind, title string) (*BatchAccrueResult, error) {
	partyType := map[string]string{"rent": "flow", "dividend": "invest", "other": "other"}[kind]
	if partyType == "" {
		return &BatchAccrueResult{}, nil
	}
	rows, err := r.db.Query(
		`SELECT s.party_id, s.recv_kind, s.amount_cents
		 FROM recv_standard s JOIN party p ON p.id = s.party_id AND p.org_id = s.org_id
		 WHERE s.org_id = ? AND s.recv_kind = ? AND p.type = ? AND s.active = 1
		 ORDER BY p.name`, orgID, kind, partyType,
	)
	if err != nil {
		return nil, fmt.Errorf("查询可结转标准失败: %w", err)
	}
	defer rows.Close()
	var items []BatchAccrueItem
	for rows.Next() {
		it := BatchAccrueItem{}
		if err := rows.Scan(&it.PartyID, &it.RecvKind, &it.AmountCents); err != nil {
			return nil, fmt.Errorf("扫描可结转标准失败: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return &BatchAccrueResult{}, nil
	}
	return r.BatchCreateReceivables(orgID, year, title, items)
}

// PreviewAccrueFromStandards 生成年度结转预览：启用标准中类型匹配的行，并标注同年同类应收单是否已存在。
func (r *Repo) PreviewAccrueFromStandards(orgID int64, year int) (*PreviewAccrueResult, error) {
	rows, err := r.db.Query(
		`SELECT s.recv_kind, s.party_id, p.name, s.amount_cents,
		        EXISTS(SELECT 1 FROM receivable rec
		               WHERE rec.org_id = s.org_id AND rec.party_id = s.party_id
		                 AND rec.recv_year = ? AND rec.recv_kind = s.recv_kind) AS already
		 FROM recv_standard s JOIN party p ON p.id = s.party_id AND p.org_id = s.org_id
		 WHERE s.org_id = ? AND s.active = 1
		   AND ((s.recv_kind = 'rent' AND p.type = 'flow') OR (s.recv_kind = 'dividend' AND p.type = 'invest'))
		 ORDER BY s.recv_kind, p.name`, year, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询年度结转预览失败: %w", err)
	}
	defer rows.Close()
	result := &PreviewAccrueResult{Year: year, Items: []PreviewAccrueItem{}}
	for rows.Next() {
		it := PreviewAccrueItem{}
		var already int
		if err := rows.Scan(&it.Kind, &it.PartyID, &it.PartyName, &it.AmountCents, &already); err != nil {
			return nil, fmt.Errorf("扫描年度结转预览失败: %w", err)
		}
		it.Exists = already == 1
		if it.Kind == "rent" {
			it.Title = fmt.Sprintf("%d年度土地流转费", year)
		} else {
			it.Title = fmt.Sprintf("%d年度投资收益", year)
		}
		result.Items = append(result.Items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func sumPaidTx(tx *sql.Tx, orgID, receivableID int64) (int64, error) {
	var paid int64
	if err := tx.QueryRow(
		`SELECT COALESCE(SUM(amount_cents), 0) FROM receipt WHERE org_id = ? AND receivable_id = ? AND status = 'normal'`,
		orgID, receivableID,
	).Scan(&paid); err != nil {
		return 0, fmt.Errorf("统计已核销金额失败: %w", err)
	}
	return paid, nil
}
