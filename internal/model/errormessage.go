package model

import "fmt"

// HTTPStatusNotFound is the Spring HttpStatus enum name for HTTP 404, as
// Jackson serializes it into the "status" field of an ErrorMessage.
const HTTPStatusNotFound = "NOT_FOUND"

// ErrorMessage is the JSON error payload returned to clients. It mirrors the
// source DTO: Status holds the Spring HttpStatus enum *name* (for example
// "NOT_FOUND"), which is how Jackson serializes the enum, and Message holds a
// human-readable description.
type ErrorMessage struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// NewErrorMessage returns an empty ErrorMessage with all fields unset.
func NewErrorMessage() *ErrorMessage {
	return &ErrorMessage{}
}

// NewErrorMessageWithAll returns an ErrorMessage with the given status
// (a Spring HttpStatus enum name such as "NOT_FOUND") and message, in
// declaration order.
func NewErrorMessageWithAll(status, message string) *ErrorMessage {
	return &ErrorMessage{Status: status, Message: message}
}

// GetStatus returns the status field.
func (e *ErrorMessage) GetStatus() string { return e.Status }

// SetStatus sets the status field.
func (e *ErrorMessage) SetStatus(status string) { e.Status = status }

// GetMessage returns the message field.
func (e *ErrorMessage) GetMessage() string { return e.Message }

// SetMessage sets the message field.
func (e *ErrorMessage) SetMessage(message string) { e.Message = message }

// Equal reports whether e and other have the same status and message.
// Two nil pointers are equal; a nil and a non-nil pointer are not.
func (e *ErrorMessage) Equal(other *ErrorMessage) bool {
	if e == nil || other == nil {
		return e == other
	}
	return e.Status == other.Status && e.Message == other.Message
}

// String returns a Lombok-style representation including both fields,
// e.g. "ErrorMessage(status=NOT_FOUND, message=User are not available)".
func (e *ErrorMessage) String() string {
	if e == nil {
		return "ErrorMessage(nil)"
	}
	return fmt.Sprintf("ErrorMessage(status=%s, message=%s)", e.Status, e.Message)
}
