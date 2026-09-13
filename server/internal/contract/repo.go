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

// ListExpiring 查询「各单位的合同最新期至」并关联往来单位（含单位名/类型）。
// 每个单位只返回期至（MAX(expires_at)）最大的那条合同；无到期日合同或并列时可能多条。
// 供「合同到期」提醒使用；须在 handler 层按当时日期筛选 30 天内/已到期。
func (r *Repo) ListExpiring(orgID int64) ([]ExpiringItem, error) {
	rows, err := r.db.Query(
		`SELECT c.id, c.party_id, p.name, p.type, c.contract_title, c.file_name, c.expires_at
		 FROM contract c JOIN party p ON p.id = c.party_id
		 WHERE c.org_id = ?
		   AND c.expires_at IS NOT NULL AND c.expires_at <> ''
		   AND c.expires_at = (
		     SELECT MAX(c2.expires_at) FROM contract c2
		     WHERE c2.party_id = c.party_id AND c2.org_id = ?
		       AND c2.expires_at IS NOT NULL AND c2.expires_at <> ''
		   )
		 ORDER BY c.expires_at`, orgID, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询到期合同失败: %w", err)
	}
	defer rows.Close()

	items := []ExpiringItem{}
	for rows.Next() {
		var it ExpiringItem
		if err := rows.Scan(&it.ContractID, &it.PartyID, &it.PartyName, &it.Type,
			&it.ContractTitle, &it.FileName, &it.ExpiresAt); err != nil {
			return nil, fmt.Errorf("扫描到期合同行失败: %w", err)
		}
		items = append(items, it)
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

// UpdateExpiry 更新合同到期日（expiresAt 为 nil 或空串时清除为 NULL）。返回是否命中该组织的记录。
func (r *Repo) UpdateExpiry(id, orgID int64, expiresAt *string) (bool, error) {
	var v any // nil → 存 NULL（清除到期）
	if expiresAt != nil && *expiresAt != "" {
		v = *expiresAt
	}
	res, err := r.db.Exec(
		`UPDATE contract SET expires_at = ?, updated_at = ? WHERE id = ? AND org_id = ?`,
		v, platform.Now(), id, orgID,
	)
	if err != nil {
		return false, fmt.Errorf("更新合同到期日失败: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
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