// Package model contains the domain types persisted by the application.
package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Table and column names for the User entity. Hibernate's default physical
// naming strategy lowercases the @Table/@Column names, so the schema uses the
// lowercased identifiers. `user` is a reserved word in MySQL and must be
// backtick-quoted in SQL.
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
)

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
// equivalent to the Lombok all-args constructor.
func NewUserWithAll(id int, name, email, password, role, about *string) *User {
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
