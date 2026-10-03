// Package model contains the domain types persisted by the application,
// together with the DDL that creates their tables at startup.
package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Table and column names for the User entity. The source sets @Table/@Column
// names; Spring Boot's default physical naming strategy lowercases them, so
// the schema uses the lowercased identifiers. `user` is a reserved word in
// both MySQL and PostgreSQL and is always quoted in the generated SQL.
const (
	UserTable          = "user"
	UserColumnID       = "user_id"
	UserColumnName     = "user_name"
	UserColumnEmail    = "user_email"
	UserColumnPassword = "user_password"
	UserColumnRole     = "user_role"
	UserColumnAbout    = "user_about"

	// UserAboutMaxLength mirrors @Column(length = 500) on User_About.
	UserAboutMaxLength = 500

	// userDefaultVarcharLength is the JPA default @Column length (255) used
	// for every String column that does not declare an explicit length.
	userDefaultVarcharLength = 255

	// userEmailUniqueConstraint names the constraint generated from
	// @Column(unique = true) on User_Email.
	userEmailUniqueConstraint = "uk_user_user_email"
)

// Dialect identifies the SQL dialect used to create the schema.
type Dialect int

const (
	// DialectMySQL generates MySQL/MariaDB DDL (the source project's database).
	DialectMySQL Dialect = iota + 1
	// DialectPostgres generates PostgreSQL DDL.
	DialectPostgres
)

// String returns a human-readable dialect name.
func (d Dialect) String() string {
	switch d {
	case DialectMySQL:
		return "mysql"
	case DialectPostgres:
		return "postgres"
	default:
		return "unknown(" + strconv.Itoa(int(d)) + ")"
	}
}

// DialectFromURL infers the SQL dialect from a database URL / DSN such as
// config.Config.DatabaseURL. URLs with a postgres:// or postgresql:// scheme
// map to DialectPostgres; everything else (mysql:// URLs and go-sql-driver
// DSNs like "user:pass@tcp(host:3306)/db") maps to DialectMySQL.
func DialectFromURL(databaseURL string) Dialect {
	lower := strings.ToLower(strings.TrimSpace(databaseURL))
	if strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://") {
		return DialectPostgres
	}
	return DialectMySQL
}

// ErrUnknownDialect is returned when a DDL operation receives an unsupported Dialect.
var ErrUnknownDialect = errors.New("model: unknown SQL dialect")

// SchemaExecer is the minimal database capability needed to create the
// schema. *sql.DB, *sql.Conn and *sql.Tx all satisfy it.
type SchemaExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// UserTableDDL returns the CREATE TABLE IF NOT EXISTS statement for the
// User entity in the given dialect. The columns mirror the JPA mapping:
//   - user_id:       INT primary key, auto-generated (@Id @GeneratedValue)
//   - user_name:     VARCHAR(255), nullable (@NotBlank is app-level only)
//   - user_email:    VARCHAR(255), nullable, UNIQUE (@Column(unique = true))
//   - user_password: VARCHAR(255), nullable
//   - user_role:     VARCHAR(255), nullable
//   - user_about:    VARCHAR(500), nullable (@Column(length = 500))
func UserTableDDL(d Dialect) (string, error) {
	varchar := "VARCHAR(" + strconv.Itoa(userDefaultVarcharLength) + ")"
	aboutVarchar := "VARCHAR(" + strconv.Itoa(UserAboutMaxLength) + ")"

	switch d {
	case DialectMySQL:
		q := func(id string) string { return "`" + id + "`" }
		var b strings.Builder
		b.WriteString("CREATE TABLE IF NOT EXISTS " + q(UserTable) + " (\n")
		b.WriteString("  " + q(UserColumnID) + " INT NOT NULL AUTO_INCREMENT,\n")
		b.WriteString("  " + q(UserColumnName) + " " + varchar + " NULL,\n")
		b.WriteString("  " + q(UserColumnEmail) + " " + varchar + " NULL,\n")
		b.WriteString("  " + q(UserColumnPassword) + " " + varchar + " NULL,\n")
		b.WriteString("  " + q(UserColumnRole) + " " + varchar + " NULL,\n")
		b.WriteString("  " + q(UserColumnAbout) + " " + aboutVarchar + " NULL,\n")
		b.WriteString("  PRIMARY KEY (" + q(UserColumnID) + "),\n")
		b.WriteString("  CONSTRAINT " + q(userEmailUniqueConstraint) + " UNIQUE (" + q(UserColumnEmail) + ")\n")
		b.WriteString(") ENGINE=InnoDB")
		return b.String(), nil

	case DialectPostgres:
		q := func(id string) string { return `"` + id + `"` }
		var b strings.Builder
		b.WriteString("CREATE TABLE IF NOT EXISTS " + q(UserTable) + " (\n")
		b.WriteString("  " + q(UserColumnID) + " SERIAL PRIMARY KEY,\n")
		b.WriteString("  " + q(UserColumnName) + " " + varchar + " NULL,\n")
		b.WriteString("  " + q(UserColumnEmail) + " " + varchar + " NULL,\n")
		b.WriteString("  " + q(UserColumnPassword) + " " + varchar + " NULL,\n")
		b.WriteString("  " + q(UserColumnRole) + " " + varchar + " NULL,\n")
		b.WriteString("  " + q(UserColumnAbout) + " " + aboutVarchar + " NULL,\n")
		b.WriteString("  CONSTRAINT " + q(userEmailUniqueConstraint) + " UNIQUE (" + q(UserColumnEmail) + ")\n")
		b.WriteString(")")
		return b.String(), nil

	default:
		return "", fmt.Errorf("user table ddl for %s: %w", d, ErrUnknownDialect)
	}
}

