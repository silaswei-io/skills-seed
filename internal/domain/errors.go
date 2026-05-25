// Package domain 提供核心领域模型和业务规则
package domain

import (
	"errors"
	"fmt"
)

// ErrorCode 错误代码
type ErrorCode string

const (
	// 客户端错误
	ErrNotFound     ErrorCode = "NOT_FOUND"    // 资源未找到
	ErrInvalid      ErrorCode = "INVALID"      // 无效的输入或参数
	ErrConflict     ErrorCode = "CONFLICT"     // 资源冲突
	ErrUnauthorized ErrorCode = "UNAUTHORIZED" // 未授权

	// 服务端错误
	ErrInternal    ErrorCode = "INTERNAL"    // 内部错误
	ErrTimeout     ErrorCode = "TIMEOUT"     // 超时
	ErrUnavailable ErrorCode = "UNAVAILABLE" // 服务不可用

	// 外部依赖错误
	ErrGitOperation ErrorCode = "GIT_ERROR" // Git 操作失败
	ErrAIService    ErrorCode = "AI_ERROR"  // AI 服务错误
	ErrDatabase     ErrorCode = "DB_ERROR"  // 数据库错误
)

// DomainError 领域错误
type DomainError struct {
	Code    ErrorCode              // 错误代码
	Message string                 // 错误消息
	Cause   error                  // 原始错误
	Context map[string]interface{} // 上下文信息
}

// Error 实现 error 接口
func (e *DomainError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 实现错误包装
func (e *DomainError) Unwrap() error {
	return e.Cause
}

// NewDomainError 创建领域错误
func NewDomainError(code ErrorCode, message string, cause error) *DomainError {
	return &DomainError{
		Code:    code,
		Message: message,
		Cause:   cause,
		Context: make(map[string]interface{}),
	}
}

// WithContext 添加上下文信息
func (e *DomainError) WithContext(key string, value interface{}) *DomainError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// === 错误类型检查函数 ===

// IsNotFound 检查是否为"未找到"错误
func IsNotFound(err error) bool {
	var domainErr *DomainError
	if err == nil {
		return false
	}
	if errors.As(err, &domainErr) {
		return domainErr.Code == ErrNotFound
	}
	return false
}

// IsTimeout 检查是否为超时错误
func IsTimeout(err error) bool {
	var domainErr *DomainError
	if err == nil {
		return false
	}
	if errors.As(err, &domainErr) {
		return domainErr.Code == ErrTimeout
	}
	return false
}

// IsInvalid 检查是否为无效输入错误
func IsInvalid(err error) bool {
	var domainErr *DomainError
	if err == nil {
		return false
	}
	if errors.As(err, &domainErr) {
		return domainErr.Code == ErrInvalid
	}
	return false
}

// IsConflict 检查是否为冲突错误
func IsConflict(err error) bool {
	var domainErr *DomainError
	if err == nil {
		return false
	}
	if errors.As(err, &domainErr) {
		return domainErr.Code == ErrConflict
	}
	return false
}

// IsInternal 检查是否为内部错误
func IsInternal(err error) bool {
	var domainErr *DomainError
	if err == nil {
		return false
	}
	if errors.As(err, &domainErr) {
		return domainErr.Code == ErrInternal
	}
	return false
}

// IsAIServiceError 检查是否为 AI 服务错误
func IsAIServiceError(err error) bool {
	var domainErr *DomainError
	if err == nil {
		return false
	}
	if errors.As(err, &domainErr) {
		return domainErr.Code == ErrAIService
	}
	return false
}

// GetErrorCode 获取错误代码
func GetErrorCode(err error) ErrorCode {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Code
	}
	return ErrInternal
}

// GetErrorContext 获取错误上下文
func GetErrorContext(err error) map[string]interface{} {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Context
	}
	return nil
}
