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
func (r *Repo) CreateUser(orgID int64, username, passwordHash, role string) (*User, error) {
	now := platform.Now()
	res, err := r.db.Exec(
		`INSERT INTO user(org_id, username, password_hash, role, status, created_at)
		 VALUES(?, ?, ?, ?, ?, ?)`,
		orgID, username, passwordHash, role, StatusActive, now,
	)
	if err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}
	id, _ := res.LastInsertId()
	return &User{ID: id, OrgID: orgID, Username: username, Role: role, Status: StatusActive, CreatedAt: now}, nil
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
	return r.findUser(`SELECT id, org_id, username, password_hash, role, status, created_at
		FROM user WHERE username = ?`, username)
}

// FindUserByID 按 ID 查找用户（含组织归属）。
func (r *Repo) FindUserByID(id int64) (*User, error) {
	return r.findUser(`SELECT id, org_id, username, password_hash, role, status, created_at
		FROM user WHERE id = ?`, id)
}

// findUser 用户行查询公共实现（无匹配返回 nil, nil）。
func (r *Repo) findUser(query string, args ...any) (*User, error) {
	u := &User{}
	err := r.db.QueryRow(query, args...).Scan(
		&u.ID, &u.OrgID, &u.Username, &u.PasswordHash, &u.Role, &u.Status, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查找用户失败: %w", err)
	}
	return u, nil
}

// ListByOrg 列出某组织全部账号（含角色/状态），按创建时间正序。
func (r *Repo) ListByOrg(orgID int64) ([]*User, error) {
	rows, err := r.db.Query(`SELECT id, org_id, username, role, status, created_at
		FROM user WHERE org_id = ? ORDER BY id ASC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("查询账号列表失败: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(&u.ID, &u.OrgID, &u.Username, &u.Role, &u.Status, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("扫描账号行失败: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// CountActiveOwners 统计启用状态的 owner 数量（最后一个 owner 保护判据）。
func (r *Repo) CountActiveOwners() (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM user WHERE role = ? AND status = ?`,
		RoleOwner, StatusActive).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("统计所有者数量失败: %w", err)
	}
	return n, nil
}

// CountOwners 统计 owner 角色账号数（忘记密码「仅单 owner 时可重置」判据）。
func (r *Repo) CountOwners() (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM user WHERE role = ?`, RoleOwner).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("统计所有者数量失败: %w", err)
	}
	return n, nil
}

// FindOwner 返回唯一 owner 账号；不存在或不唯一时返回 nil（忘记密码兜底）。
func (r *Repo) FindOwner() (*User, error) {
	rows, err := r.db.Query(`SELECT id, org_id, username, password_hash, role, status, created_at
		FROM user WHERE role = ? ORDER BY id ASC`, RoleOwner)
	if err != nil {
		return nil, fmt.Errorf("查找所有者失败: %w", err)
	}
	defer rows.Close()

	var u *User
	for rows.Next() {
		cur := &User{}
		if err := rows.Scan(&cur.ID, &cur.OrgID, &cur.Username, &cur.PasswordHash, &cur.Role, &cur.Status, &cur.CreatedAt); err != nil {
			return nil, fmt.Errorf("扫描所有者行失败: %w", err)
		}
		if u != nil {
			return nil, nil // 多个 owner，调用方按「不唯一」处理
		}
		u = cur
	}
	return u, rows.Err()
}

// UpdateRoleStatus 更新账号角色与状态（用户管理）。
func (r *Repo) UpdateRoleStatus(userID int64, role, status string) error {
	if _, err := r.db.Exec(`UPDATE user SET role = ?, status = ? WHERE id = ?`,
		role, status, userID); err != nil {
		return fmt.Errorf("更新账号失败: %w", err)
	}
	return nil
}

// DeleteSessionsByUser 删除账号的全部会话（停用/重置密码后强制下线，见方案 §8.3）。
func (r *Repo) DeleteSessionsByUser(userID int64) error {
	if _, err := r.db.Exec(`DELETE FROM session WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("清除会话失败: %w", err)
	}
	return nil
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

// createUserTx 事务内建用户（注册首人固定为 owner）。
func (r *Repo) createUserTx(tx *sql.Tx, orgID int64, username, passwordHash, role string) (int64, error) {
	now := platform.Now()
	res, err := tx.Exec(
		`INSERT INTO user(org_id, username, password_hash, role, status, created_at)
		 VALUES(?, ?, ?, ?, ?, ?)`,
		orgID, username, passwordHash, role, StatusActive, now,
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
