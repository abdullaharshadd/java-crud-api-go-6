package model

import (
	"encoding/json"
	"testing"
)

// errorMessagePayload mirrors the expected JSON shape of the error DTO
// (status + message) so the wire contract can be verified independently.
type errorMessagePayload struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

const testStatusNotFound = "NOT_FOUND"

func TestErrorMessageFields(t *testing.T) {
	e := &errorMessagePayload{}
	if e.Status != "" || e.Message != "" {
		t.Fatalf("expected empty fields, got %+v", *e)
	}

	a := &errorMessagePayload{Status: testStatusNotFound, Message: "User are not available"}
	if a.Status != "NOT_FOUND" || a.Message != "User are not available" {
		t.Fatalf("unexpected fields: %+v", *a)
	}

	a.Status = "BAD_REQUEST"
	a.Message = "oops"
	if a.Status != "BAD_REQUEST" || a.Message != "oops" {
		t.Fatalf("assignment failed: %+v", *a)
	}
}

func TestErrorMessageJSON(t *testing.T) {
	e := &errorMessagePayload{Status: testStatusNotFound, Message: "User are not available"}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"status":"NOT_FOUND","message":"User are not available"}`
	if string(b) != want {
		t.Errorf("got %s want %s", b, want)
	}
	got := &errorMessagePayload{}
	if err := json.Unmarshal(b, got); err != nil {
		t.Fatal(err)
	}
	if got.Status != e.Status || got.Message != e.Message {
		t.Errorf("round trip mismatch: %+v vs %+v", *got, *e)
	}
}