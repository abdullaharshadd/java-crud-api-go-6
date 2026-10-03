package service

import (
	"context"
	"errors"
	"testing"

	"migrated-app/internal/apperr"
	"migrated-app/internal/model"
	"migrated-app/internal/repository"
)

// fakeRepo is an in-memory repository.UserRepository used to exercise the
// service without a database.
type fakeRepo struct {
	users     map[int]*model.User
	byName    map[string]*model.User
	err       error
	saveCalls []*model.User
	deleted   []int
}

var _ repository.UserRepository = (*fakeRepo)(nil)

func newFakeRepo() *fakeRepo {
	return &fakeRepo{users: map[int]*model.User{}, byName: map[string]*model.User{}}
}

func (f *fakeRepo) Save(_ context.Context, u *model.User) (*model.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.saveCalls = append(f.saveCalls, u)
	f.users[u.ID] = u
	return u, nil
}

func (f *fakeRepo) FindByID(_ context.Context, id int) (*model.User, bool, error) {
	if f.err != nil {
		return nil, false, f.err
	}
	u, ok := f.users[id]
	return u, ok, nil
}

func (f *fakeRepo) FindAll(_ context.Context) ([]*model.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []*model.User
	for _, u := range f.users {
		out = append(out, u)
	}
	return out, nil
}

func (f *fakeRepo) DeleteByID(_ context.Context, id int) error {
	if f.err != nil {
		return f.err
	}
	if _, ok := f.users[id]; !ok {
		return repository.ErrNoRowsDeleted
	}
	delete(f.users, id)
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeRepo) FindByName(_ context.Context, name string) (*model.User, bool, error) {
	if f.err != nil {
		return nil, false, f.err
	}
	u, ok := f.byName[name]
	return u, ok, nil
}

func mustService(t *testing.T, repo repository.UserRepository) UserService {
	t.Helper()
	svc, err := NewUserService(repo)
	if err != nil {
		t.Fatalf("NewUserService: %v", err)
	}
	return svc
}

var errBoom = errors.New("boom")

func TestNewUserService_NilRepo(t *testing.T) {
	if _, err := NewUserService(nil); err == nil {
		t.Fatal("expected error for nil repository")
	}
}

func TestSaveUser(t *testing.T) {
	tests := []struct {
		name    string
		repoErr error
		user    *model.User
		wantErr error
		anyErr  bool
	}{
		{name: "saves and returns repository entity", user: &model.User{ID: 1}},
		{name: "repository error is wrapped", user: &model.User{ID: 2}, repoErr: errBoom, wantErr: errBoom},
		{name: "nil user rejected", user: nil, anyErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			repo.err = tc.repoErr
			got, err := mustService(t, repo).SaveUser(context.Background(), tc.user)
			switch {
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
			case tc.anyErr:
				if err == nil {
					t.Fatal("expected error")
				}
			default:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.user {
					t.Fatalf("got %p, want repository-returned %p", got, tc.user)
				}
			}
		})
	}
}

func TestFetchUserList(t *testing.T) {
	tests := []struct {
		name    string
		seed    []*model.User
		repoErr error
		wantLen int
	}{
		{name: "empty repository yields empty non-nil slice", wantLen: 0},
		{name: "returns all users", seed: []*model.User{{ID: 1}, {ID: 2}}, wantLen: 2},
		{name: "repository error", repoErr: errBoom},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			for _, u := range tc.seed {
				repo.users[u.ID] = u
			}
			repo.err = tc.repoErr
			got, err := mustService(t, repo).FetchUserList(context.Background())
			if tc.repoErr != nil {
				if !errors.Is(err, tc.repoErr) {
					t.Fatalf("err = %v, want %v", err, tc.repoErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("slice must never be nil")
			}
			if len(got) != tc.wantLen {
				t.Fatalf("len = %d, want %d", len(got), tc.wantLen)
			}
		})
	}
}

