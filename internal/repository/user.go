// Package repository provides persistence for the application's domain types.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"migrated-app/internal/model"
)

// ErrNoRowsDeleted is returned by DeleteByID when no user with the given id
// exists. It mirrors Spring Data's EmptyResultDataAccessException raised by
// JpaRepository.deleteById for a missing id.
var ErrNoRowsDeleted = errors.New("repository: no user entity with the given id exists")

// ErrNonUniqueResult is returned by FindByName when more than one user has
// the requested name. It mirrors Spring's IncorrectResultSizeDataAccessException
// raised by a single-result derived query that matches several rows.
var ErrNonUniqueResult = errors.New("repository: query did not return a unique result")

// UserRepository is the persistence contract for model.User, replacing the
// Spring Data JPA UserDao (JpaRepository<User, Integer> + findByName).
type UserRepository interface {
	// Save inserts u when it has no persisted row (or ID == 0), otherwise
	// updates every column of the existing row. It returns the saved user
	// carrying its (possibly newly generated) id.
	Save(ctx context.Context, u *model.User) (*model.User, error)
	// FindByID returns the user with the given id; the bool is false when
	// no such user exists.
	FindByID(ctx context.Context, id int) (*model.User, bool, error)
	// FindAll returns every user ordered by id. The slice is never nil.
	FindAll(ctx context.Context) ([]*model.User, error)
	// DeleteByID removes the user with the given id, returning
	// ErrNoRowsDeleted when it does not exist.
	DeleteByID(ctx context.Context, id int) error
	// FindByName returns the single user whose name exactly equals name;
	// the bool is false when none matches, and ErrNonUniqueResult is
	// returned when several match.
	FindByName(ctx context.Context, name string) (*model.User, bool, error)
}

// MySQLUserRepository implements UserRepository on top of database/sql with
// the go-sql-driver/mysql driver.
type MySQLUserRepository struct {
	db *sql.DB
}

// Compile-time interface check.
var _ UserRepository = (*MySQLUserRepository)(nil)

// NewMySQLUserRepository returns a UserRepository backed by db. The `user`
// table must already exist (see model.EnsureUserSchema).
func NewMySQLUserRepository(db *sql.DB) *MySQLUserRepository {
	return &MySQLUserRepository{db: db}
}

var (
	userSelectColumns = "`" + model.UserColumnID + "`, `" + model.UserColumnName + "`, `" +
		model.UserColumnEmail + "`, `" + model.UserColumnPassword + "`, `" +
		model.UserColumnRole + "`, `" + model.UserColumnAbout + "`"

	userTableQuoted = "`" + model.UserTable + "`"

	selectUserByIDSQL = "SELECT " + userSelectColumns + " FROM " + userTableQuoted +
		" WHERE `" + model.UserColumnID + "` = ?"

	lockUserByIDSQL = "SELECT `" + model.UserColumnID + "` FROM " + userTableQuoted +
		" WHERE `" + model.UserColumnID + "` = ? FOR UPDATE"

	selectAllUsersSQL = "SELECT " + userSelectColumns + " FROM " + userTableQuoted +
		" ORDER BY `" + model.UserColumnID + "`"

	selectUserByNameSQL = "SELECT " + userSelectColumns + " FROM " + userTableQuoted +
		" WHERE `" + model.UserColumnName + "` = ? LIMIT 2"

	insertUserSQL = "INSERT INTO " + userTableQuoted + " (`" + model.UserColumnName + "`, `" +
		model.UserColumnEmail + "`, `" + model.UserColumnPassword + "`, `" +
		model.UserColumnRole + "`, `" + model.UserColumnAbout + "`) VALUES (?, ?, ?, ?, ?)"

	updateUserSQL = "UPDATE " + userTableQuoted + " SET `" + model.UserColumnName + "` = ?, `" +
		model.UserColumnEmail + "` = ?, `" + model.UserColumnPassword + "` = ?, `" +
		model.UserColumnRole + "` = ?, `" + model.UserColumnAbout + "` = ? WHERE `" +
		model.UserColumnID + "` = ?"

	deleteUserByIDSQL = "DELETE FROM " + userTableQuoted + " WHERE `" + model.UserColumnID + "` = ?"
)

