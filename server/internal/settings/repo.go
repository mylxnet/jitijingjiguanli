package settings

import (
	"database/sql"
	"fmt"
	"strconv"
)

// Repo 封装 app_setting 表的 SQL 查询。
type Repo struct {
	db *sql.DB
}

// NewRepo 创建 Repo。
func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// Get 读取指定组织的系统配置（app_setting.value 为 TEXT，需显式转 int64）。
func (r *Repo) Get(orgID int64) (*Settings, error) {
	s := &Settings{}

	var val string
	err := r.db.QueryRow(
		`SELECT value FROM app_setting WHERE org_id = ? AND key = 'bank_opening_balance_cents'`,
		orgID,
	).Scan(&val)
	if err == sql.ErrNoRows {
		return s, nil // 未设置过 → 期初余额 0
	}
	if err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}

	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("配置值不是合法整数: %w", err)
	}
	s.BankOpeningBalanceCents = n
	return s, nil
}

// Upsert 写入或更新组织配置项。
func (r *Repo) Upsert(orgID int64, key string, value string) error {
	_, err := r.db.Exec(
		`INSERT INTO app_setting(org_id, key, value) VALUES(?, ?, ?)
		 ON CONFLICT(org_id, key) DO UPDATE SET value = excluded.value`,
		orgID, key, value,
	)
	if err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}
	return nil
}