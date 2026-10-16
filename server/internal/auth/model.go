// Package auth 提供登录、登出与会话中间件。
package auth

import "time"

// 角色（由高到低）。owner 唯一，由注册首个账号产生；admin/viewer 由 owner/admin 后台创建。
const (
	RoleOwner  = "owner"  // 所有者 / 超管：全功能 + 最高危操作
	RoleAdmin  = "admin"  // 高级用户：全功能日常操作 + 管理一般用户
	RoleViewer = "viewer" // 一般用户：只读（可查看/导出，不可写、不可进管理页）
)

// 账号状态（软停用，不做物理删除以保留历史归属）。
const (
	StatusActive   = "active"   // 启用
	StatusDisabled = "disabled" // 已停用
)

// Org 对应 org 表（集体经济组织，数据隔离单元）。
type Org struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// User 对应 user 表。
type User struct {
	ID           int64     `json:"id"`
	OrgID        int64     `json:"orgId"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`   // owner / admin / viewer
	Status       string    `json:"status"` // active / disabled
	CreatedAt    time.Time `json:"createdAt"`
}

// Identity 是一次会话校验的结果（写入 gin 上下文，供鉴权中间件与业务层读取）。
type Identity struct {
	UserID   int64
	OrgID    int64
	Username string
	Role     string
}

// Session 对应 session 表。
type Session struct {
	ID        string    `json:"id"`
	UserID    int64     `json:"userId"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// LoginRequest 登录请求体。
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest 注册请求体（组织名 + 管理员账号，见 v0.3 F8）。
type RegisterRequest struct {
	OrgName  string `json:"orgName" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// CreateUserRequest 新建账号请求体（用户管理页）。
type CreateUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role" binding:"required"`
}

// UpdateUserRequest 修改账号请求体（角色/状态，均可选）。
type UpdateUserRequest struct {
	Role   *string `json:"role"`
	Status *string `json:"status"`
}
