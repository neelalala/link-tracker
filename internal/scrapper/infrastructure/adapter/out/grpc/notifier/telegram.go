package notifier

import (
	"context"
	"fmt"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	pb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto/bot"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

const defaultTimeout = 10 * time.Second

type Bot struct {
	conn       *grpc.ClientConn
	grpcClient pb.BotServiceClient
}

func NewBot(url string) (*Bot, error) {
	conn, err := grpc.NewClient(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to bot grpc server at %s: %w", url, err)
	}

	return &Bot{
		conn:       conn,
		grpcClient: pb.NewBotServiceClient(conn),
	}, nil
}

func (bot *Bot) Close() error {
	if bot.conn != nil {
		return bot.conn.Close()
	}
	return nil
}

func (bot *Bot) SendUpdate(ctx context.Context, update domain.LinkUpdate) error {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	req := &pb.LinkUpdate{
		Id:          update.ID,
		Url:         update.URL,
		Description: update.Description,
		TgChatIds:   update.TgChatIDs,
	}

	_, err := bot.grpcClient.SendUpdate(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send update to bot via gRPC: %w", err)
	}

	return nil
}