// EnsureUserSchema creates the `user` table (with its auto-increment primary
// key, unique email constraint and VARCHAR(500) about column) if it does not
// already exist. It replaces Hibernate's spring.jpa.hibernate.ddl-auto=update
// table creation and MUST be called once at application startup, right after
// the database connection is opened and before the HTTP server starts:
//
//	db, err := sql.Open(driver, cfg.DatabaseURL)
//	...
//	if err := model.EnsureUserSchema(ctx, db, model.DialectFromURL(cfg.DatabaseURL)); err != nil {
//		return fmt.Errorf("init schema: %w", err)
//	}
//
// The statement is idempotent, so calling it on every boot is safe.
//
// MIGRATION_NOTE: ddl-auto=update also ALTERs an existing table to add
// columns that are missing from it. This function only creates the table
// when absent; it does not diff or alter a pre-existing table.
func EnsureUserSchema(ctx context.Context, db SchemaExecer, d Dialect) error {
	if db == nil {
		return errors.New("ensure user schema: nil database handle")
	}
	ddl, err := UserTableDDL(d)
	if err != nil {
		return fmt.Errorf("ensure user schema: %w", err)
	}
	if _, err := db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("ensure user schema: create table %q: %w", UserTable, err)
	}
	return nil
}

// NameBlankMessage is the @NotBlank message declared on User.name.
const NameBlankMessage = "please Add the department Name"

// ErrNameBlank is returned by User.Validate when the name is nil or blank.
var ErrNameBlank = errors.New(NameBlankMessage)

// User maps to the `user` table. String fields are pointers so that JSON
// null / SQL NULL round-trip exactly like Java's nullable String fields.
//
// MIGRATION_NOTE: Password is serialized in JSON responses for parity with the
// Lombok/Jackson source behaviour. This leaks credentials and should be
// reviewed as a security follow-up.
type User struct {
	ID       int     `json:"id"`
	Name     *string `json:"name"`
	Email    *string `json:"email"`
	Password *string `json:"password"`
	Role     *string `json:"role"`
	About    *string `json:"about"`
}

// NewUser returns an empty User with default (zero/nil) field values,
// equivalent to the Lombok no-args constructor.
func NewUser() *User {
	return &User{}
}

// NewUserWithAll returns a User with every field set, in declaration order,
// equivalent to the Lombok all-args constructor. The source entity declares
// About before Role, so the parameter order is (id, name, email, password,
// about, role).
func NewUserWithAll(id int, name, email, password, about, role *string) *User {
	return &User{
		ID:       id,
		Name:     name,
		Email:    email,
		Password: password,
		Role:     role,
		About:    about,
	}
}

// GetID returns the user id.
func (u *User) GetID() int { return u.ID }

