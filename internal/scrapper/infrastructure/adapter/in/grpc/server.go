package grpc

import (
	"context"
	"errors"
	"fmt"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	pb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto/scrapper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"log/slog"
	"net"
)

type Server struct {
	pb.UnimplementedScrapperServiceServer
	port       uint16
	grpcServer *grpc.Server
	service    *application.SubscriptionService
	logger     *slog.Logger
}

func NewServer(port uint16, service *application.SubscriptionService, logger *slog.Logger) *Server {
	grpcServer := grpc.NewServer()

	srv := &Server{
		port:       port,
		grpcServer: grpcServer,
		service:    service,
		logger:     logger,
	}

	pb.RegisterScrapperServiceServer(grpcServer, srv)
	return srv
}

func (s *Server) Start(ctx context.Context) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", s.port, err)
	}

	errCh := make(chan error, 1)

	go func() {
		s.logger.Info("gRPC server is running", slog.Int("port", int(s.port)))
		if err := s.grpcServer.Serve(lis); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("grpc server failed: %w", err)

	case <-ctx.Done():
		s.logger.Info("shutting down gRPC server gracefully...")
		s.grpcServer.GracefulStop()

		return nil
	}
}

func (s *Server) RegisterChat(ctx context.Context, req *pb.RegisterChatRequest) (*emptypb.Empty, error) {
	err := s.service.RegisterChat(ctx, req.GetId())
	if err != nil {
		if errors.Is(err, domain.ErrChatAlreadyRegistered) {
			return nil, status.Errorf(codes.AlreadyExists, "chat with id %d already registered", req.GetId())
		}

		s.logger.Error(
			"failed to register chat",
			slog.Int64("chat_id", req.GetId()),
			slog.String("error", err.Error()),
			slog.String("method", "RegisterChat"),
		)
		return nil, status.Errorf(codes.Internal, "failed to register chat: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) DeleteChat(ctx context.Context, req *pb.DeleteChatRequest) (*emptypb.Empty, error) {
	err := s.service.DeleteChat(ctx, req.GetId())
	if err != nil {
		if errors.Is(err, domain.ErrChatNotRegistered) {
			return nil, status.Errorf(codes.NotFound, "chat with id %d not registered", req.GetId())
		}

		s.logger.Error(
			"failed to delete chat",
			slog.Int64("chat_id", req.GetId()),
			slog.String("error", err.Error()),
			slog.String("method", "DeleteChat"),
		)
		return nil, status.Errorf(codes.Internal, "failed to delete chat: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) GetLinks(ctx context.Context, req *pb.GetLinksRequest) (*pb.ListLinksResponse, error) {
	chatId := req.GetTgChatId()
	links, err := s.service.GetTrackedLinks(ctx, chatId)
	if err != nil {
		if errors.Is(err, domain.ErrChatNotRegistered) {
			return nil, status.Errorf(codes.NotFound, "chat with id %d not registered", chatId)
		}

		s.logger.Error(
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

func (s *Server) AddLink(ctx context.Context, req *pb.AddLinkRequest) (*pb.LinkResponse, error) {
	chatId := req.GetTgChatId()
	linkUrl := req.GetLink()
	tags := req.GetTags()

	link, err := s.service.AddLink(ctx, chatId, linkUrl, tags)
	if err != nil {
		if errors.Is(err, domain.ErrChatNotRegistered) {
			return nil, status.Errorf(codes.NotFound, "chat %d not registered yet", chatId)
		}
		if errors.Is(err, domain.ErrAlreadySubscribed) {
			return nil, status.Errorf(codes.AlreadyExists, "link %s already tracked", linkUrl)
		}
		if errors.Is(err, application.ErrUrlNotSupported) {
			return nil, status.Errorf(codes.Unimplemented, "link %s not supported", linkUrl)
		}

		s.logger.Error(
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

func (s *Server) RemoveLink(ctx context.Context, req *pb.RemoveLinkRequest) (*pb.LinkResponse, error) {
	chatId := req.GetTgChatId()
	linkUrl := req.GetLink()

	link, err := s.service.RemoveLink(ctx, chatId, linkUrl)
	if err != nil {
		if errors.Is(err, domain.ErrChatNotRegistered) {
			return nil, status.Errorf(codes.NotFound, "chat %d not registered", chatId)
		}
		if errors.Is(err, domain.ErrNotSubscribed) {
			return nil, status.Errorf(codes.NotFound, "link %s not tracked in chat %d", linkUrl, chatId)
		}

		s.logger.Error(
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
