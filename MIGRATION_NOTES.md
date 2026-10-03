# Migration Notes

**Model self-assessed confidence:** 94% (not a measured result)  
**Build at time of writing:** passed  
**Target unit tests:** failed  
**Behavior compared with the original:** passed

The final recommendation is in the pull request description.

---

## What was migrated

- `src/main/java/com/smartContact/error/UserNotFoundException.java` → `internal/apperr/errors.go` (88% confidence)
- `src/main/java/com/smartContact/model/User.java` → `internal/model/user.go` (77% confidence) ⚠️ needs review
- `src/main/java/com/smartContact/model/ErrorMessage.java` → `internal/model/errormessage.go` (89% confidence)
- `src/main/java/com/smartContact/repository/UserDao.java` → `internal/repository/user.go` (88% confidence)
- `src/main/java/com/smartContact/error/RestResponseEntityExceptionHandling.java` → `internal/httpapi/errors.go` (86% confidence)
- `src/main/java/com/smartContact/service/UserService.java` → `internal/service/user.go` (85% confidence)
- `src/main/java/com/smartContact/service/UserServiceImp.java` → `internal/service/user_test.go` (88% confidence)
- `src/main/java/com/smartContact/Controller/UserController.java` → `internal/httpapi/handler.go` (83% confidence) ⚠️ needs review

## Components that could not be automatically migrated

These components require manual implementation. The migrated code contains
`MIGRATION_NOTE` comments at the relevant locations.

### `User (@Entity schema auto-generation via Hibernate ddl-auto)` in `src/main/java/com/smartContact/model/User.java`
**Reason:** Hibernate derives DDL implicitly from annotations at boot. Most target stacks have no 1:1 equivalent that infers DDL from a plain class with identical semantics (AUTO id strategy, default VARCHAR(255) lengths, unique constraints).
**Suggestion:** Manual rewrite: define the model in the target ORM (SQLAlchemy, Prisma, TypeORM, GORM, EF Core, Sequelize, etc.) with explicit column names, types and lengths. Wire real schema creation so it actually runs at startup (e.g. Base.metadata.create_all(engine), sequelize.sync(), TypeORM synchronize, or AutoMigrate), or put migration execution into the app entrypoint. Verify with a boot test that the table exists with all six columns and the unique constraint on User_Email.

### `Lombok annotations (@Data, @Builder, @NoArgsConstructor, @AllArgsConstructor)` in `src/main/java/com/smartContact/model/User.java`
**Reason:** These are compile-time Java code generators with no runtime presence, so there is nothing to port directly.
**Suggestion:** Replace them with idiomatic target constructs (dataclass or pydantic model, TS interface or class, Go struct). Explicitly exclude the password from toString and serialization.

### `UserDao (runtime-generated implementation)` in `src/main/java/com/smartContact/repository/UserDao.java`
**Reason:** Spring Data creates the implementation from the interface signature and method-name parsing at runtime. Go has no equivalent reflection-based query derivation.
**Suggestion:** Manual rewrite: write a concrete Go struct implementing the repository interface with hand-written SQL (sqlx) or gorm calls for each method actually used.

### `Persistence context / dirty checking / lazy loading` in `src/main/java/com/smartContact/repository/UserDao.java`
**Reason:** Hibernate's first-level cache, automatic flush and lazy proxies are implicit behaviors with no direct Go equivalent.
**Suggestion:** Make all updates explicit, load associations explicitly (JOIN or gorm Preload), and manage transactions explicitly in the service layer.

## Observer agent findings

The Observer agent monitored the migration and identified these patterns:

- **After 3 modules:** The target expert migrates only the visible class structure and ignores behavior that Spring/JPA provides implicitly through annotations and config, especially schema creation and id generation.
- **After 6 modules:** Spring/JPA behavior that comes from configuration and annotations (ddl-auto schema creation, column constraints) is not migrated into explicit Go DDL. Separately, the validator adds noise by flagging intentional, documented, source-faithful differences as failed specs.

## Files requiring manual review

These files were migrated but scored below the confidence threshold.
Review them carefully before merging.

### `src/main/java/com/smartContact/model/User.java`
Confidence: 77%
Issues:
  - [warning] EnsureUserSchema is defined, but nothing in the target calls it, and there is no main or DB-open code yet. So I can't confirm the table is actually created at boot.

### `src/main/java/com/smartContact/Controller/UserController.java`
Confidence: 83%
