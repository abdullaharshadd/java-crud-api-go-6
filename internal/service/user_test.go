package service_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"

	"migrated-app/internal/apperr"
	"migrated-app/internal/model"
	"migrated-app/internal/repository"
	"migrated-app/internal/service"
)

// userSvc is the local view of the service contract exercised by these tests.
type userSvc interface {
	SaveUser(ctx context.Context, u *model.User) (*model.User, error)
	FetchUserList(ctx context.Context) ([]*model.User, error)
	FetchUserByID(ctx context.Context, id int) (*model.User, error)
	DeleteUser(ctx context.Context, id int) error
	UpdateUser(ctx context.Context, id int, u *model.User) error
	GetUserNameByName(ctx context.Context, name string) (*model.User, bool, error)
}

// fakeRepo is an in-memory repository.UserRepository mimicking the MySQL semantics.
type fakeRepo struct {
	users  map[int]*model.User
	nextID int

	saveErr, findAllErr, findByIDErr, deleteErr, findByNameErr error
	nilFindAll                                                 bool

	saveCalls, deleteCalls int
}

func newFakeRepo() *fakeRepo { return &fakeRepo{users: map[int]*model.User{}, nextID: 1} }

func cp(u *model.User) *model.User { c := *u; return &c }

func (f *fakeRepo) Save(ctx context.Context, u *model.User) (*model.User, error) {
	f.saveCalls++
	if f.saveErr != nil {
		return nil, f.saveErr
	}
	if u == nil {
		return nil, errors.New("nil")
	}
	saved := cp(u)
	if _, ok := f.users[u.ID]; u.ID == 0 || !ok {
		saved.ID = f.nextID
		f.nextID++
	}
	f.users[saved.ID] = cp(saved)
	return saved, nil
}

func (f *fakeRepo) FindByID(ctx context.Context, id int) (*model.User, bool, error) {
	if f.findByIDErr != nil {
		return nil, false, f.findByIDErr
	}
	u, ok := f.users[id]
	if !ok {
		return nil, false, nil
	}
	return cp(u), true, nil
}

