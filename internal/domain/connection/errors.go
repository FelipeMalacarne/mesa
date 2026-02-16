package connection

import "errors"

var (
	// Connection Errors
	ErrConnectionFailed     = errors.New("failed to connect to the database")
	ErrAuthenticationFailed = errors.New("authentication failed")
	ErrHostUnreachable      = errors.New("host unreachable")
	ErrInvalidConfiguration = errors.New("invalid connection configuration")

	// Query/Execution Errors
	ErrQueryFailed      = errors.New("query execution failed")
	ErrPermissionDenied = errors.New("permission denied")
	ErrResourceNotFound = errors.New("resource not found")
	ErrTimeout          = errors.New("operation timed out")
)
