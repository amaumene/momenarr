package trakt

import (
	"encoding/json"
	"net/http"
)

type ErrorCode string

const (
	ErrorCodeUnknownError ErrorCode = "unknown_error"

	ErrorCodeInvalidRequest     ErrorCode = "invalid_request"
	ErrorCodeForbidden          ErrorCode = "forbidden"
	ErrorCodeUnauthorized       ErrorCode = "unauthorized"
	ErrorCodeNotFound           ErrorCode = "not_found"
	ErrorCodeInvalidOperation   ErrorCode = "invalid_operation"
	ErrorCodeConflict           ErrorCode = "resource_conflict"
	ErrorCodeInvalidContentType ErrorCode = "invalid_content_type"
	ErrorCodeValidationError    ErrorCode = "validation_error"
	ErrorCodeRateLimitExceeded  ErrorCode = "rate_limit_exceeded"
	ErrorCodeServerError        ErrorCode = "server_error"
	ErrorCodeServerUnavailable  ErrorCode = "server_unavailable"

	// Error codes specific to polling for a device code.
	ErrorCodePendingDeviceCode ErrorCode = "pending_device_code"
	ErrorCodeInvalidDeviceCode ErrorCode = "invalid_device_code"
	ErrorCodeDeviceCodeUsed    ErrorCode = "device_code_used"
	ErrorCodeDeviceCodeExpired ErrorCode = "device_code_expired"
	ErrorCodeDeviceCodeDenied  ErrorCode = "device_code_denied"

	// Error codes specific to posting a comment.
	ErrorCodePostInvalidUser        ErrorCode = "invalid_or_banned_user"
	ErrorCodePostInvalidItem        ErrorCode = "invalid_item_or_comments_disabled"
	ErrorCodeCommentCannotBeRemoved ErrorCode = "comment_cannot_be_removed"

	// Error codes specific to performing a checkin
	ErrorCodeCheckinInProgress ErrorCode = "checkin_in_progress"

	// Error codes for miscellaneous errors from within the SDK.
	ErrorCodeEmptyFrameData ErrorCode = "empty_frame_data"
	ErrorCodeEncodingError  ErrorCode = "encoding_error"
)

// DefaultErrorHandler the default error handler which is used to determine
// the error code from a HTTP status code.
var DefaultErrorHandler ErrorHandler = &defaultErrorHandler{}

type ErrorHandler interface {
	Code(statusCode int) ErrorCode
}

type Error struct {
	// HTTPStatusCode the status code of the request.
	HTTPStatusCode int `json:"status,omitempty"`
	// RequestID the uuid of the request.
	RequestID string `json:"request_id,omitempty"`
	// Resource the path of the resource attempted to access.
	Resource string `json:"resource,omitempty"`
	// Body the request body.
	Body string `json:"body"`
	// Code the error code attached to the error.
	Code ErrorCode `json:"code"`
}

// Error serializes the error object to JSON and returns it as a string.
func (e *Error) Error() string {
	ret, _ := json.Marshal(e)
	return string(ret)
}

type defaultErrorHandler struct{}

// Code default error handler which maps HTTP status codes
// to an error code which gives a little bit more context to an error.
func (defaultErrorHandler) Code(statusCode int) ErrorCode {
	switch statusCode {
	case http.StatusBadRequest:
		return ErrorCodeInvalidRequest
	case http.StatusUnauthorized:
		return ErrorCodeUnauthorized
	case http.StatusForbidden:
		return ErrorCodeForbidden
	case http.StatusNotFound:
		return ErrorCodeNotFound
	case http.StatusMethodNotAllowed:
		return ErrorCodeInvalidOperation
	case http.StatusConflict:
		return ErrorCodeConflict
	case http.StatusPreconditionFailed:
		return ErrorCodeInvalidContentType
	case http.StatusUnprocessableEntity:
		return ErrorCodeValidationError
	case http.StatusTooManyRequests:
		return ErrorCodeRateLimitExceeded
	case http.StatusInternalServerError:
		return ErrorCodeServerError
	// includes cloudflare errors.
	case http.StatusServiceUnavailable, http.StatusGatewayTimeout, 520, 521, 522:
		return ErrorCodeServerUnavailable
	}

	return ErrorCodeUnknownError
}
