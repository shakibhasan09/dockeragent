# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
go build ./cmd/dockeragent          # compile binary
go vet ./...                        # lint all packages
go test ./...                       # run all tests
go test ./internal/service          # run tests for a single package
go test -run TestCreateContainer ./internal/handler  # run a single test
API_KEY=<key> go run ./cmd/dockeragent  # run locally (requires Docker daemon)
```

Makefile targets: `make build` (to `./dist/`), `make run`, `make test`, `make vet`, `make clean`, `make docker-build`.

## Required Environment Variables

- `API_KEY` (required) — the server refuses to start without it
- `LISTEN_ADDR` (optional, default `:3000`)
- Docker client reads `DOCKER_HOST` and `DOCKER_API_VERSION` from env automatically

## Architecture

Single-binary Go HTTP API managing Docker containers via the Docker Engine API. Uses **Fiber v3** (`github.com/gofiber/fiber/v3`) and **moby/moby client** (`github.com/moby/moby/client` v0.2.2).

### Request Flow

```
Request → recover → requestid → requestLogger → limiter → [keyauth on /api/v1/*] → handler → service → Docker daemon
```

`/health` bypasses auth. All `/api/v1/*` routes require `X-API-Key` header.

### Layer Responsibilities

- **`handler`** — HTTP concerns only: parse input (`c.Bind().JSON`/`.Query`), validate, call service, return JSON. Errors returned via `fiber.NewError()` and handled by the global `ErrorHandler` in `router.go`.
- **`service`** — Translates between `model` DTOs and Docker API types. Builds `container.Config`, `container.HostConfig`, `network.NetworkingConfig`. The Docker `*client.Client` is injected via interface.
- **`model`** — Pure data types. `request.go` defines API input shapes, `response.go` defines output shapes. These are the API contract.
- **`router`** — Middleware chain assembly and route registration. Defines the global `ErrorHandler` that produces consistent `model.ErrorResponse` JSON.
- **`middleware`** — Fiber keyauth configured with `extractors.FromHeader("X-API-Key")`.

### Dependency Injection Pattern

All layers use interfaces for testability:
- Handlers depend on `ContainerServicer` / `FileServicer` interfaces (not concrete service types)
- `FileHandler` depends on a `SymlinkEvaluator` interface (wrapping `filepath.EvalSymlinks`)
- `ContainerService` depends on a `DockerClient` interface (not `*client.Client` directly)
- `FileService` depends on a `FileSystem` interface (wrapping `os` operations)

### Key Docker API Patterns

The moby/moby API v1.53.0 uses structured types that differ from older versions:
- `network.Port` is a struct (not a string) — use `network.ParsePort("80/tcp")` to create
- `network.PortBinding.HostIP` is `netip.Addr` — parse from string with `netip.ParseAddr()`
- Client methods return result structs (e.g., `ContainerCreateResult`, `ContainerListResult` with `.Items`) — not bare values
- `ContainerLogs` returns a `ContainerLogsResult` implementing `io.ReadCloser`

### Error Handling

Handler errors flow through `classifyDockerError()` in `handler/container.go` (and `classifyFileError()` in `handler/file.go`). `classifyDockerError` first attempts type-based classification using `containerd/errdefs` (`IsNotFound`, `IsConflict`, `IsAlreadyExists`, etc.), then falls back to string matching for errors that don't implement errdefs interfaces. The global `ErrorHandler` in `router.go` converts `*fiber.Error` to the standard `model.ErrorResponse` JSON envelope.

### File Operations

`POST /api/v1/files` writes files to the host filesystem. The handler prefixes all paths with `/host` (for Docker host mount access) and validates that paths are absolute, contain no `..` traversal, and don't escape `/host` via symlinks (using `filepath.EvalSymlinks` on the parent directory). File content is capped at 10 MB. The `FileService` uses an `OSFileSystem` wrapper that auto-creates parent directories. Symlink resolution is abstracted via `SymlinkEvaluator` interface for testability.

### Log Streaming

`GET /api/v1/containers/:id/logs` supports two modes:
- `follow=false` (default): reads all logs, returns JSON `{"lines": [...]}`
- `follow=true`: streams via SSE using `c.SendStreamWriter()` (Fiber v3's streaming API)

## Testing Conventions

Tests use table-driven patterns with mock structs that have function fields for behavior injection:

```go
type mockDockerClient struct {
    ContainerCreateFn func(...) (...)
}
```

Handler tests use `newTestApp()` and `doRequest()` helpers to set up a Fiber app and execute requests. Service tests mock the `DockerClient` and `FileSystem` interfaces directly.
