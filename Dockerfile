FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/dockeragent ./cmd/dockeragent

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

COPY --from=builder /bin/dockeragent /bin/dockeragent

EXPOSE 3000

ENTRYPOINT ["/bin/dockeragent"]
