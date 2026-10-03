// Package apperr defines the application's domain errors.
package apperr

import "errors"

// ErrUserNotFound is the sentinel signalling that a requested user does not
// exist. Use errors.Is(err, ErrUserNotFound) to detect it anywhere in a
// wrapped chain. Its text matches the message the source service used
// ("User are not available", grammar kept because it appears in the
// HTTP 404 response body).
var ErrUserNotFound = errors.New("User are not available")

// UserNotFoundError carries an optional detail message and an optional
// underlying cause for a "user not found" condition. It replaces the Java
// UserNotFoundException and all of its constructors. Every
// *UserNotFoundError matches ErrUserNotFound via errors.Is.
type UserNotFoundError struct {
	msg   string
	cause error
}

// NewUserNotFound returns a UserNotFoundError with no detail message and no
// cause (Java: new UserNotFoundException()).
func NewUserNotFound() *UserNotFoundError {
	return &UserNotFoundError{}
}

// NewUserNotFoundMsg returns a UserNotFoundError with the given detail
// message and no cause (Java: new UserNotFoundException(message)).
func NewUserNotFoundMsg(message string) *UserNotFoundError {
	return &UserNotFoundError{msg: message}
}

// NewUserNotFoundMsgCause returns a UserNotFoundError with the given detail
// message and cause (Java: new UserNotFoundException(message, cause)).
func NewUserNotFoundMsgCause(message string, cause error) *UserNotFoundError {
	return &UserNotFoundError{msg: message, cause: cause}
}

// NewUserNotFoundCause returns a UserNotFoundError wrapping cause; its
// message is derived from the cause, mirroring Java's
// Throwable(Throwable cause) behaviour (message = cause.toString(), or
// empty when cause is nil).
func NewUserNotFoundCause(cause error) *UserNotFoundError {
	e := &UserNotFoundError{cause: cause}
	if cause != nil {
		e.msg = cause.Error()
	}
	return e
}

// MIGRATION_NOTE: the protected Java constructor
// (message, cause, enableSuppression, writableStackTrace) has no Go
// equivalent: Go errors have neither suppressed exceptions nor captured stack
// traces. Callers needing it should use NewUserNotFoundMsgCause.

// Message returns the detail message exactly as supplied (Java getMessage()).
// It is empty when the error was constructed without one; HTTP handlers
// should use this for the ErrorMessage body.
func (e *UserNotFoundError) Message() string {
	return e.msg
}

// Error implements the error interface. When no detail message was supplied
// it falls back to the sentinel's text so logs are never blank.
func (e *UserNotFoundError) Error() string {
	if e.msg == "" {
		return ErrUserNotFound.Error()
	}
	return e.msg
}

// Unwrap exposes the underlying cause for errors.Is / errors.As.
func (e *UserNotFoundError) Unwrap() error {
	return e.cause
}

// Is reports whether target is ErrUserNotFound, so that every
// UserNotFoundError matches the sentinel regardless of message or cause.
func (e *UserNotFoundError) Is(target error) bool {
	return target == ErrUserNotFound
}
