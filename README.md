# Go GraphQL Starter

A production-ready Go GraphQL starter using [gqlgen](https://gqlgen.com/) and [sqlc](https://sqlc.dev/), with PostgreSQL, JWT auth, and Docker-based development.

## Architecture

```
cmd/main.go        → entry point (env, migrations, HTTP server)
graph/*.graphql    → schema definitions
graph/*.resolvers.go → resolver implementations
internal/store/    → sqlc-generated data access layer
internal/auth/     → JWT middleware
internal/service/  → business logic layer
database/migrations/ → golang-migrate SQL migrations
container/          → Docker Compose + Dockerfile
```

## Quick Start

### Prerequisites

- Go 1.26+
- Docker & Docker Compose
- [Task](https://taskfile.dev/) (task runner)

### Setup

```bash
# Install dependencies, regenerate code, install git hooks
task install

# Start everything (Postgres + app + Apollo Sandbox)
task dev
```

The app runs at `http://localhost:4000/graphql` with **Apollo Sandbox** — a full-featured GraphQL IDE. No account required.

> The Sandbox fetches the schema automatically — introspection queries bypass JWT auth in development mode.

## Development

### Environment Variables (Docker)

All env vars have sensible defaults in `container/docker-compose.yml`. Copy `.env.example` to `.env` if you need to override them.

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | `postgres://postgres:postgres@db:5432/app_db` | PostgreSQL connection string |
| `JWT_SECRET` | `dev-secret` | Secret for signing/validating JWT tokens |
| `PORT` | `4000` | Server port |
| `DEBUG` | `true` | Enables Apollo Sandbox + root redirect |

### Common Tasks

```bash
task install        # First-clone setup (deps, codegen, hooks)
task dev            # Docker Compose with live reload (DEBUG=true)
task build          # Build binary to build/app
task test           # Run tests with race detection
task test:cover     # Run tests with coverage report
task lint           # Run linters (golangci-lint)
task lint:fix       # Auto-fix lint issues
task gen            # Regenerate all: sqlc → gqlgen
task gen:sqlc       # Regenerate sqlc code only
task gen:gqlgen     # Regenerate gqlgen code only
task token          # Generate a JWT token for testing
task hooks:install  # Install pre-commit git hook (lints staged Go files)
task hooks:uninstall # Remove pre-commit git hook
```

### Pagination

The API supports two pagination modes via `PaginationInput.mode` (default: `CURSOR`):

**Cursor-based** — infinite scroll / "load more":
```graphql
# First page (no cursor needed)
{ notes(input: { pageSize: 10 }) { items { ... } pagination { next { ... on CursorPage { cursor } } } } }

# Next page (use cursor from response)
{ notes(input: { pageSize: 10, cursor: "<next-cursor>" }) { items { ... } } }

# Previous page (use previous cursor from response)
{ notes(input: { pageSize: 10, cursor: "<previous-cursor>" }) { items { ... } } }
```

Cursors are opaque and self-describing; pass the returned cursor back unchanged.

**Offset-based** — page selector / jump-to-page:
```graphql
{ notes(input: { mode: OFFSET, pageSize: 10, pageNumber: 0 }) { items { ... } } }
{ notes(input: { mode: OFFSET, pageSize: 10, pageNumber: 2 }) { items { ... } } }
```

Both modes support sorting and filtering:
```graphql
{ notes(input: {
    pageSize: 10,
    sort: [{ field: "title", order: ASC }],
    filter: { filters: [{ field: "title", operator: CONTAINS, value: "foo" }] }
  }) { items { ... } pagination { total } }
}
```

Page numbers are **0-based** (first page = 0). The `PaginationMetadata.next`/`previous` fields indicate available pages for either mode.

### Adding a New Feature

1. **Database**: Add a migration in `database/migrations/` and a query in `database/queries/`.
2. **Regenerate**: `task gen:sqlc` to update `internal/store/`.
3. **GraphQL Schema**: Add types/queries/mutations in `graph/*.graphql`.
4. **Regenerate**: `task gen:gqlgen` to update stubs.
5. **Implement**: Fill in the resolver in `graph/*.resolvers.go` using the service layer.

### Linting

The project uses [golangci-lint](https://golangci-lint.run/) pinned as a Go tool dependency. Linters enabled: `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`, `gosec`, and `gofmt`.

Install the pre-commit hook to lint staged files automatically on every commit:

```bash
task hooks:install
```

Bypass with `git commit --no-verify`.

### Authentication

All GraphQL requests require a `Authorization: Bearer <token>` header. Generate a token:

```bash
task token -- USERNAME=user@example.com
```

In Apollo Sandbox, go to the **Headers** tab and add:

```json
{
  "Authorization": "Bearer <your-token>"
}
```
