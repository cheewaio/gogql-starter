# CLAUDE.md — gogql-starter

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

### Common Tasks
- **First-clone setup**: `task install` (deps, codegen, git hooks)
- **Run application**: `task run`
- **Development mode (Docker + live reload)**: `task dev`
- **Build binary**: `task build` (outputs to `bin/app`)
- **Build Docker image**: `task image`
- **Run tests**: `task test` (with race detection)
- **Run tests with coverage**: `task test:cover`
- **Run linters**: `task lint` (golangci-lint: errcheck, govet, ineffassign, staticcheck, unused, gosec + gofmt)
- **Auto-fix lint issues**: `task lint:fix`
- **Install pre-commit hook**: `task hooks:install` (lints staged Go files before each commit)
- **Remove pre-commit hook**: `task hooks:uninstall`

### GraphQL Development
- The GraphQL schema files are in `graph/*.graphql`.
- Configuration for code generation is in `gqlgen.yml`.
- After modifying the schema, run `task generate` to update `graph/generated.go`, `graph/model/models_gen.go`, and resolver stubs.
- Development server uses **Apollo Sandbox** as the GraphQL IDE (served when `DEBUG=true`).
- **Introspection bypass**: The auth middleware at `internal/auth/middleware.go:27` detects `__schema` queries and allows them through without authentication, so the IDE can fetch the schema immediately.

### Code Generation
- **gqlgen** (`task gen:gqlgen`): Generates GraphQL server code from schema files.
- **sqlc** (`task gen:sqlc`): Generates Go code from SQL queries in `database/queries/`. Output goes to `internal/store/`.
- **Full regenerate** (`task gen`): Runs both sqlc and gqlgen sequentially.

### Linting
- Configuration is in `.golangci.yml` (v1 format, pinned via `go tool golangci-lint`).
- The pre-commit hook (`task hooks:install`) runs `golangci-lint` only on staged `.go` files.
- Bypass the hook with `git commit --no-verify`.

## Project Architecture

### High-Level Structure
- **Entry Point**: `cmd/main.go` initializes environment variables, runs database migrations, and starts the HTTP server.
- **GraphQL Layer**:
    - `graph/*.graphql`: GraphQL schema files (reside in `graph/`).
    - `graph/resolver.go`: Root resolver with injected `*store.Queries`.
    - `graph/*.resolvers.go`: Resolver implementations for each schema file.
    - `graph/generated.go` & `graph/model/`: Auto-generated code by `gqlgen`.
- **Data Layer**: `internal/store/` combines sqlc-generated CRUD (`note.sql.go`, `models.go`) with hand-written custom queries (`query.go`). The `NewDB()` connection factory lives here too.
- **Auth**: `internal/auth/` handles JWT token validation and user context. Introspection queries automatically bypass auth (see `isIntrospectionRequest` at `internal/auth/middleware.go:27`).
- **Service Layer**: `internal/service/` wraps business logic, keeping resolvers thin.
- **Database**:
    - Migrations are managed via `golang-migrate` and located in `database/migrations/`.
    - Migrations are applied automatically on application startup using the `DATABASE_URL`.
    - sqlc uses `database/schema.sql` with `database/.sqlc.yml` pointing to `database/queries/` for code generation.
- **Infrastructure**:
    - Docker-based development and deployment configured in `container/` (includes `docker-compose.yml` and `Dockerfile`).

### Environment Variables
- `DATABASE_URL`: Connection string for the PostgreSQL database (required for migrations).
- `JWT_SECRET`: Secret key for signing and validating JWT tokens.
- `PORT`: Port on which the server will listen (defaults to `4000`).
- `DEBUG`: When set to `true`, enables Apollo Sandbox IDE and root-to-graphql redirect.
