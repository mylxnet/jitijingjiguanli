package platform

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIError 统一错误响应体。约定见 03-design §4.4。
type APIError struct {
	Code    string      `json:"code"`              // 机器可读错误码
	Message string      `json:"message"`           // 面向用户的中文提示
	Details interface{} `json:"details,omitempty"` // 可选，供前端渲染
}

// ErrorResponse 是失败响应的外层结构。
type ErrorResponse struct {
	Error APIError `json:"error"`
}

// Fail 以统一格式返回错误响应。
func Fail(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorResponse{Error: APIError{Code: code, Message: message}})
}

// OK 返回 { "data": payload }。
func OK(c *gin.Context, payload interface{}) {
	c.JSON(http.StatusOK, gin.H{"data": payload})
}
