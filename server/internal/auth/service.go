// Package auth 负责身份认证与会话：口令哈希校验、会话签发与校验、组织注册。
//
// 设计依据：03-design §4.3 接口清单与 §9（v0.3 多组织）；PRD F8 注册。
// 多组织模型：每个账号唯一绑定一个组织（org），数据隔离以会话携带的 orgID 为准。
//
// 分层约定：service 编排业务规则，SQL 全部收敛在 repo。

package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"jititaizhang/server/internal/platform"
)

// 业务错误。
var (
	ErrInvalidCredentials = errors.New("账号或密码错误")
	ErrUnauthorized       = errors.New("未登录或登录已过期")
	ErrUsernameTaken      = errors.New("该账号已存在，请更换")
	ErrInvalidOrgName     = errors.New("请填写组织名称")
	ErrInvalidPassword    = errors.New("密码至少 6 位")
	ErrOldPasswordWrong   = errors.New("原密码错误")
	ErrRegistrationClosed = errors.New("系统已注册，禁止重复注册")
	// 多用户 / 权限相关。
	ErrResetNotAllowed = errors.New("系统存在多个所有者账号，请登录后由管理员重置密码")
	ErrUserNotFound    = errors.New("账号不存在")
	ErrInvalidRole     = errors.New("角色不合法")
	ErrInvalidUsername = errors.New("请填写账号")
	ErrForbiddenRole   = errors.New("无权操作该账号")
	ErrSelfOperation   = errors.New("不能对自己执行该操作")
	ErrLastOwner       = errors.New("必须保留至少一个所有者账号")
)

const (
	// SessionTTL 会话有效期。局域网自用场景，7 天免反复登录。
	SessionTTL = 7 * 24 * time.Hour
	// CookieName 会话 Cookie 名。
	CookieName = "jt_session"
	// MinPasswordLen 注册口令最小长度。
	MinPasswordLen = 6
)

// Service 提供认证、注册与会话能力。
type Service struct {
	repo *Repo
}

// NewService 创建认证服务。
func NewService(db *sql.DB) *Service {
	return &Service{repo: NewRepo(db)}
}

// RegisterOrg 自助注册组织（v0.3 F8）：
// 单事务创建 组织 + 管理员账号，并初始化预置科目；成功即签发会话（自动登录）。
func (s *Service) RegisterOrg(orgName, username, password string) (string, time.Time, error) {
	orgName = strings.TrimSpace(orgName)
	username = strings.TrimSpace(username)
	if orgName == "" {
		return "", time.Time{}, ErrInvalidOrgName
	}
	if username == "" {
		return "", time.Time{}, errors.New("请填写管理员账号")
	}
	if len(password) < MinPasswordLen {
		return "", time.Time{}, ErrInvalidPassword
	}

	// 单用户限制：已有用户则禁止重复注册
	n, err := s.repo.CountUsers()
	if err != nil {
		return "", time.Time{}, err
	}
	if n > 0 {
		return "", time.Time{}, ErrRegistrationClosed
	}

	// 用户名预检，给出友好错误（唯一索引兜底）
	existing, err := s.repo.FindByUsername(username)
	if err != nil {
		return "", time.Time{}, err
	}
	if existing != nil {
		return "", time.Time{}, ErrUsernameTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", time.Time{}, err
	}

	var orgID, userID int64
	err = s.repo.WithTx(func(tx *sql.Tx) error {
		orgID, err = s.repo.createOrgTx(tx, orgName)
		if err != nil {
			return err
		}
		userID, err = s.repo.createUserTx(tx, orgID, username, string(hash), RoleOwner)
		if err != nil {
			return err
		}
		return s.repo.seedPresetCategoriesTx(tx, orgID)
	})
	if err != nil {
		return "", time.Time{}, err
	}

	token, err := newToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := platform.Now().Add(SessionTTL)
	if err := s.repo.CreateSession(token, userID, expiresAt); err != nil {
		return "", time.Time{}, err
	}
	_ = orgID
	return token, expiresAt, nil
}

// IsRegistrationOpen 报告是否仍允许注册：系统中尚无任何用户时为 true。
// 与 RegisterOrg 的关闸条件（CountUsers > 0 即拒绝）完全同源，避免前后端判断不一致。
func (s *Service) IsRegistrationOpen() (bool, error) {
	n, err := s.repo.CountUsers()
	if err != nil {
		return false, err
	}
	return n == 0, nil
}

// Login 校验账号口令，成功则签发会话并返回账号角色。
// 已停用账号与账号不存在/密码错误返回同一错误，避免账号枚举与状态探测。
func (s *Service) Login(username, password string) (string, time.Time, string, error) {
	u, err := s.repo.FindByUsername(strings.TrimSpace(username))
	if err != nil {
		return "", time.Time{}, "", err
	}
	if u == nil || u.Status == StatusDisabled ||
		bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return "", time.Time{}, "", ErrInvalidCredentials
	}

	token, err := newToken()
	if err != nil {
		return "", time.Time{}, "", err
	}
	expiresAt := platform.Now().Add(SessionTTL)
	if err := s.repo.CreateSession(token, u.ID, expiresAt); err != nil {
		return "", time.Time{}, "", err
	}
	return token, expiresAt, u.Role, nil
}

// Resolve 校验会话 token，返回会话身份（用户/组织/用户名/角色）。无效或过期返回 ErrUnauthorized。
func (s *Service) Resolve(token string) (*Identity, error) {
	if token == "" {
		return nil, ErrUnauthorized
	}
	sess, err := s.repo.FindSession(token)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, ErrUnauthorized
	}
	u, err := s.repo.FindUserByID(sess.UserID)
	if err != nil {
		return nil, err
	}
	if u == nil || u.Status == StatusDisabled {
		return nil, ErrUnauthorized
	}
	return &Identity{UserID: u.ID, OrgID: u.OrgID, Username: u.Username, Role: u.Role}, nil
}

// Logout 删除会话（登出）。token 为空时视为无操作。
func (s *Service) Logout(token string) error {
	if token == "" {
		return nil
	}
	return s.repo.DeleteSession(token)
}

// ResetPassword 将密码重置为 admin888（忘记密码兜底）。
// 多用户后公开接口不再允许「重置任意用户」：仅当系统内恰好存在一个 owner 账号时，
// 才重置该 owner；存在多个 owner（或没有任何账号）时返回 ErrResetNotAllowed。
func (s *Service) ResetPassword() (string, error) {
	n, err := s.repo.CountOwners()
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", ErrUserNotFound
	}
	if n > 1 {
		return "", ErrResetNotAllowed
	}
	u, err := s.repo.FindOwner()
	if err != nil {
		return "", err
	}
	if u == nil {
		return "", ErrResetNotAllowed
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("admin888"), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	if err := s.repo.UpdatePassword(u.ID, string(hash)); err != nil {
		return "", err
	}
	// 重置密码后强制下线，避免旧会话继续可用。
	if err := s.repo.DeleteSessionsByUser(u.ID); err != nil {
		return "", err
	}
	return u.Username, nil
}

// ChangePassword 校验原口令后更新新口令哈希（修改密码）。
func (s *Service) ChangePassword(userID int64, oldPassword, newPassword string) error {
	if len(newPassword) < MinPasswordLen {
		return ErrInvalidPassword
	}
	u, err := s.repo.FindUserByID(userID)
	if err != nil {
		return err
	}
	if u == nil {
		return ErrUnauthorized
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPassword)) != nil {
		return ErrOldPasswordWrong
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdatePassword(userID, string(hash))
}

// newToken 生成 32 字节随机 token（十六进制字符串）。
func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
