# Docker Agent

A lightweight Go HTTP API for managing Docker containers via the Docker Engine API. Provides a clean REST interface with API key authentication, structured JSON responses, and real-time log streaming.

## Features

- Full container lifecycle management (create, list, inspect, stop, remove)
- Host file writing with configurable permissions
- Real-time log streaming via Server-Sent Events (SSE)
- API key authentication on all protected routes
- Rate limiting (100 requests/minute per IP)
- Request body size limit (10 MB)
- Resource limits (CPU, memory) per container
- Port mappings with range validation (1-65535), volume mounts, network configuration
- Signal validation (POSIX signal names) for container stop
- Symlink traversal protection on file write paths
- Restart policies
- Health check endpoint with Docker daemon status
- Structured JSON logging with request IDs
- Graceful shutdown

## Requirements

- Go 1.25+
- Docker daemon running and accessible

## Quick Start

```bash
# Build
go build ./cmd/dockeragent

# Run
API_KEY=your-secret-key go run ./cmd/dockeragent
```

Or use the Makefile:

```bash
make build   # compile to ./dist/dockeragent
make run     # go run
make vet     # lint
make clean   # remove build artifacts
```

### Docker

```bash
docker build -t dockeragent .
docker run -d \
  -e API_KEY=your-secret-key \
  -p 3000:3000 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /:/host \
  dockeragent
```

## Configuration

| Variable             | Required | Default | Description                         |
| -------------------- | -------- | ------- | ----------------------------------- |
| `API_KEY`            | Yes      | -       | API key for authenticating requests |
| `LISTEN_ADDR`        | No       | `:3000` | Address the server listens on       |
| `DOCKER_HOST`        | No       | -       | Docker daemon socket/host           |
| `DOCKER_API_VERSION` | No       | -       | Docker API version override         |

## API

All `/api/v1/*` routes require the `X-API-Key` header. Requests are rate-limited to 100 per minute per IP. Request bodies are limited to 10 MB.

### Health Check

```
GET /health
```

Returns server and Docker daemon status. No authentication required.

```json
{
  "status": "ok",
  "docker": "connected",
  "timestamp": "2026-02-21T10:00:00Z"
}
```

### Create Container

```
POST /api/v1/containers
```

```json
{
  "image": "nginx:latest",
  "name": "web-server",
  "cmd": ["nginx", "-g", "daemon off;"],
  "env": ["NODE_ENV=production"],
  "ports": [
    {
      "container_port": "80",
      "host_port": "8080",
      "protocol": "tcp",
      "host_ip": "0.0.0.0"
    }
  ],
  "volumes": [
    {
      "source": "/host/data",
      "target": "/container/data",
      "read_only": false
    }
  ],
  "networks": ["my-network"],
  "restart_policy": {
    "name": "always"
  },
  "resources": {
    "cpus": 1.5,
    "memory_mb": 512
  }
}
```

Response (`201 Created`):

```json
{
  "id": "abc123def456...",
  "warnings": []
}
```

### List Containers

```
GET /api/v1/containers?all=true
```

| Query Param | Default | Description                |
| ----------- | ------- | -------------------------- |
| `all`       | `false` | Include stopped containers |

### Inspect Container

```
GET /api/v1/containers/:id
```

Returns the full Docker inspect response for the container.

### Stop Container

```
POST /api/v1/containers/:id/stop
```

```json
{
  "timeout": 10,
  "signal": "SIGTERM"
}
```

The `signal` field must be a valid POSIX signal name (e.g., `SIGTERM`, `SIGKILL`, `SIGINT`). The body is optional — sending an empty request will use Docker's defaults.

### Remove Container

```
DELETE /api/v1/containers/:id?force=true&v=true
```

| Query Param | Default | Description                         |
| ----------- | ------- | ----------------------------------- |
| `force`     | `false` | Force remove a running container    |
| `v`         | `false` | Remove associated anonymous volumes |

### Get Container Logs

```
GET /api/v1/containers/:id/logs?tail=100&follow=false
```

| Query Param  | Default | Description                      |
| ------------ | ------- | -------------------------------- |
| `follow`     | `false` | Stream logs in real-time via SSE |
| `tail`       | `all`   | Number of lines from the end     |
| `since`      | -       | Show logs since timestamp        |
| `until`      | -       | Show logs until timestamp        |
| `timestamps` | `false` | Include timestamps in log lines  |

Non-follow response:

```json
{
  "lines": [
    "2026-02-21 10:00:00 Server started",
    "2026-02-21 10:00:01 Listening on port 80"
  ]
}
```

Follow mode streams as `text/event-stream` (SSE).

### Write File

```
POST /api/v1/files
```

Writes a file to the host filesystem. Parent directories are created automatically.

```json
{
  "path": "/etc/nginx/nginx.conf",
  "content": "server { listen 80; }",
  "permission": "0644"
}
```

| Field        | Required | Default | Description                              |
| ------------ | -------- | ------- | ---------------------------------------- |
| `path`       | Yes      | -       | Absolute file path on the host           |
| `content`    | No       | `""`    | File content as a string (max 10 MB)     |
| `permission` | No       | `0644`  | Unix file permission (octal string)      |

Paths are validated against directory traversal (`..`) and symlink escape attacks. All paths are resolved relative to the `/host` mount inside the container.

Response (`201 Created`):

```json
{
  "path": "/etc/nginx/nginx.conf",
  "size": 21,
  "message": "file written successfully"
}
```

## Error Responses

All errors return a consistent JSON envelope:

```json
{
  "error": "Not Found",
  "message": "no such container: abc123",
  "status": 404
}
```

## Architecture

```
Request -> recover -> requestid -> requestLogger -> limiter -> [keyauth] -> handler -> service -> Docker daemon
```

| Layer        | Responsibility                                            |
| ------------ | --------------------------------------------------------- |
| `handler`    | HTTP concerns: parse input, validate, return JSON         |
| `service`    | Translate between API models and Docker API types         |
| `model`      | Pure data types for API request/response contracts        |
| `router`     | Middleware chain, rate limiting, and route registration    |
| `middleware` | API key authentication via `X-API-Key` header             |

### Security

- **Authentication**: API key via `X-API-Key` header on all `/api/v1/*` routes
- **Rate limiting**: 100 requests/minute per IP address (returns `429 Too Many Requests`)
- **Body size limit**: 10 MB maximum request body
- **Input validation**: Port ranges (1-65535), POSIX signal names, octal file permissions
- **Path traversal protection**: `..` components rejected, symlinks resolved and verified to stay under `/host`
- **Error classification**: Docker errors mapped via `containerd/errdefs` type assertions with string-matching fallback

## License

MIT
