package grpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	pb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto/bot"
)

type Bot struct {
	connection *grpc.ClientConn
	grpcClient pb.BotServiceClient
	timeout    time.Duration
	log        *slog.Logger
}

func NewBot(url string, timeout time.Duration, log *slog.Logger) (*Bot, error) {
	connection, err := grpc.NewClient(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to bot grpc server at %s: %w", url, err)
	}

	return &Bot{
		connection: connection,
		grpcClient: pb.NewBotServiceClient(connection),
		timeout:    timeout,
		log:        log,
	}, nil
}

func (bot *Bot) Close() error {
	if bot.connection != nil {
		return bot.connection.Close()
	}
	return nil
}

func (bot *Bot) SendUpdate(ctx context.Context, update domain.LinkUpdate) error {
	bot.log.Debug("sending update to bot",
		"url", update.URL,
		"description", update.Description,
		"preview", update.Preview,
	)

	request := &pb.LinkUpdate{
		Id:          update.ID,
		Url:         update.URL,
		Author:      update.Author,
		Description: update.Description,
		Preview:     update.Preview,
		TgChatIds:   update.TgChatIDs,
	}

	ctx, cancel := context.WithTimeout(ctx, bot.timeout)
	defer cancel()

	_, err := bot.grpcClient.SendUpdate(ctx, request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("send update request timed out: %w", err)
		}
		return fmt.Errorf("failed to send update to bot via gRPC: %w", err)
	}

	bot.log.Debug("update sent to bot",
		"url", update.URL,
	)

	return nil
}
