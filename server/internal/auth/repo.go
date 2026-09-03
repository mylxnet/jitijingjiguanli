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

// seedPresetCategoriesTx 事务内写入预置科目（v0.3 F12，五件套 + 常用二级）。
func (r *Repo) seedPresetCategoriesTx(tx *sql.Tx, orgID int64) error {
	now := platform.Now()
	// (name, level, parentOrder 引用自身 sort，用占位再更新) —— 直接先插一级收集 id
	insert := func(name string, level int, parentID any, bt, kind string, sort int, preset int) (int64, error) {
		res, err := tx.Exec(
			`INSERT INTO category(org_id, name, level, parent_id, status, balance_type, kind,
				opening_balance_cents, include_in_reconciliation, preset, sort_order, created_at, updated_at)
			 VALUES(?,?,?,?, 'active', ?,?, 0,0,?,?,?,?)`,
			orgID, name, level, parentID, bt, kind, preset, sort, now, now,
		)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}

	// 一级五件套（v0.3 预置科目，用户审定命名）
	l1Fund, err := insert("本金", 1, nil, "residual", "normal", 1, 1)
	if err != nil {
		return fmt.Errorf("预置科目失败(本金): %w", err)
	}
	if _, err := insert("对外投资", 1, nil, "residual", "normal", 2, 1); err != nil {
		return fmt.Errorf("预置科目失败(对外投资): %w", err)
	}
	l1Income, err := insert("经营收入", 1, nil, "residual", "normal", 3, 1)
	if err != nil {
		return fmt.Errorf("预置科目失败(经营收入): %w", err)
	}
	l1Dist, err := insert("收益分配", 1, nil, "spending", "normal", 4, 1)
	if err != nil {
		return fmt.Errorf("预置科目失败(收益分配): %w", err)
	}
	if _, err := insert("公益支出", 1, nil, "spending", "normal", 5, 1); err != nil {
		return fmt.Errorf("预置科目失败(公益支出): %w", err)
	}

	// 二级预设（经营收入 / 收益分配 下）
	presetL2 := []struct {
		name     string
		parentID int64
		bt       string
		sort     int
	}{
		{"土地流转费收入", l1Income, "residual", 1},
		{"投资分红收益", l1Income, "residual", 2},
		{"其他收入", l1Income, "residual", 3},
		{"收益分红发放", l1Dist, "spending", 1},
		{"流转费分发", l1Dist, "spending", 2},
		{"福利发放", l1Dist, "spending", 3},
	}
	for _, c := range presetL2 {
		if _, err := insert(c.name, 2, c.parentID, c.bt, "normal", c.sort, 1); err != nil {
			return fmt.Errorf("预置二级科目失败(%s): %w", c.name, err)
		}
	}
	// 本金 / 对外投资 / 公益支出 的二级留空，由使用者按拨款项目/投资项目/用途自建
	_ = l1Fund
	return nil
}
