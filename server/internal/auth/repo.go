package auth

import (
	"database/sql"
	"fmt"
	"time"

	"jititaizhang/server/internal/platform"
)

// Repo 封装 org / user / session 表的 SQL 查询。
type Repo struct {
	db *sql.DB
}

// NewRepo 创建 Repo。
func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// CreateOrg 创建组织。
func (r *Repo) CreateOrg(name string) (*Org, error) {
	now := platform.Now()
	res, err := r.db.Exec(
		`INSERT INTO org(name, created_at, updated_at) VALUES(?, ?, ?)`,
		name, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("创建组织失败: %w", err)
	}
	id, _ := res.LastInsertId()
	return &Org{ID: id, Name: name, CreatedAt: now, UpdatedAt: now}, nil
}

// CreateUser 创建用户（绑定组织）。
func (r *Repo) CreateUser(orgID int64, username, passwordHash string) (*User, error) {
	now := platform.Now()
	res, err := r.db.Exec(
		`INSERT INTO user(org_id, username, password_hash, created_at) VALUES(?, ?, ?, ?)`,
		orgID, username, passwordHash, now,
	)
	if err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}
	id, _ := res.LastInsertId()
	return &User{ID: id, OrgID: orgID, Username: username, CreatedAt: now}, nil
}

// FindByUsername 按用户名查找用户（含组织归属）。
func (r *Repo) FindByUsername(username string) (*User, error) {
	u := &User{}
	err := r.db.QueryRow(
		`SELECT id, org_id, username, password_hash, created_at FROM user WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.OrgID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查找用户失败: %w", err)
	}
	return u, nil
}

// FindUserByID 按 ID 查找用户（含组织归属）。
func (r *Repo) FindUserByID(id int64) (*User, error) {
	u := &User{}
	err := r.db.QueryRow(
		`SELECT id, org_id, username, password_hash, created_at FROM user WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.OrgID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查找用户失败: %w", err)
	}
	return u, nil
}

// UpdatePassword 更新口令哈希（修改密码）。
func (r *Repo) UpdatePassword(userID int64, passwordHash string) error {
	if _, err := r.db.Exec(`UPDATE user SET password_hash = ? WHERE id = ?`, passwordHash, userID); err != nil {
		return fmt.Errorf("更新口令失败: %w", err)
	}
	return nil
}

// CreateSession 创建会话。
func (r *Repo) CreateSession(id string, userID int64, expiresAt time.Time) error {
	_, err := r.db.Exec(
		`INSERT INTO session(id, user_id, expires_at) VALUES(?, ?, ?)`,
		id, userID, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("创建会话失败: %w", err)
	}
	return nil
}

// FindSession 按会话 ID 查找有效会话。
func (r *Repo) FindSession(id string) (*Session, error) {
	s := &Session{}
	err := r.db.QueryRow(
		`SELECT id, user_id, expires_at FROM session WHERE id = ? AND expires_at > ?`,
		id, platform.Now(),
	).Scan(&s.ID, &s.UserID, &s.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查找会话失败: %w", err)
	}
	return s, nil
}

// DeleteSession 删除会话（登出）。
func (r *Repo) DeleteSession(id string) error {
	_, err := r.db.Exec(`DELETE FROM session WHERE id = ?`, id)
	return err
}

// WithTx 在单事务内执行 fn（注册 = 建组织+用户+预置科目原子完成）。
func (r *Repo) WithTx(fn func(tx *sql.Tx) error) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// createOrgTx 事务内建组织。
func (r *Repo) createOrgTx(tx *sql.Tx, name string) (int64, error) {
	now := platform.Now()
	res, err := tx.Exec(
		`INSERT INTO org(name, created_at, updated_at) VALUES(?, ?, ?)`,
		name, now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("创建组织失败: %w", err)
	}
	return res.LastInsertId()
}

// createUserTx 事务内建用户。
func (r *Repo) createUserTx(tx *sql.Tx, orgID int64, username, passwordHash string) (int64, error) {
	now := platform.Now()
	res, err := tx.Exec(
		`INSERT INTO user(org_id, username, password_hash, created_at) VALUES(?, ?, ?, ?)`,
		orgID, username, passwordHash, now,
	)
	if err != nil {
		return 0, fmt.Errorf("创建用户失败: %w", err)
	}
	return res.LastInsertId()
}

// seedPresetCategoriesTx 事务内写入预置科目（长期投资留空：使用时由操作者按公司建普通二级，
// 投资/收回直接在快速记账里走收支流水；资产类/资金划转已弃用，仅保留旧数据兼容）。
func (r *Repo) seedPresetCategoriesTx(tx *sql.Tx, orgID int64) error {
	now := platform.Now()
	insert := func(name string, level int, parentID any, kind string, sort int) (int64, error) {
		res, err := tx.Exec(
			`INSERT INTO category(org_id, name, level, parent_id, status, kind,
				preset, sort_order, created_at, updated_at)
			 VALUES(?,?,?,?, 'active', ?,1,?,?,?)`,
			orgID, name, level, parentID, kind, sort, now, now,
		)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}

	// 一级分组（权益容器）
	l1Fund, err := insert("本金", 1, nil, "equity", 1)
	if err != nil {
		return fmt.Errorf("预置科目失败(本金): %w", err)
	}
	l1Invest, err := insert("长期投资", 1, nil, "equity", 2)
	if err != nil {
		return fmt.Errorf("预置科目失败(长期投资): %w", err)
	}
	l1Income, err := insert("经营收入", 1, nil, "equity", 3)
	if err != nil {
		return fmt.Errorf("预置科目失败(经营收入): %w", err)
	}
	l1Dist, err := insert("分配与支出", 1, nil, "equity", 4)
	if err != nil {
		return fmt.Errorf("预置科目失败(分配与支出): %w", err)
	}

	// 权益二级预设
	presetL2 := []struct {
		name     string
		parentID int64
		kind     string
		sort     int
	}{
		{"上级补助", l1Fund, "equity", 1},
		{"投资收益", l1Income, "equity", 1},
		{"土地流转费收入", l1Income, "equity", 2},
		{"流转管理费", l1Income, "equity", 3},
		{"其他收入", l1Income, "equity", 4},
		{"土地流转费-转付农户", l1Dist, "equity", 1},
		{"成员分红", l1Dist, "equity", 2},
		{"福利发放", l1Dist, "equity", 3},
		{"公益支出", l1Dist, "equity", 4},
		{"管理费支出", l1Dist, "equity", 5},
	}
	for _, c := range presetL2 {
		if _, err := insert(c.name, 2, c.parentID, c.kind, c.sort); err != nil {
			return fmt.Errorf("预置二级科目失败(%s): %w", c.name, err)
		}
	}
	// 长期投资一级留空：使用时由操作者在快速记账「投资给公司」里按公司建普通二级（自动建往来单位）
	_ = l1Invest
	return nil
}