func TestFetchUserByID(t *testing.T) {
	existing := &model.User{ID: 7}
	tests := []struct {
		name     string
		id       int
		repoErr  error
		want     *model.User
		notFound bool
	}{
		{name: "found", id: 7, want: existing},
		{name: "missing maps to UserNotFound", id: 99, notFound: true},
		{name: "repository error passes through wrapped", id: 7, repoErr: errBoom},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			repo.users[existing.ID] = existing
			repo.err = tc.repoErr
			got, err := mustService(t, repo).FetchUserByID(context.Background(), tc.id)
			switch {
			case tc.repoErr != nil:
				if !errors.Is(err, tc.repoErr) {
					t.Fatalf("err = %v, want %v", err, tc.repoErr)
				}
				if errors.Is(err, apperr.ErrUserNotFound) {
					t.Fatal("repository error must not map to not-found")
				}
			case tc.notFound:
				if !errors.Is(err, apperr.ErrUserNotFound) {
					t.Fatalf("err = %v, want ErrUserNotFound", err)
				}
			default:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.want {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestDeleteUser(t *testing.T) {
	tests := []struct {
		name    string
		id      int
		repoErr error
		wantErr error
	}{
		{name: "deletes existing", id: 3},
		{name: "missing id propagates ErrNoRowsDeleted", id: 42, wantErr: repository.ErrNoRowsDeleted},
		{name: "repository error", id: 3, repoErr: errBoom, wantErr: errBoom},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			repo.users[3] = &model.User{ID: 3}
			repo.err = tc.repoErr
			err := mustService(t, repo).DeleteUser(context.Background(), tc.id)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				if errors.Is(err, apperr.ErrUserNotFound) {
					t.Fatal("delete must not map to not-found")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if _, ok := repo.users[tc.id]; ok {
				t.Fatal("user was not deleted")
			}
		})
	}
}

func TestUpdateUser(t *testing.T) {
	tests := []struct {
		name    string
		id      int
		user    *model.User
		repoErr error
		anyErr  bool
	}{
		{name: "sets id and saves", id: 5, user: &model.User{ID: 0}},
		{name: "overrides body id", id: 6, user: &model.User{ID: 123}},
		{name: "repository error", id: 5, user: &model.User{}, repoErr: errBoom, anyErr: true},
		{name: "nil user rejected", id: 5, user: nil, anyErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			repo.err = tc.repoErr
			err := mustService(t, repo).UpdateUser(context.Background(), tc.id, tc.user)
			if tc.anyErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tc.repoErr != nil && !errors.Is(err, tc.repoErr) {
					t.Fatalf("err = %v, want wrapping %v", err, tc.repoErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.user.ID != tc.id {
				t.Fatalf("user.ID = %d, want %d", tc.user.ID, tc.id)
			}
			if len(repo.saveCalls) != 1 || repo.saveCalls[0] != tc.user {
				t.Fatalf("Save calls = %v, want exactly the given user", repo.saveCalls)
			}
		})
	}
}

func TestGetUserNameByName(t *testing.T) {
	alice := &model.User{ID: 1}
	tests := []struct {
		name    string
		lookup  string
		repoErr error
		want    *model.User
		wantOK  bool
	}{
		{name: "found", lookup: "alice", want: alice, wantOK: true},
		{name: "not found returns false without error", lookup: "bob"},
		{name: "non-unique error propagates", lookup: "alice", repoErr: repository.ErrNonUniqueResult},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			repo.byName["alice"] = alice
			repo.err = tc.repoErr
			got, ok, err := mustService(t, repo).GetUserNameByName(context.Background(), tc.lookup)
			if tc.repoErr != nil {
				if !errors.Is(err, tc.repoErr) {
					t.Fatalf("err = %v, want %v", err, tc.repoErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok != tc.wantOK || got != tc.want {
				t.Fatalf("got (%v, %v), want (%v, %v)", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
