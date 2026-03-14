# AGENTS.md — GoLoginApp

Containerized CLI login system in Go with user registration, authentication, optional TOTP-based 2FA, and session management backed by PostgreSQL.

## Tech Stack

| Layer        | Technology                              |
|--------------|-----------------------------------------|
| Language     | Go 1.25                                 |
| Database     | PostgreSQL 16 (Docker)                  |
| DB Driver    | pgx v5.8.0                              |
| Migrations   | golang-migrate v4.19.1 (embedded SQL)   |
| CLI          | chzyer/readline v1.5.1                  |
| Terminal UI  | pterm v0.12.83                          |
| 2FA / TOTP   | pquerna/otp v1.5.0                      |
| QR Code      | mdp/qrterminal v3.2.1                   |
| Passwords    | golang.org/x/crypto bcrypt              |
| Test mocking | pashagolub/pgxmock v3.4.0               |
| Containers   | Docker + Docker Compose                 |

## Project Structure

```
golang-cli-app/
├── cmd/main.go                     # Entry point — wires all services
├── internal/
│   ├── auth/
│   │   ├── service.go              # Register, Login, lockout logic + dbIface
│   │   ├── session.go              # Session lookup and delete
│   │   └── totp.go                 # TOTP generate, enable, disable
│   ├── cli/
│   │   ├── router.go               # Readline loop, command dispatch
│   │   ├── commands_pre.go         # register, login, help, exit
│   │   └── commands_post.go        # whoami, enable-2fa, disable-2fa, logout, help
│   ├── config/config.go            # Env var loading with defaults
│   ├── db/
│   │   ├── db.go                   # Connection pool + embedded migration runner
│   │   └── migrations/             # SQL files (embedded into binary via go:embed)
│   └── models/user.go              # User and Session structs
├── Dockerfile                      # Multi-stage build
└── docker-compose.yml              # App + PostgreSQL with health check
```

## Commands

```bash
# Install / sync dependencies
go mod tidy

# Build binary
go build -o loginapp ./cmd/main.go

# Run all unit tests (no database required)
go test ./internal/... -v

# Run tests for a specific package
go test ./internal/auth/...
go test ./internal/models/...
go test ./internal/config/...

# Start PostgreSQL only (for local development)
docker compose up -d postgres

# Run app locally (requires postgres running)
DB_URL="postgres://user:pass@localhost:5432/loginapp?sslmode=disable" go run ./cmd/main.go

# Run app in Docker (interactive — must use 'run', not 'up')
docker compose up -d postgres
docker compose run --rm app
```

## Key Conventions

### DB interface for testability
All auth services accept `dbIface` (defined in `service.go`), not `*pgxpool.Pool` directly.
This allows pgxmock injection in tests without changing the public API.

```go
// Good — testable
func NewService(db dbIface, cfg *config.Config) *Service

// Bad — not testable without real DB
func NewService(db *pgxpool.Pool, cfg *config.Config) *Service
```

### Error handling
Return sentinel errors; wrap with `%w` only when adding context.

```go
// Good
return nil, ErrInvalidCredentials

// Good — adds lockout time context
return nil, fmt.Errorf("%w, try again after %s", ErrAccountLocked, ...)

// Bad — swallows the original error type
return nil, errors.New("invalid credentials")
```

### Tests
Use `pgxmock.NewPool()` for DB-dependent tests. Always call `mock.ExpectationsWereMet()` at the end.

```go
mock, _ := pgxmock.NewPool()
defer mock.Close()
mock.ExpectExec("INSERT INTO users").WithArgs(...).WillReturnResult(...)
// ... call service ...
if err := mock.ExpectationsWereMet(); err != nil { t.Error(err) }
```

### SQL migrations
Add new migrations as numbered SQL files in `internal/db/migrations/`.
They are embedded into the binary and run automatically on startup.

```
001_init.up.sql / 001_init.down.sql
002_add_column.up.sql / 002_add_column.down.sql   ← next migration
```

## Environment Variables

| Variable              | Default                                                        | Description                        |
|-----------------------|----------------------------------------------------------------|------------------------------------|
| `DB_URL`              | `postgres://user:pass@localhost:5432/loginapp?sslmode=disable` | PostgreSQL connection string        |
| `SESSION_TIMEOUT`     | `30m`                                                          | Session validity duration           |
| `MAX_FAILED_ATTEMPTS` | `5`                                                            | Failed logins before lockout        |
| `LOCKOUT_DURATION`    | `15m`                                                          | Account lockout duration            |

## AI Boundaries

### Always do
- Run `go test ./internal/...` after any change to `internal/auth/` or `internal/models/`
- Add mock expectations for every DB call in new service tests
- Follow existing sentinel error pattern when adding new error types
- Keep `dbIface` minimal — only add methods that are actually used

### Ask first
- Adding a new SQL migration (schema changes affect running containers)
- Adding a new dependency to `go.mod`
- Changing `docker-compose.yml` service configuration
- Modifying the `dbIface` interface (breaks all mock implementations)

### Never do
- Commit real credentials or secrets anywhere in the codebase
- Use `*pgxpool.Pool` directly in service structs (breaks testability)
- Skip `mock.ExpectationsWereMet()` in tests
- Use `bcrypt.MinCost` outside of test files
- Modify `go.sum` manually
