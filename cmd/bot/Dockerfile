FROM golang:1.25 AS builder

RUN apt update && apt install -y protobuf-compiler
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
ENV PATH="$PATH:$(go env GOPATH)/bin"

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/bot ./cmd/bot
COPY internal/bot ./internal/bot
COPY docs/scrapper.proto ./docs/
COPY docs/bot.proto ./docs/
COPY internal/validation ./internal/validation

RUN mkdir -p pkg/api/proto/scrapper

RUN protoc -I=docs \
    --go_out=pkg/api/proto/scrapper --go_opt=paths=source_relative \
    --go-grpc_out=pkg/api/proto/scrapper --go-grpc_opt=paths=source_relative \
    scrapper.proto

RUN mkdir -p pkg/api/proto/bot

RUN protoc -I=docs \
    --go_out=pkg/api/proto/bot --go_opt=paths=source_relative \
    --go-grpc_out=pkg/api/proto/bot --go-grpc_opt=paths=source_relative \
    bot.proto

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/bot ./cmd/bot/main.go

FROM alpine:latest

WORKDIR /app

COPY cmd/bot/bot.conf ./bot.conf
COPY --from=builder /bin/bot /app/bot

EXPOSE 63342

ENTRYPOINT ["/app/bot"]