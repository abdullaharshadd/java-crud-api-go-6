package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"migrated-app/internal/apperr"
)

func TestWriteErrorNil(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	WriteError(rec, req, nil)
	if rec.Body.Len() != 0 {
		t.Fatalf("expected no body, got %q", rec.Body.String())
	}
}

func TestWriteErrorNotFound(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantMsg string
	}{
		{"typed", apperr.NewUserNotFoundMsg("User are not available"), "User are not available"},
		{"typed wrapped", fmt.Errorf("wrap: %w", apperr.NewUserNotFoundMsg("gone")), "gone"},
		{"sentinel joined", errors.Join(apperr.ErrUserNotFound), apperr.ErrUserNotFound.Error()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/get_user_data/7", nil)
			WriteError(rec, req, tc.err)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("code=%d", rec.Code)
			}
			var m map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
				t.Fatalf("body=%q: %v", rec.Body.String(), err)
			}
			if m["status"] != "NOT_FOUND" || m["message"] != tc.wantMsg {
				t.Fatalf("body=%v", m)
			}
		})
	}
}

func TestWriteErrorInternal(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/save_user_data", nil)
	WriteError(rec, req, errors.New("db down"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code=%d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type=%q", ct)
	}
	var e springError
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("body=%q", rec.Body.String())
	}
	if e.Status != 500 || e.Path != "/save_user_data" || e.Error != "Internal Server Error" || e.Timestamp == "" {
		t.Fatalf("err=%+v", e)
	}
}

func TestNotFoundRoute(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nope", nil)
	NotFoundRoute(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code=%d", rec.Code)
	}
	var e springError
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("body=%q", rec.Body.String())
	}
	if e.Status != 404 || e.Path != "/nope" || e.Error != "Not Found" {
		t.Fatalf("err=%+v", e)
	}
}

func TestWriteEmpty(t *testing.T) {
	for _, code := range []int{http.StatusBadRequest, http.StatusMethodNotAllowed} {
		rec := httptest.NewRecorder()
		writeEmpty(rec, code)
		if rec.Code != code || rec.Body.Len() != 0 {
			t.Fatalf("code=%d body=%q", rec.Code, rec.Body.String())
		}
	}
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, map[string]int{"a": 1})
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	var m map[string]int
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil || m["a"] != 1 {
		t.Fatalf("body=%q", rec.Body.String())
	}
}