# GoLoginApp — Containerized CLI Login System

A secure command-line login system built in Go with user registration, authentication, optional TOTP-based two-factor authentication, and session management. Runs fully containerized using Docker and PostgreSQL.

---

## Features

- User registration with bcrypt password hashing
- Login with username and password
- Optional TOTP-based 2FA (Google Authenticator compatible)
- Account lockout after configurable failed attempts
- Session management with configurable timeout
- Interactive CLI with command history and tab-completion
- PostgreSQL persistence across container restarts
- Auto-runs database migrations on startup

---

## Project Structure

```
golang-cli-app/
├── cmd/
│   └── main.go                     # Entry point
├── internal/
│   ├── auth/
│   │   ├── service.go              # Register, login, lockout logic
│   │   ├── session.go              # Session lookup and invalidation
│   │   └── totp.go                 # TOTP generate, enable, disable
│   ├── cli/
│   │   ├── router.go               # Readline loop and command dispatch
│   │   ├── commands_pre.go         # Commands before login: register, login, help, exit
│   │   └── commands_post.go        # Commands after login: whoami, enable-2fa, disable-2fa, logout, help
│   ├── config/
│   │   └── config.go               # Environment variable loading
│   ├── db/
│   │   ├── db.go                   # Postgres connection pool + migration runner
│   │   └── migrations/
│   │       ├── 001_init.up.sql     # Create users and sessions tables
│   │       └── 001_init.down.sql   # Drop tables (rollback)
│   └── models/
│       └── user.go                 # User and Session structs
├── scripts/                        # Helper scripts
├── Dockerfile
├── docker-compose.yml
└── README.md
```

---

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and Docker Compose
- Go 1.24+ (only needed for local development without Docker)

---

## Quick Start (Docker)

### 1. Clone the repository

```bash
git clone https://github.com/rishant-tech/golang-cli-app.git
cd golang-cli-app
```

### 2. Start PostgreSQL in the background

```bash
docker compose up -d postgres
```

### 3. Run the app interactively

```bash
docker compose run --rm app
```

> **Why `run` instead of `up`?**
> `docker compose up` streams logs from all services but does not wire your terminal's stdin to any specific container.
> `docker compose run` properly connects your keyboard input to the container, which is required for an interactive CLI.

You should see:

```
GoLoginApp — Secure CLI Login System
Type 'help' to see available commands.

>
```

---

## Local Development (without Docker)

### 1. Start a local PostgreSQL instance

```bash
docker compose up -d postgres
```

### 2. Install Go dependencies

```bash
go mod tidy
```

### 3. Run the app

```bash
DB_URL="postgres://user:pass@localhost:5432/loginapp?sslmode=disable" go run ./cmd/main.go
```

---

## CLI Commands

### Before Login

| Command    | Description                        |
|------------|------------------------------------|
| `register` | Create a new user account          |
| `login`    | Login with username and password   |
| `help`     | Show available commands            |
| `exit`     | Quit the application               |

### After Login

| Command       | Description                                  |
|---------------|----------------------------------------------|
| `whoami`      | Show account details and session information |
| `enable-2fa`  | Enable TOTP-based two-factor authentication  |
| `disable-2fa` | Disable two-factor authentication            |
| `logout`      | End your current session                     |
| `help`        | Show available commands                      |
| `exit`        | Quit the application                         |

---

## Usage Walkthrough

### Register a new user

```
> register
Username: alice
Password:
Confirm password:
✓ User 'alice' registered successfully. You can now login.
```

### Login

```
> login
Username: alice
Password:
✓ Welcome back, alice!

 Username        │ alice
 Registered      │ 2026-03-13 10:00:00 UTC
 MFA Status      │ Disabled
 Session Expires │ 2026-03-13 10:30:00 UTC
 Last Login      │ Never (this is your first login)
```

### Enable 2FA

```
[alice] > enable-2fa

Scan the QR code below with Google Authenticator or a compatible app:

█████████████████████████
█ ... QR CODE ... ██████
█████████████████████████

Can't scan? Enter this key manually: JBSWY3DPEHPK3PXP

Enter the 6-digit code from your app to verify: 123456
✓ 2FA has been enabled on your account.
```

### Login with 2FA enabled

```
> login
Username: alice
Password:
2FA Code: 123456
✓ Welcome back, alice!
```

---

## Configuration

All settings are controlled via environment variables. You can override them in `docker-compose.yml` or pass them directly when running locally.

| Variable             | Default                                                | Description                                      |
|----------------------|--------------------------------------------------------|--------------------------------------------------|
| `DB_URL`             | `postgres://user:pass@localhost:5432/loginapp?sslmode=disable` | PostgreSQL connection string          |
| `SESSION_TIMEOUT`    | `30m`                                                  | How long a session stays valid (e.g. `1h`, `30m`) |
| `MAX_FAILED_ATTEMPTS`| `5`                                                    | Failed login attempts before account lockout     |
| `LOCKOUT_DURATION`   | `15m`                                                  | How long an account stays locked after lockout   |

---

## Database Schema

```sql
-- Users table
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username        VARCHAR(50) UNIQUE NOT NULL,
    password_hash   TEXT NOT NULL,
    totp_secret     TEXT,                          -- NULL when 2FA is disabled
    totp_enabled    BOOLEAN NOT NULL DEFAULT FALSE,
    failed_attempts INT NOT NULL DEFAULT 0,
    locked_until    TIMESTAMPTZ,                   -- NULL when not locked
    last_login_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Sessions table
CREATE TABLE sessions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Migrations run automatically on startup via embedded SQL files — no manual setup required.

---

## Security Design

| Concern              | Implementation                                                     |
|----------------------|--------------------------------------------------------------------|
| Password storage     | bcrypt with default cost (min cost 10)                             |
| Session tokens       | Random UUIDs generated server-side, stored in PostgreSQL           |
| Session expiry       | Checked on every command, configurable timeout                     |
| Account lockout      | Configurable threshold and lockout duration                        |
| 2FA                  | TOTP (RFC 6238), compatible with Google Authenticator              |
| 2FA verification     | Code verified before secret is saved; code required to disable     |

---

## Tech Stack

| Layer       | Technology                                                                 |
|-------------|----------------------------------------------------------------------------|
| Language    | Go 1.24                                                                    |
| Database    | PostgreSQL 16                                                              |
| DB Driver   | [pgx v5](https://github.com/jackc/pgx)                                    |
| Migrations  | [golang-migrate](https://github.com/golang-migrate/migrate) (embedded SQL) |
| CLI         | [chzyer/readline](https://github.com/chzyer/readline) (history + tab-completion) |
| Terminal UI | [pterm](https://github.com/pterm/pterm)                                    |
| 2FA / TOTP  | [pquerna/otp](https://github.com/pquerna/otp)                              |
| QR Code     | [mdp/qrterminal](https://github.com/mdp/qrterminal)                        |
| Passwords   | [golang.org/x/crypto/bcrypt](https://pkg.go.dev/golang.org/x/crypto/bcrypt) |
| Containers  | Docker + Docker Compose                                                    |

---

## Stopping the App

To stop all containers:

```bash
docker compose down
```

To stop and also delete the database volume (all data):

```bash
docker compose down -v
```
