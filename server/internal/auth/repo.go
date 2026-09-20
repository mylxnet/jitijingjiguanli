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

// CountUsers 统计用户总数（用于单用户注册限制）。
func (r *Repo) CountUsers() (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM user`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("统计用户数失败: %w", err)
	}
	return n, nil
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

// FindAnyUser 返回系统中任意一个用户（用于重置密码）。
func (r *Repo) FindAnyUser() (*User, error) {
	u := &User{}
	err := r.db.QueryRow(
		`SELECT id, org_id, username, password_hash, created_at FROM user ORDER BY id ASC LIMIT 1`,
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

// seedPresetCategoriesTx 事务内写入预置科目。
// 共 9 个一级，按 sort 排序：
//   1 本金(空)  2 长期投资(空)  3 再投资(空)  4 经营收入(其他财政收入/其他收入)
//   5 投资收益(空)  6 再投资收益(空)  7 土地流转费收入(空)  8 流转管理费(空)  9 分配与支出(5个L2)
// 空容器在业务操作时按往来单位名自动建二级（invest→长期投资；flow→土地流转费收入+流转管理费；
// 收益核销入账容器按 recv_kind 定位：dividend→投资收益，reinvest_dividend→再投资收益）。
// 预置科目 preset=1，handler 层禁止删除和重命名（可停用、可改期初）。
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

	// 一级分组（9 个 equity 容器）
	l1Fund, _ := insert("本金", 1, nil, "equity", 1)
	_, _ = insert("长期投资", 1, nil, "equity", 2)
	_, _ = insert("再投资", 1, nil, "equity", 3)
	l1Income, _ := insert("经营收入", 1, nil, "equity", 4)
	_, _ = insert("投资收益", 1, nil, "equity", 5)
	_, _ = insert("再投资收益", 1, nil, "equity", 6)
	_, _ = insert("土地流转费收入", 1, nil, "equity", 7)
	_, _ = insert("流转管理费", 1, nil, "equity", 8)
	l1Dist, _ := insert("分配与支出", 1, nil, "equity", 9)

	// 二级预设（长期投资/再投资/投资收益/再投资收益/土地流转费收入/流转管理费 六个 L1 留空，业务时自动建）
	presetL2 := []struct {
		name     string
		parentID int64
		kind     string
		sort     int
	}{
		{"上级补助", l1Fund, "equity", 1},
		{"待投资", l1Fund, "equity", 2},
		{"其他财政收入", l1Income, "equity", 1},
		{"其他收入", l1Income, "equity", 2},
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
	return nil
}
