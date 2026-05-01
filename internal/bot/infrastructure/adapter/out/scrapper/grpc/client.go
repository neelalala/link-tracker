package grpc

import (
	"context"
	"fmt"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	pb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto/scrapper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type Client struct {
	connection *grpc.ClientConn
	grpcClient pb.ScrapperServiceClient
}

func NewClient(url string) (*Client, error) {
	connection, err := grpc.NewClient(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &Client{
		connection: connection,
		grpcClient: pb.NewScrapperServiceClient(connection),
	}, nil
}

func (client *Client) Close() error {
	if client.connection != nil {
		return client.connection.Close()
	}
	return nil
}

func (client *Client) RegisterChat(ctx context.Context, chatId int64) error {
	_, err := client.grpcClient.RegisterChat(ctx, &pb.RegisterChatRequest{Id: chatId})
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			return domain.ErrChatAlreadyRegistered
		}
		return fmt.Errorf("scrapper api returned unexpected error: %w", err)
	}

	return nil
}

func (client *Client) DeleteChat(ctx context.Context, chatId int64) error {
	_, err := client.grpcClient.DeleteChat(ctx, &pb.DeleteChatRequest{Id: chatId})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return domain.ErrChatNotRegistered
		}
		return fmt.Errorf("scrapper api returned unexpected error: %w", err)
	}

	return nil
}

func (client *Client) GetTrackedLinks(ctx context.Context, chatId int64) ([]domain.TrackedLink, error) {
	resp, err := client.grpcClient.GetLinks(ctx, &pb.GetLinksRequest{TgChatId: chatId})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, domain.ErrChatNotRegistered
		}
		return nil, fmt.Errorf("scrapper api returned unexpected error: %w", err)
	}

	var links []domain.TrackedLink
	for _, link := range resp.Links {
		links = append(links, domain.TrackedLink{
			ID:   link.Id,
			URL:  link.Url,
			Tags: link.Tags,
		})
	}

	return links, nil
}

func (client *Client) AddLink(ctx context.Context, chatId int64, url string, tags []string) (domain.TrackedLink, error) {
	request := &pb.AddLinkRequest{
		TgChatId: chatId,
		Link:     url,
		Tags:     tags,
	}

	resp, err := client.grpcClient.AddLink(ctx, request)
	if err != nil {
		code := status.Code(err)
		switch code {
		case codes.NotFound:
			return domain.TrackedLink{}, domain.ErrChatNotRegistered
		case codes.AlreadyExists:
			return domain.TrackedLink{}, domain.ErrAlreadySubscribed
		case codes.Unimplemented:
			return domain.TrackedLink{}, domain.ErrURLNotSupported
		default:
			return domain.TrackedLink{}, fmt.Errorf("scrapper api returned unexpected error: %w", err)
		}
	}

	return domain.TrackedLink{
		ID:   resp.Id,
		URL:  resp.Url,
		Tags: resp.Tags,
	}, nil
}

func (client *Client) RemoveLink(ctx context.Context, chatId int64, url string) (domain.TrackedLink, error) {
	req := &pb.RemoveLinkRequest{
		TgChatId: chatId,
		Link:     url,
	}

	resp, err := client.grpcClient.RemoveLink(ctx, req)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return domain.TrackedLink{}, domain.ErrChatNotRegisteredOrLinkNotFound
		}
		return domain.TrackedLink{}, fmt.Errorf("scrapper api returned unexpected error: %w", err)
	}

	return domain.TrackedLink{
		ID:   resp.Id,
		URL:  resp.Url,
		Tags: resp.Tags,
	}, nil
}
