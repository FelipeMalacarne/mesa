// Package errs defines the error kinds the application reports. Transports map
// kinds to their own status codes; drivers wrap their failures in a kind so
// nothing engine-specific leaks to a client.
package errs

import (
	"errors"
	"fmt"
)

// Kind classifies an error for callers that must react to it.
type Kind int

const (
	Unknown Kind = iota
	NotFound
	Invalid
	Forbidden
	Conflict
	Unavailable
	Unsupported
	NeedsConfirmation
	// Unauthenticated means the request carries no valid sign-in.
	Unauthenticated
)

func (k Kind) String() string {
	switch k {
	case NotFound:
		return "not_found"
	case Invalid:
		return "invalid"
	case Forbidden:
		return "forbidden"
	case Conflict:
		return "conflict"
	case Unavailable:
		return "unavailable"
	case Unsupported:
		return "unsupported"
	case NeedsConfirmation:
		return "needs_confirmation"
	case Unauthenticated:
		return "unauthenticated"
	default:
		return "unknown"
	}
}

// Error is an error with a kind and a message safe to show to a person.
type Error struct {
	Kind    Kind
	Message string
	Cause   error
	// Hint carries extra text a client needs to act: for NeedsConfirmation,
	// the text the person must type.
	Hint string
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Cause }

// New returns an error of the given kind.
func New(kind Kind, msg string) error { return &Error{Kind: kind, Message: msg} }

// Newf formats a message for an error of the given kind.
func Newf(kind Kind, format string, args ...any) error {
	return &Error{Kind: kind, Message: fmt.Sprintf(format, args...)}
}

// Wrap attaches a kind and message to an underlying cause.
func Wrap(kind Kind, msg string, cause error) error {
	if cause == nil {
		return nil
	}
	return &Error{Kind: kind, Message: msg, Cause: cause}
}

// KindOf reports the kind of err, or Unknown for errors without one.
func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return Unknown
}

// Is reports whether err carries the given kind.
func Is(err error, kind Kind) bool { return KindOf(err) == kind }

// Hint returns the hint attached to err, if any.
func Hint(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Hint
	}
	return ""
}

// Message returns the person-facing message of err.
func Message(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Message
	}
	return err.Error()
}