// Save performs JpaRepository.save upsert semantics inside a transaction:
//   - ID != 0 and a row with that id exists: UPDATE all five data columns
//     (nil fields are written as NULL, exactly like a JPA merge).
//   - otherwise: INSERT a new row; the database generates the id and any
//     supplied (non-existent) id is ignored.
//
// MIGRATION_NOTE: Hibernate's GenerationType.AUTO on MySQL draws ids from a
// hibernate_sequence table. The migrated schema (model.UserTableDDL) uses
// AUTO_INCREMENT instead, so ids come from LastInsertId. Ids are still
// database-generated and monotonically increasing; review if an existing
// database with a populated hibernate_sequence is being reused.
func (r *MySQLUserRepository) Save(ctx context.Context, u *model.User) (*model.User, error) {
	if u == nil {
		return nil, errors.New("save user: nil user")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("save user: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after a successful Commit

	saved := *u

	exists := false
	if u.ID != 0 {
		var lockedID int
		err := tx.QueryRowContext(ctx, lockUserByIDSQL, u.ID).Scan(&lockedID)
		switch {
		case err == nil:
			exists = true
		case errors.Is(err, sql.ErrNoRows):
			exists = false
		default:
			return nil, fmt.Errorf("save user %d: lock row: %w", u.ID, err)
		}
	}

	if exists {
		if _, err := tx.ExecContext(ctx, updateUserSQL,
			nullable(u.Name), nullable(u.Email), nullable(u.Password),
			nullable(u.Role), nullable(u.About), u.ID); err != nil {
			return nil, fmt.Errorf("save user %d: update: %w", u.ID, err)
		}
	} else {
		res, err := tx.ExecContext(ctx, insertUserSQL,
			nullable(u.Name), nullable(u.Email), nullable(u.Password),
			nullable(u.Role), nullable(u.About))
		if err != nil {
			return nil, fmt.Errorf("save user: insert: %w", err)
		}
		newID, err := res.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("save user: last insert id: %w", err)
		}
		saved.ID = int(newID)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("save user: commit: %w", err)
	}
	return &saved, nil
}

// FindByID returns the user with the given id (JpaRepository.findById).
func (r *MySQLUserRepository) FindByID(ctx context.Context, id int) (*model.User, bool, error) {
	u, err := scanUser(r.db.QueryRowContext(ctx, selectUserByIDSQL, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("find user by id %d: %w", id, err)
	}
	return u, true, nil
}

// FindAll returns every user ordered by id (JpaRepository.findAll).
func (r *MySQLUserRepository) FindAll(ctx context.Context) ([]*model.User, error) {
	rows, err := r.db.QueryContext(ctx, selectAllUsersSQL)
	if err != nil {
		return nil, fmt.Errorf("find all users: %w", err)
	}
	defer rows.Close()

	users := make([]*model.User, 0)
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("find all users: scan: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find all users: iterate: %w", err)
	}
	return users, nil
}

// DeleteByID removes the user with the given id (JpaRepository.deleteById).
func (r *MySQLUserRepository) DeleteByID(ctx context.Context, id int) error {
	res, err := r.db.ExecContext(ctx, deleteUserByIDSQL, id)
	if err != nil {
		return fmt.Errorf("delete user %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete user %d: rows affected: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("delete user %d: %w", id, ErrNoRowsDeleted)
	}
	return nil
}

// FindByName returns the single user whose name equals name (the derived
// query UserDao.findByName). Comparison follows the column's collation, as
// the JPA-generated `WHERE user_name = ?` did.
func (r *MySQLUserRepository) FindByName(ctx context.Context, name string) (*model.User, bool, error) {
	rows, err := r.db.QueryContext(ctx, selectUserByNameSQL, name)
	if err != nil {
		return nil, false, fmt.Errorf("find user by name: %w", err)
	}
	defer rows.Close()

	var found *model.User
	count := 0
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, false, fmt.Errorf("find user by name: scan: %w", err)
		}
		count++
		if count == 1 {
			found = u
		}
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("find user by name: iterate: %w", err)
	}
	switch count {
	case 0:
		return nil, false, nil
	case 1:
		return found, true, nil
	default:
		return nil, false, fmt.Errorf("find user by name %q: %w", name, ErrNonUniqueResult)
	}
}

// rowScanner is satisfied by *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(s rowScanner) (*model.User, error) {
	var (
		u                                  model.User
		name, email, password, role, about sql.NullString
	)
	if err := s.Scan(&u.ID, &name, &email, &password, &role, &about); err != nil {
		return nil, err
	}
	u.Name = fromNull(name)
	u.Email = fromNull(email)
	u.Password = fromNull(password)
	u.Role = fromNull(role)
	u.About = fromNull(about)
	return &u, nil
}

func nullable(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func fromNull(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}