// SetID sets the user id.
func (u *User) SetID(id int) { u.ID = id }

// GetName returns the user name (nil when unset).
func (u *User) GetName() *string { return u.Name }

// SetName sets the user name.
func (u *User) SetName(name *string) { u.Name = name }

// GetEmail returns the user email (nil when unset).
func (u *User) GetEmail() *string { return u.Email }

// SetEmail sets the user email.
func (u *User) SetEmail(email *string) { u.Email = email }

// GetPassword returns the user password (nil when unset).
func (u *User) GetPassword() *string { return u.Password }

// SetPassword sets the user password.
func (u *User) SetPassword(password *string) { u.Password = password }

// GetRole returns the user role (nil when unset).
func (u *User) GetRole() *string { return u.Role }

// SetRole sets the user role.
func (u *User) SetRole(role *string) { u.Role = role }

// GetAbout returns the user "about" text (nil when unset).
func (u *User) GetAbout() *string { return u.About }

// SetAbout sets the user "about" text.
func (u *User) SetAbout(about *string) { u.About = about }

// Equal reports whether u and other have equal values for all six fields
// (Lombok @Data equals semantics). Two nil Users are equal.
func (u *User) Equal(other *User) bool {
	if u == nil || other == nil {
		return u == other
	}
	return u.ID == other.ID &&
		strPtrEqual(u.Name, other.Name) &&
		strPtrEqual(u.Email, other.Email) &&
		strPtrEqual(u.Password, other.Password) &&
		strPtrEqual(u.Role, other.Role) &&
		strPtrEqual(u.About, other.About)
}

// String renders the User in Lombok's toString format, listing every field
// (including password, for parity) with nil rendered as "null".
func (u *User) String() string {
	if u == nil {
		return "null"
	}
	var b strings.Builder
	b.WriteString("User(id=")
	b.WriteString(strconv.Itoa(u.ID))
	fmt.Fprintf(&b, ", name=%s", strPtrString(u.Name))
	fmt.Fprintf(&b, ", email=%s", strPtrString(u.Email))
	fmt.Fprintf(&b, ", password=%s", strPtrString(u.Password))
	fmt.Fprintf(&b, ", role=%s", strPtrString(u.Role))
	fmt.Fprintf(&b, ", about=%s", strPtrString(u.About))
	b.WriteString(")")
	return b.String()
}

// Validate enforces the @NotBlank constraint on Name. It returns ErrNameBlank
// when Name is nil or contains only characters <= U+0020 (Java String.trim
// semantics).
func (u *User) Validate() error {
	if u.Name == nil || javaTrim(*u.Name) == "" {
		return ErrNameBlank
	}
	return nil
}

// UserBuilder is a fluent builder for User, equivalent to Lombok's @Builder.
type UserBuilder struct {
	u User
}

// NewUserBuilder returns a new, empty UserBuilder.
func NewUserBuilder() *UserBuilder {
	return &UserBuilder{}
}

// ID sets the id on the builder.
func (b *UserBuilder) ID(id int) *UserBuilder { b.u.ID = id; return b }

// Name sets the name on the builder.
func (b *UserBuilder) Name(name *string) *UserBuilder { b.u.Name = name; return b }

// Email sets the email on the builder.
func (b *UserBuilder) Email(email *string) *UserBuilder { b.u.Email = email; return b }

// Password sets the password on the builder.
func (b *UserBuilder) Password(password *string) *UserBuilder { b.u.Password = password; return b }

// Role sets the role on the builder.
func (b *UserBuilder) Role(role *string) *UserBuilder { b.u.Role = role; return b }

// About sets the about text on the builder.
func (b *UserBuilder) About(about *string) *UserBuilder { b.u.About = about; return b }

// Build returns a new User populated from the builder. Each call returns an
// independent copy.
func (b *UserBuilder) Build() *User {
	u := b.u
	return &u
}

// StringPtr is a convenience helper returning a pointer to s.
func StringPtr(s string) *string { return &s }

func strPtrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func strPtrString(s *string) string {
	if s == nil {
		return "null"
	}
	return *s
}

// javaTrim mimics java.lang.String#trim: strips leading/trailing chars <= U+0020.
func javaTrim(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return r <= ' ' })
}
