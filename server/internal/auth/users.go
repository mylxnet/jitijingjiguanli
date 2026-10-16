// 账号管理（多用户 + 角色权限，见 docs/27-多用户与权限控制方案.md §5.3）。
//
// 越权约束（后端强制，前端隐藏只是体验）：
//   - admin 只能创建/管理 viewer，不能创建或改动 admin/owner；
//   - owner 可管理任何人，但不能对自己降级/停用；
//   - 任何操作都不得让系统失去最后一个启用的 owner。

package auth

import (
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// ListUsers 列出本组织全部账号（含角色/状态）。
func (s *Service) ListUsers(orgID int64) ([]*User, error) {
	return s.repo.ListByOrg(orgID)
}

// CreateUser 创建账号。admin 仅能建 viewer；owner 可建 viewer/admin（owner 唯一，不新增）。
func (s *Service) CreateUser(actorRole string, orgID int64, username, password, role string) (*User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, ErrInvalidUsername
	}
	if len(password) < MinPasswordLen {
		return nil, ErrInvalidPassword
	}
	if role != RoleViewer && role != RoleAdmin {
		return nil, ErrInvalidRole
	}
	if actorRole != RoleOwner && role != RoleViewer {
		return nil, ErrForbiddenRole
	}

	existing, err := s.repo.FindByUsername(username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrUsernameTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateUser(orgID, username, string(hash), role)
}

// UpdateUser 修改账号角色/状态（role、status 均可选，nil 表示不变）。
// 涉及停用或降级时强制下线并保护最后一个 owner。
func (s *Service) UpdateUser(actor *Identity, orgID, targetID int64, role, status *string) (*User, error) {
	target, err := s.loadTarget(actor, orgID, targetID)
	if err != nil {
		return nil, err
	}
	if target.ID == actor.UserID {
		return nil, ErrSelfOperation
	}

	newRole, newStatus := target.Role, target.Status
	if role != nil {
		if *role != RoleOwner && *role != RoleAdmin && *role != RoleViewer {
			return nil, ErrInvalidRole
		}
		newRole = *role
	}
	if status != nil {
		if *status != StatusActive && *status != StatusDisabled {
			return nil, ErrInvalidRole
		}
		newStatus = *status
	}

	// admin 只能操作 viewer，且只能改状态（角色变更仅 owner 可做）。
	if actor.Role != RoleOwner {
		if target.Role != RoleViewer || newRole != RoleViewer {
			return nil, ErrForbiddenRole
		}
	}

	// 最后一个 owner 保护：不得降级或停用（防把系统锁死）。
	if target.Role == RoleOwner && (newRole != RoleOwner || newStatus == StatusDisabled) {
		if err := s.ensureOwnerRemains(); err != nil {
			return nil, err
		}
	}

	if err := s.repo.UpdateRoleStatus(target.ID, newRole, newStatus); err != nil {
		return nil, err
	}
	if newStatus == StatusDisabled {
		if err := s.repo.DeleteSessionsByUser(target.ID); err != nil {
			return nil, err
		}
	}
	target.Role, target.Status = newRole, newStatus
	return target, nil
}

// ResetUserPassword 将指定账号密码重置为初始密码 admin888，并强制其下线。
// admin 仅能重置 viewer 的密码；owner 可重置任何其他账号。
func (s *Service) ResetUserPassword(actor *Identity, orgID, targetID int64) error {
	target, err := s.loadTarget(actor, orgID, targetID)
	if err != nil {
		return err
	}
	if target.ID == actor.UserID {
		return ErrSelfOperation
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("admin888"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := s.repo.UpdatePassword(target.ID, string(hash)); err != nil {
		return err
	}
	return s.repo.DeleteSessionsByUser(target.ID)
}

// loadTarget 取出目标账号并做归属/越权校验（跨组织访问一律视为不存在）。
func (s *Service) loadTarget(actor *Identity, orgID, targetID int64) (*User, error) {
	target, err := s.repo.FindUserByID(targetID)
	if err != nil {
		return nil, err
	}
	if target == nil || target.OrgID != orgID {
		return nil, ErrUserNotFound
	}
	// admin 只能操作 viewer（owner 可操作除自己外的任何人）。
	if actor.Role != RoleOwner && target.Role != RoleViewer {
		return nil, ErrForbiddenRole
	}
	return target, nil
}

// ensureOwnerRemains 校验停用/降级后系统仍至少有一个启用的 owner。
func (s *Service) ensureOwnerRemains() error {
	n, err := s.repo.CountActiveOwners()
	if err != nil {
		return err
	}
	if n <= 1 {
		return ErrLastOwner
	}
	return nil
}
