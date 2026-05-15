FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/dockeragent ./cmd/dockeragent

FROM alpine:3.21

RUN apk add --no-cache ca-certificates wget

COPY --from=builder /bin/dockeragent /bin/dockeragent

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:3000/health >/dev/null || exit 1

ENTRYPOINT ["/bin/dockeragent"]
