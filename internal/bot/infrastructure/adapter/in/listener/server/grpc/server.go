package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	pb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto/bot"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Server struct {
	pb.UnimplementedBotServiceServer
	port          uint16
	grpcServer    *grpc.Server
	updateHandler domain.LinkUpdateHandler
	logger        *slog.Logger
}

func NewServer(port uint16, updateHandler domain.LinkUpdateHandler, logger *slog.Logger) *Server {
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

func (server *Server) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", server.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", server.port, err)
	}

	server.logger.Info("gRPC server is running", slog.Int("port", int(server.port)))
	if err := server.grpcServer.Serve(listener); err != nil {
		return err
	}
	return nil
}

func (server *Server) Stop(ctx context.Context) error {
	server.logger.Info("Shutting down gRPC server...")

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

func (server *Server) SendUpdate(ctx context.Context, request *pb.LinkUpdate) (*emptypb.Empty, error) {
	linkUpdate := domain.LinkUpdate{
		ID:          request.GetId(),
		URL:         request.GetUrl(),
		Description: request.GetDescription(),
		Preview:     request.GetPreview(),
		TgChatIDs:   request.GetTgChatIds(),
	}

	err := server.updateHandler.HandleUpdate(ctx, linkUpdate)
	if err != nil {
		server.logger.Error(
			"failed to handle link update",
			slog.Int64("link_id", request.GetId()),
			slog.String("link_url", request.GetUrl()),
			slog.String("error", err.Error()),
			slog.String("method", "SendUpdate"),
		)

		return nil, status.Errorf(codes.Internal, "failed to handle update for link %s: %v", request.GetUrl(), err)
	}

	return &emptypb.Empty{}, nil
}
