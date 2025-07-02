package errors

import (
	"errors"
	"fmt"
)

// Common error types
var (
	// ErrNotFound indicates that a requested resource was not found
	ErrNotFound = errors.New("not found")

	// ErrAlreadyExists indicates that a resource already exists
	ErrAlreadyExists = errors.New("already exists")

	// ErrInvalidInput indicates that invalid input was provided
	ErrInvalidInput = errors.New("invalid input")

	// ErrTimeout indicates that an operation timed out
	ErrTimeout = errors.New("operation timed out")

	// ErrCancelled indicates that an operation was cancelled
	ErrCancelled = errors.New("operation cancelled")

	// ErrUnauthorized indicates that the request lacks valid authentication
	ErrUnauthorized = errors.New("unauthorized")

	// ErrForbidden indicates that the request is not allowed
	ErrForbidden = errors.New("forbidden")

	// ErrRateLimited indicates that the rate limit has been exceeded
	ErrRateLimited = errors.New("rate limit exceeded")

	// ErrExternalService indicates an error with an external service
	ErrExternalService = errors.New("external service error")

	// ErrDatabaseOperation indicates a database operation failure
	ErrDatabaseOperation = errors.New("database operation failed")

	// ErrNetworkOperation indicates a network operation failure
	ErrNetworkOperation = errors.New("network operation failed")

	// ErrDownloadFailed indicates that a download operation failed
	ErrDownloadFailed = errors.New("download failed")

	// ErrAlreadyInQueue indicates that an item is already in the download queue
	ErrAlreadyInQueue = errors.New("already in download queue")

	// ErrNoNZBFound indicates that no suitable NZB was found
	ErrNoNZBFound = errors.New("no suitable NZB found")
)

// ServiceError represents a service-level error with additional context
type ServiceError struct {
	Op      string // Operation that failed
	Service string // Service where the error occurred
	Err     error  // Underlying error
	Context map[string]interface{} // Additional context
}

// Error implements the error interface
func (e *ServiceError) Error() string {
	if e.Context != nil && len(e.Context) > 0 {
		return fmt.Sprintf("%s.%s: %v (context: %v)", e.Service, e.Op, e.Err, e.Context)
	}
	return fmt.Sprintf("%s.%s: %v", e.Service, e.Op, e.Err)
}

// Unwrap allows errors.Is and errors.As to work
func (e *ServiceError) Unwrap() error {
	return e.Err
}

// NewServiceError creates a new ServiceError
func NewServiceError(service, op string, err error) *ServiceError {
	return &ServiceError{
		Service: service,
		Op:      op,
		Err:     err,
	}
}

// WithContext adds context to a ServiceError
func (e *ServiceError) WithContext(key string, value interface{}) *ServiceError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// IsNotFound checks if an error is a not found error
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsAlreadyExists checks if an error is an already exists error
func IsAlreadyExists(err error) bool {
	return errors.Is(err, ErrAlreadyExists)
}

// IsTimeout checks if an error is a timeout error
func IsTimeout(err error) bool {
	return errors.Is(err, ErrTimeout)
}

// IsCancelled checks if an error is a cancellation error
func IsCancelled(err error) bool {
	return errors.Is(err, ErrCancelled)
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	return errors.Is(err, ErrTimeout) || 
		errors.Is(err, ErrNetworkOperation) || 
		errors.Is(err, ErrRateLimited) ||
		errors.Is(err, ErrExternalService)
}