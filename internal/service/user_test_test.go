package service

import (
	"context"
	"errors"
	"testing"

	"migrated-app/internal/apperr"
	"migrated-app/internal/model"
	"migrated-app/internal/repository"
)

// extRepo is an in-memory repository.UserRepository with extra call
// tracking, distinct from the fakeRepo declared in user_test.go.
type extRepo struct {
	users      map[int]*model.User
	byName     map[string]*model.User
	err        error
	nextID     int
	saveResult *model.User
	saveCalls  []*model.User
	deleted    []int
	findByName []string
	writes     int
}

var _ repository.UserRepository = (*extRepo)(nil)

func newExtRepo() *extRepo {
	return &extRepo{users: map[int]*model.User{}, byName: map[string]*model.User{}, nextID: 100}
}

func (f *extRepo) Save(_ context.Context, u *model.User) (*model.User, error) {
	f.writes++
	if f.err != nil {
		return nil, f.err
	}
	f.saveCalls = append(f.saveCalls, u)
	if f.saveResult != nil {
		return f.saveResult, nil
	}
	saved := *u
	if _, ok := f.users[u.ID]; u.ID == 0 || !ok {
		saved.ID = f.nextID
		f.nextID++
	}
	f.users[saved.ID] = &saved
	return &saved, nil
}

func (f *extRepo) FindByID(_ context.Context, id int) (*model.User, bool, error) {
	if f.err != nil {
		return nil, false, f.err
	}
	u, ok := f.users[id]
	return u, ok, nil
}

func (f *extRepo) FindAll(_ context.Context) ([]*model.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []*model.User
	for _, u := range f.users {
		out = append(out, u)
	}
	return out, nil
}

func (f *extRepo) DeleteByID(_ context.Context, id int) error {
	f.writes++
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

func (f *extRepo) FindByName(_ context.Context, name string) (*model.User, bool, error) {
	f.findByName = append(f.findByName, name)
	if f.err != nil {
		return nil, false, f.err
	}
	u, ok := f.byName[name]
	return u, ok, nil
}

func mustExtService(t *testing.T, repo repository.UserRepository) UserService {
	t.Helper()
	svc, err := NewUserService(repo)
	if err != nil {
		t.Fatalf("NewUserService: %v", err)
	}
	return svc
}

var errExtBoom = errors.New("boom")

func TestExtSaveUser(t *testing.T) {
	ctx := context.Background()

	t.Run("new user gets generated id", func(t *testing.T) {
		repo := newExtRepo()
		in := &model.User{Name: model.StringPtr("a")}
		got, err := mustExtService(t, repo).SaveUser(ctx, in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || got.ID != 100 || *got.Name != "a" {
			t.Fatalf("got %v, want id 100 name a", got)
		}
		if len(repo.saveCalls) != 1 || repo.saveCalls[0] != in {
			t.Fatal("expected one Save with given user")
		}
	})

	t.Run("existing id overwrites record", func(t *testing.T) {
		repo := newExtRepo()
		repo.users[5] = &model.User{ID: 5, Name: model.StringPtr("old")}
		got, err := mustExtService(t, repo).SaveUser(ctx, &model.User{ID: 5, Name: model.StringPtr("new")})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != 5 || *got.Name != "new" || *repo.users[5].Name != "new" {
			t.Fatalf("got %v, stored %v", got, repo.users[5])
		}
	})

	tests := []struct {
		name    string
		repoErr error
		user    *model.User
		wantErr error
		anyErr  bool
	}{
		{name: "returns exactly repository entity", user: &model.User{ID: 1}},
		{name: "repository error is wrapped", user: &model.User{ID: 2}, repoErr: errExtBoom, wantErr: errExtBoom},
		{name: "nil user rejected", user: nil, anyErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newExtRepo()
			repo.err = tc.repoErr
			sentinel := &model.User{ID: 999}
			repo.saveResult = sentinel
			got, err := mustExtService(t, repo).SaveUser(ctx, tc.user)
			switch {
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				if errors.Is(err, apperr.ErrUserNotFound) {
					t.Fatal("must not map to not-found")
				}
			case tc.anyErr:
				if err == nil {
					t.Fatal("expected error")
				}
				if len(repo.saveCalls) != 0 {
					t.Fatal("repository must not be called")
				}
			default:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != sentinel {
					t.Fatalf("got %p, want repository-returned %p", got, sentinel)
				}
			}
		})
	}
}

func TestExtFetchUserList(t *testing.T) {
	tests := []struct {
		name    string
		seed    []*model.User
		repoErr error
		wantLen int
	}{
		{name: "empty repository yields empty non-nil slice", wantLen: 0},
		{name: "returns all users", seed: []*model.User{{ID: 1}, {ID: 2}}, wantLen: 2},
		{name: "repository error", repoErr: errExtBoom},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newExtRepo()
			for _, u := range tc.seed {
				repo.users[u.ID] = u
			}
			repo.err = tc.repoErr
			got, err := mustExtService(t, repo).FetchUserList(context.Background())
			if repo.writes != 0 {
				t.Fatal("must not modify data")
			}
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
			seen := map[*model.User]bool{}
			for _, u := range got {
				seen[u] = true
			}
			for _, u := range tc.seed {
				if !seen[u] {
					t.Fatalf("missing user %v", u)
				}
			}
		})
	}
}

