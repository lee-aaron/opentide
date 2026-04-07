// Package errors provides structured error types for OpenTide.
// Every error carries a code, human-readable message, cause, fix suggestion, and docs link.
package errors

import "fmt"

// Code identifies the error category. Codes are stable across versions.
type Code string

const (
	// Config errors
	CodeConfigInvalid  Code = "CONFIG_INVALID"
	CodeConfigMissing  Code = "CONFIG_MISSING"
	CodeConfigEnvEmpty Code = "CONFIG_ENV_EMPTY"

	// Provider errors
	CodeProviderAuth    Code = "PROVIDER_AUTH"
	CodeProviderRate    Code = "PROVIDER_RATE_LIMITED"
	CodeProviderTimeout Code = "PROVIDER_TIMEOUT"
	CodeProviderError   Code = "PROVIDER_ERROR"

	// Adapter errors
	CodeAdapterConnect  Code = "ADAPTER_CONNECT"
	CodeAdapterSend     Code = "ADAPTER_SEND"
	CodeAdapterReceive  Code = "ADAPTER_RECEIVE"
	CodeAdapterInput    Code = "ADAPTER_INPUT_INVALID"
	CodeAdapterRateLim  Code = "ADAPTER_RATE_LIMITED"

	// Approval errors
	CodeApprovalDenied  Code = "APPROVAL_DENIED"
	CodeApprovalExpired Code = "APPROVAL_EXPIRED"
	CodeApprovalHash    Code = "APPROVAL_HASH_MISMATCH"
	CodeApprovalScope   Code = "APPROVAL_SCOPE_VIOLATION"

	// Skill errors
	CodeSkillNotFound   Code = "SKILL_NOT_FOUND"
	CodeSkillTimeout    Code = "SKILL_TIMEOUT"
	CodeSkillEgress     Code = "SKILL_EGRESS_BLOCKED"
	CodeSkillSignature  Code = "SKILL_SIGNATURE_INVALID"
	CodeSkillSandbox    Code = "SKILL_SANDBOX_VIOLATION"

	// State errors
	CodeStateConnect Code = "STATE_DB_CONNECT"
	CodeStateQuery   Code = "STATE_DB_QUERY"

	// Security errors
	CodeSecurityHash    Code = "SECURITY_HASH_FAILURE"
	CodeSecuritySign    Code = "SECURITY_SIGN_FAILURE"
	CodeSecurityVerify  Code = "SECURITY_VERIFY_FAILURE"

	// Admin API errors
	CodeAdminAuthRequired  Code = "ADMIN_AUTH_REQUIRED"
	CodeAdminAuthInvalid   Code = "ADMIN_AUTH_INVALID"
	CodeAdminRateLimited   Code = "ADMIN_RATE_LIMITED"
	CodeAdminBadRequest    Code = "ADMIN_BAD_REQUEST"
	CodeAdminNotFound      Code = "ADMIN_NOT_FOUND"
	CodeAdminConflict      Code = "ADMIN_CONFLICT"
	CodeAdminInternal      Code = "ADMIN_INTERNAL"
)

// Error is the structured error type for OpenTide.
// Every public-facing error should use this type.
type Error struct {
	Code    Code   // Stable error code
	Message string // Human-readable description
	Cause   error  // Underlying error (may be nil)
	Fix     string // Suggested fix for the user
	DocsURL string // Link to relevant documentation
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// New creates a new Error with the given code and message.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Wrap creates a new Error wrapping an underlying cause.
func Wrap(code Code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}

// WithFix returns a copy of the error with a fix suggestion.
func (e *Error) WithFix(fix string) *Error {
	e.Fix = fix
	return e
}

// WithDocs returns a copy of the error with a docs URL.
func (e *Error) WithDocs(url string) *Error {
	e.DocsURL = url
	return e
}
