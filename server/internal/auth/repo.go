package auth

import (
	"database/sql"
	"fmt"
	"time"

	"jititaizhang/server/internal/platform"
)

// Repo 封装 user 与 session 表的 SQL 查询。
type Repo struct {
	db *sql.DB
}

// NewRepo 创建 Repo。
func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// CountUsers 返回用户总数（用于初始化引导判断）。
func (r *Repo) CountUsers() (int, error) {
	var n int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM user`).Scan(&n); err != nil {
		return 0, fmt.Errorf("统计用户失败: %w", err)
	}
	return n, nil
}

// FindByUsername 按用户名查找用户。
func (r *Repo) FindByUsername(username string) (*User, error) {
	u := &User{}
	err := r.db.QueryRow(
		`SELECT id, username, password_hash, created_at FROM user WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查找用户失败: %w", err)
	}
	return u, nil
}

// CreateUser 创建新用户（仅初始化用）。
func (r *Repo) CreateUser(username, passwordHash string) (*User, error) {
	now := platform.Now()
	res, err := r.db.Exec(
		`INSERT INTO user(username, password_hash, created_at) VALUES(?, ?, ?)`,
		username, passwordHash, now,
	)
	if err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}
	id, _ := res.LastInsertId()
	return &User{ID: id, Username: username, CreatedAt: now}, nil
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