func TestExtFetchUserByID(t *testing.T) {
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
		{name: "repository error passes through wrapped", id: 7, repoErr: errExtBoom},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newExtRepo()
			repo.users[existing.ID] = existing
			repo.err = tc.repoErr
			got, err := mustExtService(t, repo).FetchUserByID(context.Background(), tc.id)
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
				var nf *apperr.UserNotFoundError
				if !errors.As(err, &nf) {
					t.Fatalf("err %T is not *UserNotFoundError", err)
				}
				if nf.Message() != "User are not available" {
					t.Fatalf("message = %q", nf.Message())
				}
				if got != nil {
					t.Fatal("expected nil user")
				}
			default:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.want || got.ID != tc.id {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestExtDeleteUser(t *testing.T) {
	tests := []struct {
		name    string
		id      int
		repoErr error
		wantErr error
	}{
		{name: "deletes existing", id: 3},
		{name: "missing id propagates ErrNoRowsDeleted", id: 42, wantErr: repository.ErrNoRowsDeleted},
		{name: "repository error", id: 3, repoErr: errExtBoom, wantErr: errExtBoom},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newExtRepo()
			repo.users[3] = &model.User{ID: 3}
			repo.err = tc.repoErr
			svc := mustExtService(t, repo)
			err := svc.DeleteUser(context.Background(), tc.id)
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
			if _, err := svc.FetchUserByID(context.Background(), tc.id); !errors.Is(err, apperr.ErrUserNotFound) {
				t.Fatalf("fetch after delete err = %v, want ErrUserNotFound", err)
			}
		})
	}
}

func TestExtUpdateUser(t *testing.T) {
	tests := []struct {
		name    string
		id      int
		seeded  bool
		user    *model.User
		repoErr error
		anyErr  bool
	}{
		{name: "existing id overwritten", id: 5, seeded: true, user: &model.User{Name: model.StringPtr("n")}},
		{name: "overrides body id", id: 5, seeded: true, user: &model.User{ID: 123}},
		{name: "missing id still saved", id: 6, user: &model.User{}},
		{name: "repository error", id: 5, user: &model.User{}, repoErr: errExtBoom, anyErr: true},
		{name: "nil user rejected", id: 5, user: nil, anyErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newExtRepo()
			if tc.seeded {
				repo.users[5] = &model.User{ID: 5, Email: model.StringPtr("e"), About: model.StringPtr("x")}
			}
			repo.err = tc.repoErr
			err := mustExtService(t, repo).UpdateUser(context.Background(), tc.id, tc.user)
			if tc.anyErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tc.repoErr != nil && !errors.Is(err, tc.repoErr) {
					t.Fatalf("err = %v, want wrapping %v", err, tc.repoErr)
				}
				if tc.user == nil && len(repo.saveCalls) != 0 {
					t.Fatal("repository must not be called for nil user")
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
			if tc.seeded {
				stored := repo.users[tc.id]
				if !stored.Equal(&model.User{ID: tc.id, Name: tc.user.Name}) {
					t.Fatalf("stored = %v, unset fields should be nil", stored)
				}
			}
		})
	}
}

func TestExtGetUserNameByName(t *testing.T) {
	hemraj := &model.User{
		ID:       3,
		Name:     model.StringPtr("hemraj"),
		Email:    model.StringPtr("hemrajmalhi1234@gmail.com"),
		About:    model.StringPtr("Sr"),
		Password: model.StringPtr("root"),
		Role:     model.StringPtr("java developer"),
	}
	tests := []struct {
		name    string
		lookup  string
		repoErr error
		want    *model.User
		wantOK  bool
	}{
		{name: "found", lookup: "hemraj", want: hemraj, wantOK: true},
		{name: "not found returns false without error", lookup: "bob"},
		{name: "non-unique error propagates", lookup: "hemraj", repoErr: repository.ErrNonUniqueResult},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newExtRepo()
			repo.byName["hemraj"] = hemraj
			repo.err = tc.repoErr
			got, ok, err := mustExtService(t, repo).GetUserNameByName(context.Background(), tc.lookup)
			if len(repo.findByName) != 1 || repo.findByName[0] != tc.lookup {
				t.Fatalf("FindByName calls = %v", repo.findByName)
			}
			if repo.writes != 0 {
				t.Fatal("must not modify data")
			}
			if tc.repoErr != nil {
				if !errors.Is(err, tc.repoErr) {
					t.Fatalf("err = %v, want %v", err, tc.repoErr)
				}
				if errors.Is(err, apperr.ErrUserNotFound) {
					t.Fatal("must not map to not-found")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok != tc.wantOK || got != tc.want {
				t.Fatalf("got (%v, %v), want (%v, %v)", got, ok, tc.want, tc.wantOK)
			}
			if got != nil && (got.Name == nil || *got.Name != tc.lookup) {
				t.Fatalf("name = %v, want %q", got.Name, tc.lookup)
			}
		})
	}
}