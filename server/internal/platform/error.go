// Package platform 提供统一的错误处理与响应格式。
package platform

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AppError 是业务错误，携带机器可读的 code 和面向用户的中文 message。
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// Error 实现 error 接口。
func (e *AppError) Error() string { return e.Message }

// 预定义业务错误
var (
	ErrUnauthorized       = &AppError{Code: "UNAUTHORIZED", Message: "请先登录"}
	ErrSessionExpired     = &AppError{Code: "SESSION_EXPIRED", Message: "登录已过期，请重新登录"}
	ErrCategoryNotFound   = &AppError{Code: "CATEGORY_NOT_FOUND", Message: "科目不存在"}
	ErrCategoryInUse      = &AppError{Code: "CATEGORY_IN_USE", Message: "该科目已被流水引用，无法删除"}
	ErrCategoryHasChild   = &AppError{Code: "CATEGORY_HAS_CHILD", Message: "该一级科目下仍有二级科目，请先处理"}
	ErrCategoryNameDup    = &AppError{Code: "CATEGORY_NAME_DUP", Message: "已存在同名科目"}
	ErrCategoryLevel2Only = &AppError{Code: "CATEGORY_LEVEL2_ONLY", Message: "流水必须关联二级科目"}
	ErrInvalidAmount      = &AppError{Code: "INVALID_AMOUNT", Message: "金额必须大于 0"}
	ErrCategoryRequired   = &AppError{Code: "CATEGORY_REQUIRED", Message: "请选择科目"}
	ErrInvalidDirection   = &AppError{Code: "INVALID_DIRECTION", Message: "收支方向不合法"}
	ErrTransactionNotFound = &AppError{Code: "TRANSACTION_NOT_FOUND", Message: "流水不存在"}
	ErrUsernameExists     = &AppError{Code: "USERNAME_EXISTS", Message: "账号已存在"}
	ErrInvalidCredentials = &AppError{Code: "INVALID_CREDENTIALS", Message: "账号或密码错误"}
	ErrNoCategory         = &AppError{Code: "NO_CATEGORY", Message: "尚无科目，请先创建科目"}
)

// ErrResponse 发送统一错误响应。
func ErrResponse(c *gin.Context, status int, appErr *AppError) {
	c.JSON(status, gin.H{"error": appErr})
}

// SuccessResponse 发送统一成功响应。
func SuccessResponse(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}