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

func (server *Server) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", server.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", server.port, err)
	}

	errCh := make(chan error, 1)

	go func() {
		server.logger.Info("gRPC server is running", slog.Int("port", int(server.port)))
		if err := server.grpcServer.Serve(listener); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("grpc server failed: %w", err)

	case <-ctx.Done():
		server.logger.Info("shutting down gRPC server gracefully...")
		server.grpcServer.GracefulStop()

		return nil
	}
}

func (server *Server) SendUpdate(ctx context.Context, request *pb.LinkUpdate) (*emptypb.Empty, error) {
	linkUpdate := scrapperdomain.LinkUpdate{
		ID:          request.GetId(),
		URL:         request.GetUrl(),
		Description: request.GetDescription(),
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

		return nil, status.Errorf(codes.Internal, "failed to handle update for link %server: %v", request.GetUrl(), err)
	}

	return &emptypb.Empty{}, nil
}
