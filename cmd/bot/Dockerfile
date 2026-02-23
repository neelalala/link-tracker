FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /bin/bot ./cmd/bot/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/application.conf .
COPY --from=builder /bin/bot /app/bot

ENTRYPOINT ["/app/bot"]