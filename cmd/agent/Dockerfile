FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/agent ./cmd/agent
COPY internal/agent ./internal/agent
COPY internal/validation ./internal/validation

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/agent ./cmd/agent/main.go

FROM alpine:latest

WORKDIR /app

COPY cmd/agent/config.yaml ./config.yaml
COPY --from=builder /bin/agent /app/agent

EXPOSE 63344

ENTRYPOINT ["/app/agent"]