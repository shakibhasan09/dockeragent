# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
go build ./cmd/dockeragent          # compile binary
go vet ./...                        # lint all packages
API_KEY=<key> go run ./cmd/dockeragent  # run locally (requires Docker daemon)
```

No tests exist yet. When adding tests, use `go test ./...` to run all, or `go test ./internal/service` for a single package.

## Required Environment Variables

- `API_KEY` (required) — the server refuses to start without it
- `LISTEN_ADDR` (optional, default `:3000`)
- Docker client reads `DOCKER_HOST` and `DOCKER_API_VERSION` from env automatically

## Architecture

This is a single-binary Go HTTP API that manages Docker containers via the Docker Engine API. It uses **Fiber v3** (`github.com/gofiber/fiber/v3`) as the web framework and **moby/moby client** (`github.com/moby/moby/client` v0.2.2) as the Docker client.

### Request Flow

```
Request → recover → requestid → requestLogger → [keyauth on /api/v1/*] → handler → service → Docker daemon
```

`/health` bypasses auth. All `/api/v1/*` routes require `X-API-Key` header.

### Layer Responsibilities

- **`handler`** — HTTP concerns only: parse input (`c.Bind().JSON`/`.Query`), validate, call service, return JSON. Errors are returned via `fiber.NewError()` and handled by the global `ErrorHandler` in `router.go`.
- **`service`** — Translates between `model` DTOs and Docker API types. This is where `container.Config`, `container.HostConfig`, `network.NetworkingConfig` are built. The Docker `*client.Client` is injected here.
- **`model`** — Pure data types. `request.go` defines API input shapes, `response.go` defines output shapes. These are the API contract.
- **`router`** — Middleware chain assembly and route registration. Also defines the global `ErrorHandler` that produces consistent `model.ErrorResponse` JSON.
- **`middleware`** — Fiber keyauth configured with `extractors.FromHeader("X-API-Key")`.

### Key Docker API Patterns

The moby/moby API v1.53.0 uses structured types that differ from older versions:
- `network.Port` is a struct (not a string) — use `network.ParsePort("80/tcp")` to create
- `network.PortBinding.HostIP` is `netip.Addr` — parse from string with `netip.ParseAddr()`
- Client methods return result structs (e.g., `ContainerCreateResult`, `ContainerListResult` with `.Items`) — not bare values
- `ContainerLogs` returns a `ContainerLogsResult` implementing `io.ReadCloser`

### Error Handling

Handler errors flow through `classifyDockerError()` in `handler/container.go`, which maps Docker error messages (string matching on "not found", "conflict", etc.) to HTTP status codes. The global `ErrorHandler` in `router.go` converts `*fiber.Error` to the standard `model.ErrorResponse` JSON envelope.

### Log Streaming

`GET /api/v1/containers/:id/logs` supports two modes:
- `follow=false` (default): reads all logs, returns JSON `{"lines": [...]}`
- `follow=true`: streams via SSE using `c.SendStreamWriter()` (Fiber v3's streaming API)
