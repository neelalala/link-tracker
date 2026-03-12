package grpc

import (
	"context"
	"fmt"
	scrapperdomain "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	pb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto/bot"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"log/slog"
	"net"
)

type LinkUpdateHandler interface {
	HandleUpdate(ctx context.Context, update scrapperdomain.LinkUpdate) error
}

type Server struct {
	pb.UnimplementedBotServiceServer
	port          uint16
	grpcServer    *grpc.Server
	updateHandler LinkUpdateHandler
	logger        *slog.Logger
}

func NewServer(port uint16, updateHandler LinkUpdateHandler, logger *slog.Logger) *Server {
	grpcServer := grpc.NewServer()

	srv := &Server{
		port:          port,
		grpcServer:    grpcServer,
		updateHandler: updateHandler,
		logger:        logger,
	}

	pb.RegisterBotServiceServer(grpcServer, srv)
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

func (s *Server) SendUpdate(ctx context.Context, req *pb.LinkUpdate) (*emptypb.Empty, error) {
	linkUpdate := scrapperdomain.LinkUpdate{
		ID:          req.GetId(),
		URL:         req.GetUrl(),
		Description: req.GetDescription(),
		TgChatIDs:   req.GetTgChatIds(),
	}

	err := s.updateHandler.HandleUpdate(ctx, linkUpdate)
	if err != nil {
		s.logger.Error(
			"failed to handle link update",
			slog.Int64("link_id", req.GetId()),
			slog.String("link_url", req.GetUrl()),
			slog.String("error", err.Error()),
			slog.String("method", "SendUpdate"),
		)

		return nil, status.Errorf(codes.Internal, "failed to handle update for link %s: %v", req.GetUrl(), err)
	}

	return &emptypb.Empty{}, nil
}
