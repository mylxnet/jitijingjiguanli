package contract

import (
	"database/sql"
	"fmt"

	"jititaizhang/server/internal/platform"
)

// Repo 封装 contract 表的 SQL 查询。
type Repo struct {
	db *sql.DB
}

// NewRepo 创建 Repo。
func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// ListContracts 查询某组织合同列表（不加载 file_data，供列表高效返回）。
// partyID 非 nil 时按单位过滤。
func (r *Repo) ListContracts(orgID int64, partyID *int64) ([]Contract, error) {
	where := "WHERE org_id = ?"
	args := []any{orgID}
	if partyID != nil {
		where += " AND party_id = ?"
		args = append(args, *partyID)
	}

	rows, err := r.db.Query(
		`SELECT id, org_id, party_id, file_name, file_size, mime_type, contract_title, contract_date, expires_at, created_at, updated_at
		 FROM contract `+where+` ORDER BY id DESC`, args...,
	)
	if err != nil {
		return nil, fmt.Errorf("查询合同列表失败: %w", err)
	}
	defer rows.Close()

	items := []Contract{}
	for rows.Next() {
		var c Contract
		if err := rows.Scan(&c.ID, &c.OrgID, &c.PartyID, &c.FileName, &c.FileSize, &c.MimeType,
			&c.ContractTitle, &c.ContractDate, &c.ExpiresAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("扫描合同行失败: %w", err)
		}
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// CreateContract 新建合同，fileData 为已解码的原始字节存 BLOB。
func (r *Repo) CreateContract(c *Contract, fileData []byte) (*Contract, error) {
	now := platform.Now()
	res, err := r.db.Exec(
		`INSERT INTO contract(org_id, party_id, file_name, file_size, mime_type, contract_title,
		                      contract_date, expires_at, file_data, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.OrgID, c.PartyID, c.FileName, c.FileSize, c.MimeType, c.ContractTitle,
		c.ContractDate, c.ExpiresAt, fileData, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("新建合同失败: %w", err)
	}
	id, _ := res.LastInsertId()
	c.ID = id
	c.CreatedAt = now
	c.UpdatedAt = now
	return c, nil
}

// FindContractByID 按 ID 查询合同（加载 file_data 到 FileBytes）。
func (r *Repo) FindContractByID(id int64) (*Contract, error) {
	c := &Contract{}
	var fileData []byte
	err := r.db.QueryRow(
		`SELECT id, org_id, party_id, file_name, file_size, mime_type, contract_title,
		        contract_date, expires_at, file_data, created_at, updated_at
		 FROM contract WHERE id = ?`, id,
	).Scan(&c.ID, &c.OrgID, &c.PartyID, &c.FileName, &c.FileSize, &c.MimeType,
		&c.ContractTitle, &c.ContractDate, &c.ExpiresAt, &fileData, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询合同失败: %w", err)
	}
	c.FileBytes = fileData
	return c, nil
}

// DeleteContract 删除合同，返回是否命中该组织的记录。
func (r *Repo) DeleteContract(id, orgID int64) (bool, error) {
	res, err := r.db.Exec(`DELETE FROM contract WHERE id = ? AND org_id = ?`, id, orgID)
	if err != nil {
		return false, fmt.Errorf("删除合同失败: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}