func (f *fakeRepo) FindAll(ctx context.Context) ([]*model.User, error) {
	if f.findAllErr != nil {
		return nil, f.findAllErr
	}
	if f.nilFindAll {
		return nil, nil
	}
	ids := make([]int, 0, len(f.users))
	for id := range f.users {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	out := make([]*model.User, 0, len(ids))
	for _, id := range ids {
		out = append(out, cp(f.users[id]))
	}
	return out, nil
}

func (f *fakeRepo) DeleteByID(ctx context.Context, id int) error {
	f.deleteCalls++
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.users[id]; !ok {
		return fmt.Errorf("delete user %d: %w", id, repository.ErrNoRowsDeleted)
	}
	delete(f.users, id)
	return nil
}

func (f *fakeRepo) FindByName(ctx context.Context, name string) (*model.User, bool, error) {
	if f.findByNameErr != nil {
		return nil, false, f.findByNameErr
	}
	var found []*model.User
	for _, u := range f.users {
		if u.Name != nil && *u.Name == name {
			found = append(found, u)
		}
	}
	switch len(found) {
	case 0:
		return nil, false, nil
	case 1:
		return cp(found[0]), true, nil
	default:
		return nil, false, fmt.Errorf("find user by name %q: %w", name, repository.ErrNonUniqueResult)
	}
}

func sp(s string) *string { return model.StringPtr(s) }

func mkUser(id int, name, email string) *model.User {
	return model.NewUserWithAll(id, sp(name), sp(email), sp("pw"), sp("admin"), sp("about"))
}

func newSvc(t *testing.T, r *fakeRepo) userSvc {
	t.Helper()
	s, err := service.NewUserService(r)
	if err != nil {
		t.Fatalf("NewUserService: %v", err)
	}
	return s
}

var errDB = errors.New("db failure")

func TestNewUserService(t *testing.T) {
	tests := []struct {
		name    string
		repo    repository.UserRepository
		wantErr bool
	}{
		{"nil repo", nil, true},
		{"valid repo", newFakeRepo(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := service.NewUserService(tt.repo)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr && s != nil {
				t.Fatal("expected nil service")
			}
			if !tt.wantErr && s == nil {
				t.Fatal("expected service")
			}
		})
	}
}

func TestSaveUser(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		seed      []*model.User
		saveErr   error
		input     *model.User
		wantErr   bool
		wantErrIs error
		wantID    int
		wantCalls int
		wantCount int
	}{
		{"new user gets generated id", nil, nil, mkUser(0, "alice", "a@x"), false, nil, 1, 1, 1},
		{"existing id overwrites", []*model.User{mkUser(0, "old", "o@x")}, nil, mkUser(1, "new", "n@x"), false, nil, 1, 2, 1},
		{"nil user rejected without write", nil, nil, nil, true, nil, 0, 0, 0},
		{"constraint violation propagated", nil, errDB, mkUser(0, "a", "dup@x"), true, errDB, 0, 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newFakeRepo()
			for _, u := range tt.seed {
				if _, err := r.Save(ctx, u); err != nil {
					t.Fatal(err)
				}
			}
			r.saveErr = tt.saveErr
			s := newSvc(t, r)
			got, err := s.SaveUser(ctx, tt.input)
			if r.saveCalls != tt.wantCalls {
				t.Errorf("save calls=%d want %d", r.saveCalls, tt.wantCalls)
			}
			if len(r.users) != tt.wantCount {
				t.Errorf("store size=%d want %d", len(r.users), tt.wantCount)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if got != nil {
					t.Error("expected nil user")
				}
				if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
					t.Errorf("err %v not wrapping %v", err, tt.wantErrIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got.ID == 0 || got.ID != tt.wantID {
				t.Errorf("id=%d want %d", got.ID, tt.wantID)
			}
			want := cp(tt.input)
			want.ID = got.ID
			if !got.Equal(want) {
				t.Errorf("got %v want %v", got, want)
			}
			fetched, err := s.FetchUserByID(ctx, got.ID)
			if err != nil || !fetched.Equal(got) {
				t.Errorf("fetch after save: %v, %v", fetched, err)
			}
		})
	}
}

func TestFetchUserList(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		seed      int
		nilResult bool
		repoErr   error
		wantLen   int
		wantErr   bool
	}{
		{"users exist", 3, false, nil, 3, false},
		{"no users", 0, false, nil, 0, false},
		{"repo returns nil slice", 0, true, nil, 0, false},
		{"repo error", 0, false, errDB, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newFakeRepo()
			for i := 0; i < tt.seed; i++ {
				r.Save(ctx, mkUser(0, fmt.Sprintf("u%d", i), fmt.Sprintf("u%d@x", i)))
			}
			r.nilFindAll, r.findAllErr = tt.nilResult, tt.repoErr
			before := r.saveCalls
			got, err := newSvc(t, r).FetchUserList(ctx)
			if r.saveCalls != before || r.deleteCalls != 0 {
				t.Error("read must not write")
			}
			if tt.wantErr {
				if !errors.Is(err, errDB) || got != nil {
					t.Fatalf("got %v, err %v", got, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got == nil {
				t.Fatal("list must not be nil")
			}
			if len(got) != tt.wantLen {
				t.Errorf("len=%d want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestFetchUserByID(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name         string
		id           int
		repoErr      error
		wantNotFound bool
		wantErrIs    error
	}{
		{"existing", 1, nil, false, nil},
		{"missing", 99, nil, true, nil},
		{"repo error", 1, errDB, false, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newFakeRepo()
			r.Save(ctx, mkUser(0, "alice", "a@x"))
			r.findByIDErr = tt.repoErr
			before := r.saveCalls
			got, err := newSvc(t, r).FetchUserByID(ctx, tt.id)
			if r.saveCalls != before {
				t.Error("read must not write")
			}
			switch {
			case tt.wantNotFound:
				if got != nil || !errors.Is(err, apperr.ErrUserNotFound) {
					t.Fatalf("got %v err %v", got, err)
				}
				var unf *apperr.UserNotFoundError
				if !errors.As(err, &unf) || unf.Message() != "User are not available" {
					t.Errorf("unexpected not-found error %v", err)
				}
			case tt.wantErrIs != nil:
				if got != nil || !errors.Is(err, tt.wantErrIs) {
					t.Fatalf("got %v err %v", got, err)
				}
				if errors.Is(err, apperr.ErrUserNotFound) {
					t.Error("repo error must not be not-found")
				}
			default:
				if err != nil || got == nil || got.ID != tt.id {
					t.Fatalf("got %v err %v", got, err)
				}
			}
		})
	}
}

func TestDeleteUser(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		id        int
		repoErr   error
		wantErrIs error
		wantLeft  int
	}{
		{"existing", 1, nil, nil, 1},
		{"missing", 99, nil, repository.ErrNoRowsDeleted, 2},
		{"repo error", 1, errDB, errDB, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newFakeRepo()
			r.Save(ctx, mkUser(0, "a", "a@x"))
			r.Save(ctx, mkUser(0, "b", "b@x"))
			r.deleteErr = tt.repoErr
			s := newSvc(t, r)
			err := s.DeleteUser(ctx, tt.id)
			if tt.wantErrIs != nil {
				if !errors.Is(err, tt.wantErrIs) {
					t.Fatalf("err %v want %v", err, tt.wantErrIs)
				}
				if errors.Is(err, apperr.ErrUserNotFound) {
					t.Error("delete must not return UserNotFound")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if _, err := s.FetchUserByID(ctx, tt.id); !errors.Is(err, apperr.ErrUserNotFound) {
					t.Errorf("expected not found after delete, got %v", err)
				}
				if _, ok := r.users[2]; !ok {
					t.Error("other user removed")
				}
			}
			if len(r.users) != tt.wantLeft {
				t.Errorf("left=%d want %d", len(r.users), tt.wantLeft)
			}
		})
	}
}

func TestUpdateUser(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		id       int
		input    *model.User
		repoErr  error
		wantErr  bool
		wantSize int
	}{
		{"existing id", 1, mkUser(0, "updated", "u@x"), nil, false, 2},
		// MIGRATION_NOTE: source upsert semantics — missing id inserts a new row.
		{"missing id upserts", 50, mkUser(0, "ghost", "g@x"), nil, false, 3},
		{"nil user", 1, nil, nil, true, 2},
		{"repo error", 1, mkUser(0, "x", "x@x"), errDB, true, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newFakeRepo()
			r.Save(ctx, mkUser(0, "a", "a@x"))
			r.Save(ctx, mkUser(0, "b", "b@x"))
			r.saveErr = tt.repoErr
			other := cp(r.users[2])
			original := cp(r.users[1])
			err := newSvc(t, r).UpdateUser(ctx, tt.id, tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v", err)
			}
			if tt.repoErr != nil && !errors.Is(err, tt.repoErr) {
				t.Errorf("err %v not wrapping repo err", err)
			}
			if tt.input != nil && tt.input.ID != tt.id {
				t.Errorf("input ID=%d want %d", tt.input.ID, tt.id)
			}
			if !r.users[2].Equal(other) {
				t.Error("other user modified")
			}
			if len(r.users) != tt.wantSize {
				t.Errorf("size=%d want %d", len(r.users), tt.wantSize)
			}
			if tt.wantErr || tt.id != 1 {
				if !r.users[1].Equal(original) {
					t.Error("target unexpectedly modified")
				}
				return
			}
			got := r.users[1]
			if got.ID != 1 || *got.Name != "updated" || *got.Email != "u@x" {
				t.Errorf("not updated: %v", got)
			}
		})
	}
}

func TestGetUserNameByName(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		query     string
		repoErr   error
		wantOK    bool
		wantErrIs error
	}{
		{"match", "alice", nil, true, nil},
		{"no match", "nobody", nil, false, nil},
		{"multiple", "dup", nil, false, repository.ErrNonUniqueResult},
		{"repo error", "alice", errDB, false, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newFakeRepo()
			r.Save(ctx, mkUser(0, "alice", "a@x"))
			r.Save(ctx, mkUser(0, "dup", "d1@x"))
			r.Save(ctx, mkUser(0, "dup", "d2@x"))
			r.findByNameErr = tt.repoErr
			before := r.saveCalls
			got, ok, err := newSvc(t, r).GetUserNameByName(ctx, tt.query)
			if r.saveCalls != before || r.deleteCalls != 0 {
				t.Error("read must not write")
			}
			if tt.wantErrIs != nil {
				if !errors.Is(err, tt.wantErrIs) || ok || got != nil {
					t.Fatalf("got %v %v %v", got, ok, err)
				}
				return
			}
			if err != nil || ok != tt.wantOK {
				t.Fatalf("ok=%v err=%v", ok, err)
			}
			if !ok {
				if got != nil {
					t.Error("expected nil user")
				}
				return
			}
			if got == nil || got.Name == nil || *got.Name != tt.query {
				t.Errorf("got %v", got)
			}
		})
	}
}