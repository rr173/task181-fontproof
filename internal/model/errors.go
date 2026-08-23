// Package model 定义字体回退覆盖证明工作台的核心实体与领域错误。
package model

import (
	"errors"
	"fmt"
)

// 领域错误：业务规则拒绝时返回给 HTTP 层的错误码与语义。
var (
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("version conflict")
	ErrInvalidState    = errors.New("invalid state transition")
	ErrDuplicate       = errors.New("duplicate entity")
	ErrForbidden       = errors.New("operation forbidden on published entity")
	ErrCycleDetected   = errors.New("fallback cycle detected")
	ErrMutuallyExclus  = errors.New("mutually exclusive rules at same priority")
	ErrMissingScript   = errors.New("required script has no font coverage")
	ErrGraphemeSplit   = errors.New("grapheme cluster must not be split")
	ErrEmptyCoverage   = errors.New("font covers no codepoints")
	ErrBadRequest      = errors.New("bad request")
	ErrChecksumMismatch = errors.New("rule set checksum mismatch")
)

// DomainError 携带稳定的错误类型，供 HTTP 层映射为 4xx/5xx。
type DomainError struct {
	Kind string
	Msg  string
	Err  error
}

func (e *DomainError) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "domain error"
}

func (e *DomainError) Unwrap() error { return e.Err }

// E 构造领域错误。
func E(kind string, err error, msg string) *DomainError {
	return &DomainError{Kind: kind, Err: err, Msg: msg}
}

// ENotFound / EConflict 等快捷构造。
func ENotFound(what string) *DomainError {
	return E("not_found", ErrNotFound, fmt.Sprintf("%s not found", what))
}

func EConflict(msg string) *DomainError {
	return E("conflict", ErrConflict, msg)
}

func EInvalidState(msg string) *DomainError {
	return E("invalid_state", ErrInvalidState, msg)
}

func EForbidden(msg string) *DomainError {
	return E("forbidden", ErrForbidden, msg)
}

func EBadRequest(msg string) *DomainError {
	return E("bad_request", ErrBadRequest, msg)
}
