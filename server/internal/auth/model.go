// Package auth 提供登录、登出与会话中间件。
package auth

import "time"

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
	CreatedAt    time.Time `json:"createdAt"`
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
