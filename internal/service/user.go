// Package service contains the application's business logic layer.
package service

import (
	"context"
	"errors"
	"fmt"

	"migrated-app/internal/apperr"
	"migrated-app/internal/model"
	"migrated-app/internal/repository"
)

// userNotAvailableMsg is the detail message the source service used when a
// user lookup by id found nothing (grammar preserved: it is client-visible).
const userNotAvailableMsg = "User are not available"

// UserService is the business-logic contract for User management. It merges
// the Java UserService interface and its UserServiceImp implementation.
type UserService interface {
	// SaveUser persists u (insert, or overwrite when u.ID refers to an
	// existing row) and returns the persisted instance.
	SaveUser(ctx context.Context, u *model.User) (*model.User, error)
	// FetchUserList returns every persisted user. The slice is never nil.
	FetchUserList(ctx context.Context) ([]*model.User, error)
	// FetchUserByID returns the user with the given id, or an error matching
	// apperr.ErrUserNotFound when it does not exist.
	FetchUserByID(ctx context.Context, id int) (*model.User, error)
	// DeleteUser deletes the user with the given id.
	DeleteUser(ctx context.Context, id int) error
	// UpdateUser stores u under the given id, overwriting the existing row.
	UpdateUser(ctx context.Context, id int, u *model.User) error
	// GetUserNameByName looks up the single user with the given name. The
	// bool is false when no user matches.
	GetUserNameByName(ctx context.Context, name string) (*model.User, bool, error)
}

// userService is the default UserService backed by a UserRepository.
type userService struct {
	repo repository.UserRepository
}

// Compile-time interface check.
var _ UserService = (*userService)(nil)

// NewUserService returns a UserService that delegates persistence to repo.
// It replaces Spring's @Service + @Autowired UserDao wiring.
func NewUserService(repo repository.UserRepository) (UserService, error) {
	if repo == nil {
		return nil, errors.New("service: nil user repository")
	}
	return &userService{repo: repo}, nil
}

// SaveUser mirrors userDao.save(user).
func (s *userService) SaveUser(ctx context.Context, u *model.User) (*model.User, error) {
	if u == nil {
		return nil, errors.New("save user: nil user")
	}
	saved, err := s.repo.Save(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("service: save user: %w", err)
	}
	return saved, nil
}

// FetchUserList mirrors userDao.findAll().
func (s *userService) FetchUserList(ctx context.Context) ([]*model.User, error) {
	users, err := s.repo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: fetch user list: %w", err)
	}
	if users == nil {
		users = make([]*model.User, 0)
	}
	return users, nil
}

// FetchUserByID mirrors userDao.findById(id), raising UserNotFoundException
// ("User are not available") when absent.
func (s *userService) FetchUserByID(ctx context.Context, id int) (*model.User, error) {
	u, ok, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("service: fetch user %d: %w", id, err)
	}
	if !ok {
		// Returned unwrapped so handlers can use Message() directly; it still
		// matches apperr.ErrUserNotFound via errors.Is.
		return nil, apperr.NewUserNotFoundMsg(userNotAvailableMsg)
	}
	return u, nil
}

// DeleteUser mirrors userDao.deleteById(id).
//
// MIGRATION_NOTE: Spring Data's deleteById throws
// EmptyResultDataAccessException for a missing id (surfacing as HTTP 500).
// The repository returns repository.ErrNoRowsDeleted in that case; it is
// propagated wrapped, so callers can detect it with errors.Is.
func (s *userService) DeleteUser(ctx context.Context, id int) error {
	if err := s.repo.DeleteByID(ctx, id); err != nil {
		return fmt.Errorf("service: delete user %d: %w", id, err)
	}
	return nil
}

// UpdateUser mirrors `user.setId(id); userDao.save(user);`.
//
// MIGRATION_NOTE: as in the source, this has save() upsert semantics: if no
// user with id exists, a NEW row is inserted with a database-generated id
// (not necessarily id). The caller's user value has its ID set to id, just
// as the Java code mutated the passed entity.
func (s *userService) UpdateUser(ctx context.Context, id int, u *model.User) error {
	if u == nil {
		return errors.New("update user: nil user")
	}
	u.ID = id
	if _, err := s.repo.Save(ctx, u); err != nil {
		return fmt.Errorf("service: update user %d: %w", id, err)
	}
	return nil
}

// GetUserNameByName mirrors userDao.findByName(name). The Java version
// returned null when nothing matched; here that is (nil, false, nil).
// Multiple matches yield an error wrapping repository.ErrNonUniqueResult.
func (s *userService) GetUserNameByName(ctx context.Context, name string) (*model.User, bool, error) {
	u, ok, err := s.repo.FindByName(ctx, name)
	if err != nil {
		return nil, false, fmt.Errorf("service: get user by name: %w", err)
	}
	if !ok {
		return nil, false, nil
	}
	return u, true, nil
}
