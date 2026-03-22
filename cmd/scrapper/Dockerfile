FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /bin/scrapper ./cmd/scrapper/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/application.conf .
COPY --from=builder /app/migrations/ ./migrations
COPY --from=builder /bin/scrapper /app/scrapper

EXPOSE 63342

ENTRYPOINT ["/app/scrapper"]