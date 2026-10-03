# java-crud-api (Go port)

A CRUD REST API for managing `User` records, migrated from Java/Spring to Go using the standard library. The original project is `abdullaharshadd/java-crud-api`, whose Java package is `com.smartContact`.

> **Migration status: incomplete. Read [Known limitations](#known-limitations) and [Manual review required](#manual-review-required) before you rely on this code.**
>
> | Check | Result |
> |---|---|
> | Build | ✅ Passed |
> | Unit tests (`go test ./...`) | ❌ **Failed** |
> | Behavior compared with the original Spring app | ✅ Passed |
> | Modules migrated | 8 / 8 |
>
> The migration tool self-assessed its confidence at 94%. That figure is an estimate. It is not a measured result.

---

## Tech stack

| Concern | Original | Migrated |
|---|---|---|
| Language | Java | Go (version is declared in `go.mod`) |
| HTTP layer | Spring MVC (`@RestController`) | Go standard library HTTP (`cmd/server/router.go`, `internal/httpapi/`) |
| Persistence | Spring Data JPA + Hibernate | Hand-written repository (`internal/repository/user.go`) |
| Model boilerplate | Lombok | Plain Go structs (`internal/model/`) |
| Dependency management | Maven/Gradle | Go modules (`go.mod`) |
| Containerization | n/a | `Dockerfile`, `docker-compose.yml` |

The database driver and any third-party modules are listed in `go.mod`.

---

## Prerequisites

- A Go toolchain that matches the version declared in `go.mod`
- A reachable relational database, configured through `DATABASE_URL`
- Optional: Docker and Docker Compose, which can use the provided `Dockerfile` and `docker-compose.yml`

The detected commands assume the project lives at `/app`, which is the container layout. If you cloned the repository somewhere else, replace `/app` with your checkout path.

---

## Getting started

Run these steps in order, starting from a fresh clone.

### 1. Install dependencies and build

```sh
cd /app && mkdir -p /app/bin && go mod tidy && go mod download && go build -o /app/bin/server ./cmd/server
```

### 2. Resolve dependencies again

During migration this command was run as a separate step after install. It succeeded, so run it again before starting the server:

```sh
go mod tidy
```

### 3. Set environment variables

```sh
export DATABASE_URL="<your database connection string>"
export PORT="<port to listen on>"
export JWT_SECRET="<secret used for JWT signing>"
```

See [Environment variables](#environment-variables) for details.

### 4. Prepare the database

No database setup command was detected.

The original app relied on Hibernate `ddl-auto` to create the `User` table automatically, and that behavior was **not** ported. See [Known limitations](#known-limitations). You must create the users table yourself before you start the server. The schema should match the original:

- six columns
- a unique constraint on `User_Email`

Check `internal/model/user.go` and `internal/repository/user.go` to get the exact table and column names that the Go code expects.

### 5. Run the server

```sh
/app/bin/server
```

---

## Running tests

```sh
cd /app && go test ./...
```

> ❌ **This test run currently fails.** The only test file is `internal/service/user_test.go`. Fixing these failures is an open task. Until it is done, do not treat the service layer as verified by unit tests.

---

## Environment variables

| Variable | Required | Description |
|---|---|---|
| `DATABASE_URL` | Yes | Connection string for the backing database. |
| `PORT` | Yes | Port the HTTP server listens on. |
| `JWT_SECRET` | Yes | Secret used for JWT handling. |

No default values are documented. Read `internal/config/config.go` to see how each variable is loaded and whether a fallback exists.

---

## Architecture overview

```
cmd/server/
  main.go               Entry point: loads config, wires dependencies, starts the HTTP server
  router.go             Route registration (replaces Spring @RequestMapping)
internal/
  config/config.go      Environment-based configuration (replaces application.properties)
  model/
    user.go             User struct (replaces the Lombok-annotated @Entity)
    errormessage.go     Error response payload model
  repository/user.go    Hand-written data access (replaces the Spring Data UserDao interface)
  service/
    user.go             Business logic for users
    user_test.go        Unit tests for the service layer (currently failing)
  httpapi/
    handler.go          HTTP handlers for user CRUD (replaces UserController)
    errors.go           Maps application errors to HTTP responses (replaces Spring exception handling)
  apperr/errors.go      Application/domain error types
```

Requests flow in one direction:

**router → httpapi handler → service → repository → database**

Dependencies are wired explicitly in `cmd/server/main.go`. Nothing is injected through a container.

---

## Migration notes

The Spring implicit behaviors were replaced with explicit Go code as follows:

- **Dependency injection.** Spring's container is gone. Components are constructed and passed in manually in `cmd/server/main.go`.
- **Controllers to handlers.** `UserController` became the handlers in `internal/httpapi/handler.go`. Routes are registered in `cmd/server/router.go`.
- **Exception handling.** Spring exception handling became:
  - explicit error types in `internal/apperr/errors.go`
  - HTTP error mapping in `internal/httpapi/errors.go`
  - the response shape in `internal/model/errormessage.go`
- **Lombok removed.** The `@Data`, `@Builder`, `@NoArgsConstructor` and `@AllArgsConstructor` annotations were replaced by a plain struct in `internal/model/user.go`.
- **Spring Data repository.** `UserDao` was generated at runtime from method names. It is now a concrete, hand-written repository in `internal/repository/user.go`.
- **Persistence context.** Hibernate's first-level cache, dirty checking, automatic flush and lazy loading no longer exist. Updates must be explicit, and transactions must be managed explicitly.
- **Configuration.** Values now come from environment variables through `internal/config/config.go`.

---

## Known limitations

1. **No automatic schema creation.**
   - *Original component:* `User` entity with Hibernate `ddl-auto`.
   - *Why it was not ported:* Hibernate derived the DDL from annotations when the app booted. That included the AUTO id strategy, default `VARCHAR(255)` lengths and the unique constraint. Go has no 1:1 equivalent.
   - *What you must do:* Create the table manually or add a migration step. Then confirm that the table has all six columns and the unique constraint on `User_Email`.

2. **Lombok has no runtime equivalent.** The annotations generated code at compile time, so there was nothing to port directly. Their behavior was approximated with a Go struct. The original guidance is to explicitly exclude the password from string and serialization output. Check that the Go struct does this (see below).

3. **No runtime query derivation.** Spring Data generated the `UserDao` implementation at runtime. Each method that was used now has to be hand-written in `internal/repository/user.go`.

4. **No Hibernate persistence context.** Code that relied on implicit flush or dirty checking could silently stop persisting changes unless the Go code saves them explicitly.

5. **Unit tests fail.** `go test ./...` does not pass. See [Running tests](#running-tests).

---

## Manual review required

The migration flagged these original modules as **low confidence**:

- **`User` model.** Review `internal/model/user.go` and `internal/repository/user.go`:
  - [ ] Column names, types and lengths match the original schema, including `VARCHAR(255)` defaults.
  - [ ] The ID generation strategy matches the original AUTO behavior.
  - [ ] The unique constraint on `User_Email` exists and is enforced. Duplicate emails should produce the correct error response.
  - [ ] The password is excluded from JSON serialization and any string or log output.

- **`UserController`.** Review `internal/httpapi/handler.go` and `cmd/server/router.go`:
  - [ ] Every original endpoint is registered with the same path and HTTP method.
  - [ ] Status codes, request validation and response bodies match the original, including error responses produced through `internal/httpapi/errors.go`.

- **`UserDao` and persistence behavior.** Review `internal/repository/user.go` and `internal/service/user.go`:
  - [ ] Every repository method the original actually used is implemented with correct SQL.
  - [ ] Updates are persisted explicitly, because there is no dirty checking.
  - [ ] Multi-step operations run inside explicit transactions where the original depended on Spring's transactional behavior.

- **Tests.** Review `internal/service/user_test.go`:
  - [ ] Find the cause of the failures and fix it. The cause could be in the tests or in `internal/service/user.go`.

The behavior comparison with the original passed. Even so, complete the checklist above before you deploy, because the comparison does not replace this review.