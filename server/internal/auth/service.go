// Package auth 负责记账人身份认证与会话：口令哈希校验、会话签发与校验。
//
// 设计依据：03-design §4.3 接口清单（1、2 项）、PRD 登录需求。
// 单人系统，无角色与权限分级；会话存库而非 JWT（单容器下 JWT 的无状态优势不成立，
// 且服务端会话可即时失效）。
//
// 分层约定（与 category / transaction 等包一致）：service 只编排业务规则，
// SQL 全部收敛在 repo。
package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidCredentials 账号或口令错误（对外不区分，避免账号枚举）。
var ErrInvalidCredentials = errors.New("账号或密码错误")

// ErrUnauthorized 未登录或会话已过期。
var ErrUnauthorized = errors.New("未登录或登录已过期")

const (
	// SessionTTL 会话有效期。局域网自用场景，7 天免反复登录。
	SessionTTL = 7 * 24 * time.Hour
	// CookieName 会话 Cookie 名。
	CookieName = "jt_session"
)

// Service 提供认证与会话能力。
type Service struct {
	repo *Repo
}

// NewService 创建认证服务。
func NewService(db *sql.DB) *Service {
	return &Service{repo: NewRepo(db)}
}

// EnsureInitialUser 在库中尚无用户时创建初始账号；已有用户则直接返回。
// 用于首次部署引导（口令由环境变量注入，不内置默认口令）。
func (s *Service) EnsureInitialUser(username, password string) error {
	n, err := s.repo.CountUsers()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.repo.CreateUser(username, string(hash))
	return err
}

// Login 校验账号口令，成功则签发会话，返回会话 token 与过期时间。
func (s *Service) Login(username, password string) (string, time.Time, error) {
	u, err := s.repo.FindByUsername(username)
	if err != nil {
		return "", time.Time{}, err
	}
	if u == nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return "", time.Time{}, ErrInvalidCredentials
	}

	token, err := newToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().UTC().Add(SessionTTL)
	if err := s.repo.CreateSession(token, u.ID, expiresAt); err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

// Resolve 校验会话 token，返回用户 id。无效或过期返回 ErrUnauthorized。
func (s *Service) Resolve(token string) (int64, error) {
	if token == "" {
		return 0, ErrUnauthorized
	}
	sess, err := s.repo.FindSession(token)
	if err != nil {
		return 0, err
	}
	if sess == nil {
		return 0, ErrUnauthorized
	}
	return sess.UserID, nil
}

// Logout 删除会话（登出）。token 为空时视为无操作。
func (s *Service) Logout(token string) error {
	if token == "" {
		return nil
	}
	return s.repo.DeleteSession(token)
}

// newToken 生成 32 字节随机 token（十六进制字符串）。
func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
