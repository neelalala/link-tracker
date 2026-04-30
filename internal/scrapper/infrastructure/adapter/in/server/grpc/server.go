package grpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	pb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto/scrapper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type SubscriptionService interface {
	RegisterChat(ctx context.Context, chatID int64) error
	DeleteChat(ctx context.Context, chatID int64) error
	GetTrackedLinks(ctx context.Context, chatID int64) ([]domain.TrackedLink, error)
	AddLink(ctx context.Context, chatID int64, url string, tags []string) (domain.TrackedLink, error)
	RemoveLink(ctx context.Context, chatID int64, url string) (domain.TrackedLink, error)
}

type Server struct {
	pb.UnimplementedScrapperServiceServer
	port       uint16
	grpcServer *grpc.Server
	service    SubscriptionService
	log        *slog.Logger
}

func NewServer(port uint16, service SubscriptionService, logger *slog.Logger) *Server {
	grpcServer := grpc.NewServer()

	server := &Server{
		port:       port,
		grpcServer: grpcServer,
		service:    service,
		log:        logger,
	}

	pb.RegisterScrapperServiceServer(grpcServer, server)
	return server
}

func (server *Server) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", server.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", server.port, err)
	}

	server.log.Info("gRPC server is running", slog.Int("port", int(server.port)))
	if err := server.grpcServer.Serve(listener); err != nil {
		return err
	}

	return nil
}

func (server *Server) Stop(ctx context.Context) error {
	server.log.Info("Shutting down gRPC server...")

	stopped := make(chan struct{})
	go func() {
		server.grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-ctx.Done():
		server.grpcServer.Stop()
		return ctx.Err()
	case <-stopped:
		return nil
	}
}

func (server *Server) RegisterChat(ctx context.Context, request *pb.RegisterChatRequest) (*emptypb.Empty, error) {
	err := server.service.RegisterChat(ctx, request.GetId())
	if err != nil {
		if errors.Is(err, domain.ErrChatAlreadyRegistered) {
			return nil, status.Errorf(codes.AlreadyExists, "chat with id %d already registered", request.GetId())
		}

		server.log.Error(
			"failed to register chat",
			slog.Int64("chat_id", request.GetId()),
			slog.String("error", err.Error()),
			slog.String("method", "RegisterChat"),
		)
		return nil, status.Errorf(codes.Internal, "failed to register chat: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (server *Server) DeleteChat(ctx context.Context, request *pb.DeleteChatRequest) (*emptypb.Empty, error) {
	err := server.service.DeleteChat(ctx, request.GetId())
	if err != nil {
		if errors.Is(err, domain.ErrChatNotRegistered) {
			return nil, status.Errorf(codes.NotFound, "chat with id %d not registered", request.GetId())
		}

		server.log.Error(
			"failed to delete chat",
			slog.Int64("chat_id", request.GetId()),
			slog.String("error", err.Error()),
			slog.String("method", "DeleteChat"),
		)
		return nil, status.Errorf(codes.Internal, "failed to delete chat: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (server *Server) GetLinks(ctx context.Context, request *pb.GetLinksRequest) (*pb.ListLinksResponse, error) {
	chatId := request.GetTgChatId()
	links, err := server.service.GetTrackedLinks(ctx, chatId)
	if err != nil {
		if errors.Is(err, domain.ErrChatNotRegistered) {
			return nil, status.Errorf(codes.NotFound, "chat with id %d not registered", chatId)
		}

		server.log.Error(
			"failed to get tracked links",
			slog.Int64("chat_id", chatId),
			slog.String("error", err.Error()),
			slog.String("method", "GetLinks"),
		)
		return nil, status.Errorf(codes.Internal, "failed to get links: %v", err)
	}

	var pbLinks []*pb.LinkResponse
	for _, link := range links {
		pbLinks = append(pbLinks, &pb.LinkResponse{
			Id:   link.ID,
			Url:  link.URL,
			Tags: link.Tags,
		})
	}

	return &pb.ListLinksResponse{
		Links: pbLinks,
		Size:  int32(len(pbLinks)),
	}, nil
}

func (server *Server) AddLink(ctx context.Context, request *pb.AddLinkRequest) (*pb.LinkResponse, error) {
	chatId := request.GetTgChatId()
	linkUrl := request.GetLink()
	tags := request.GetTags()

	link, err := server.service.AddLink(ctx, chatId, linkUrl, tags)
	if err != nil {
		if errors.Is(err, domain.ErrChatNotRegistered) {
			return nil, status.Errorf(codes.NotFound, "chat %d not registered yet", chatId)
		}
		if errors.Is(err, domain.ErrAlreadySubscribed) {
			return nil, status.Errorf(codes.AlreadyExists, "link %s already tracked", linkUrl)
		}
		if errors.Is(err, domain.ErrURLNotSupported) {
			return nil, status.Errorf(codes.Unimplemented, "link %s not supported", linkUrl)
		}

		server.log.Error(
			"failed to add link",
			slog.Int64("chat_id", chatId),
			slog.String("link", linkUrl),
			slog.String("error", err.Error()),
			slog.String("method", "AddLink"),
		)
		return nil, status.Errorf(codes.Internal, "failed to add link: %v", err)
	}

	return &pb.LinkResponse{
		Id:   link.ID,
		Url:  link.URL,
		Tags: link.Tags,
	}, nil
}

func (server *Server) RemoveLink(ctx context.Context, req *pb.RemoveLinkRequest) (*pb.LinkResponse, error) {
	chatId := req.GetTgChatId()
	linkUrl := req.GetLink()

	link, err := server.service.RemoveLink(ctx, chatId, linkUrl)
	if err != nil {
		if errors.Is(err, domain.ErrChatNotRegistered) {
			return nil, status.Errorf(codes.NotFound, "chat %d not registered", chatId)
		}
		if errors.Is(err, domain.ErrNotSubscribed) || errors.Is(err, domain.ErrLinkNotFound) {
			return nil, status.Errorf(codes.NotFound, "link %s not tracked in chat %d", linkUrl, chatId)
		}

		server.log.Error(
			"failed to remove link",
			slog.Int64("chat_id", chatId),
			slog.String("link", linkUrl),
			slog.String("error", err.Error()),
			slog.String("method", "RemoveLink"),
		)
		return nil, status.Errorf(codes.Internal, "failed to remove link: %v", err)
	}

	return &pb.LinkResponse{
		Id:   link.ID,
		Url:  link.URL,
		Tags: link.Tags,
	}, nil
}
