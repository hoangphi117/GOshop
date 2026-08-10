# GOshop

E-commerce backend API built with Go, Gin, and PostgreSQL.

## Architecture

```
cmd/api/            - Application entry point
internal/
  ├── app/          - Application bootstrap & module registration
  ├── config/       - Configuration management
  ├── db/           - Database connection, migrations, SQLC generated code
  ├── dto/          - Data transfer objects (versioned)
  ├── handler/      - HTTP handlers (versioned)
  ├── middleware/   - HTTP middleware pipeline
  ├── repository/   - Data access layer
  ├── routes/       - Route registration (versioned)
  ├── service/      - Business logic (versioned)
  ├── utils/        - Helper functions
  └── validation/   - Custom validation rules
pkg/logger/         - Logging infrastructure
```

## Tech Stack

- **Framework**: Gin Web Framework
- **Database**: PostgreSQL 18 (pgx/v5 driver)
- **Query Builder**: SQLC
- **Migration**: golang-migrate
- **Logging**: zerolog + lumberjack
- **Validation**: go-playground/validator

## Features

- Clean Architecture (handler → service → repository)
- Versioned API (v1)
- Middleware pipeline: rate limiting, logging, recovery, API key auth
- SQLC type-safe SQL queries
- Structured logging with rotation
- Custom validation rules
- Graceful shutdown

## Prerequisites

- Go 1.26.5+
- PostgreSQL 18+
- Docker & Docker Compose (optional)
- golang-migrate CLI

## Quick Start

### 1. Clone & Install

```bash
git clone <repository-url>
cd goshop
go mod download
```

### 2. Setup Database

```bash
# Start PostgreSQL with Docker
docker-compose up -d

# Run migrations
make migrate-up
```

### 3. Configure Environment

```bash
cp .env.example .env
# Edit .env with your settings
```

### 4. Run Server

```bash
make server
# Server starts at http://localhost:8080
```

## API Endpoints

### Users (v1)

All endpoints require `x-Api-Key` header.

| Method | Endpoint           | Description       |
|--------|--------------------|-------------------|
| GET    | /api/v1/users      | List all users    |
| POST   | /api/v1/users      | Create user       |
| GET    | /api/v1/users/:uuid| Get user by UUID  |
| PUT    | /api/v1/users/:uuid| Update user       |
| DELETE | /api/v1/users/:uuid| Delete user       |

## Development

### Database Commands

```bash
# Create new migration
make migrate-create NAME=migration_name

# Run all pending migrations
make migrate-up

# Rollback last migration
make migrate-down

# Generate SQLC code
make sqlc
```

### Project Structure Conventions

- **Versioning**: API routes, handlers, services, DTOs are versioned (v1/, v2/)
- **Interfaces**: Defined in `interfaces.go` within each layer
- **Modules**: Each feature is a self-contained module (app/user_module.go)
- **Error Handling**: Centralized error types in utils/response.go

## Configuration

Environment variables (see `.env.example`):

- `API_KEY`: API authentication key
- `SERVER_PORT`: HTTP server port
- `APP_ENV`: Environment (development/production)
- `DB_*`: Database connection settings
- `RATE_LIMITER_*`: Rate limiting configuration

## Logging

Logs are written to `internal/logs/`:
- `http.log`: HTTP request/response logs
- `recovery.log`: Panic recovery logs
- `rate_limiter.log`: Rate limit violations

In development mode, logs pretty-print to stdout.

## License

MIT
