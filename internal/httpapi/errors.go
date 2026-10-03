// Package httpapi contains the HTTP transport layer: handlers, routing and
// the translation of domain errors into HTTP responses.
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"migrated-app/internal/apperr"
	"migrated-app/internal/model"
)

// springError is the body Spring Boot's default error controller produces
// (BasicErrorController) for errors that no @ExceptionHandler claims.
type springError struct {
	Timestamp string `json:"timestamp"`
	Status    int    `json:"status"`
	Error     string `json:"error"`
	Path      string `json:"path"`
}

// WriteError translates an error returned by the service layer into an HTTP
// response. It replaces the source's @ControllerAdvice
// RestResponseEntityExceptionHandling:
//
//   - any user-not-found error (an *apperr.UserNotFoundError or anything
//     matching apperr.ErrUserNotFound via errors.Is) becomes HTTP 404 with an
//     ErrorMessage JSON body {"status":"NOT_FOUND","message":<message>};
//   - every other error becomes Spring's default 500 error JSON and is logged.
//
// Handlers must call WriteError explicitly; Go has no AOP-style interception.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}

	var nf *apperr.UserNotFoundError
	if errors.As(err, &nf) {
		// Java: exception.getMessage().
		// MIGRATION_NOTE: when the exception had no message Java serialized
		// "message":null; Go emits "" because ErrorMessage.Message is a string.
		writeNotFound(w, nf.Message())
		return
	}
	if errors.Is(err, apperr.ErrUserNotFound) {
		writeNotFound(w, apperr.ErrUserNotFound.Error())
		return
	}

	writeSpringError(w, r, http.StatusInternalServerError, err)
}

// NotFoundRoute writes Spring's default 404 error JSON for requests that
// match no registered route. Use it as the fallback handler of the router.
func NotFoundRoute(w http.ResponseWriter, r *http.Request) {
	writeSpringError(w, r, http.StatusNotFound, nil)
}

// writeNotFound writes HTTP 404 with an ErrorMessage body, exactly as the
// source userNotFoundException handler did.
func writeNotFound(w http.ResponseWriter, message string) {
	body := model.NewErrorMessageWithAll(model.HTTPStatusNotFound, message)
	writeJSON(w, http.StatusNotFound, body)
}

// writeEmpty writes a status code with an empty body. Spring's inherited
// ResponseEntityExceptionHandler answers validation failures, unreadable JSON
// and type mismatches with 400, and unsupported methods with 405, all with no
// body.
func writeEmpty(w http.ResponseWriter, status int) {
	w.WriteHeader(status)
}

// writeSpringError writes Spring Boot's default error JSON for the given
// status. A non-nil cause is logged (never exposed to the client).
func writeSpringError(w http.ResponseWriter, r *http.Request, status int, cause error) {
	if cause != nil {
		slog.Error("request failed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"error", cause,
		)
	}
	body := springError{
		Timestamp: time.Now().Format("2006-01-02T15:04:05.000-07:00"),
		Status:    status,
		Error:     http.StatusText(status),
		Path:      r.URL.Path,
	}
	writeJSON(w, status, body)
}

// writeJSON serializes v as the JSON response body with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Headers are already sent; all we can do is log.
		slog.Error("failed to encode JSON response", "error", err)
	}
}
