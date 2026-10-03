package repository

import (
	"testing"

	"migrated-app/internal/model"
)

func sp(s string) *string { return model.StringPtr(s) }

func TestStringPtr(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"value", "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := sp(tt.in)
			if p == nil || *p != tt.in {
				t.Fatalf("StringPtr(%q) = %v", tt.in, p)
			}
		})
	}
}

func TestNewUserWithAll(t *testing.T) {
	u := model.NewUserWithAll(0, sp("hemraj"), sp("hemrajmalhi1234@gmail.com"), sp("root"), sp("java developer"), sp("Sr"))
	if u == nil {
		t.Fatal("expected user")
	}
	if u.ID != 0 {
		t.Fatalf("id = %d", u.ID)
	}
	if u.Name == nil || *u.Name != "hemraj" {
		t.Fatalf("name = %v", u.Name)
	}
	if u.Email == nil || *u.Email != "hemrajmalhi1234@gmail.com" {
		t.Fatalf("email = %v", u.Email)
	}
	if u.Password == nil || *u.Password != "root" {
		t.Fatalf("password = %v", u.Password)
	}
	if u.About == nil || *u.About != "java developer" {
		t.Fatalf("about = %v", u.About)
	}
	if u.Role == nil || *u.Role != "Sr" {
		t.Fatalf("role = %v", u.Role)
	}

	n := model.NewUserWithAll(0, sp("n"), nil, nil, nil, nil)
	if n.Email != nil || n.Password != nil || n.Role != nil || n.About != nil {
		t.Fatalf("expected nil fields, got %v", n)
	}
}