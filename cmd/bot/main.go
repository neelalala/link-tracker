package main

import (
	"context"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/repository/sql"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/repository/sqlbuilder"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/commands"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/grpc"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/http"
	telegramin "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/telegram"
	grpcscrapper "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/out/grpc/scrapper"
	httpscrapper "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/out/http/scrapper"
	telegramout "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/out/http/telegram"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/logger"
	"golang.org/x/sync/errgroup"
)

type ApiServer interface {
	Start(ctx context.Context) error
}

func main() {
	cfg, err := config.Load("bot.conf")
	if err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	var out io.Writer = os.Stdout

	if cfg.Logger.File != "" {
		file, err := os.OpenFile(cfg.Logger.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		out = file
	}

	slogger := logger.NewLogger(cfg.Logger.Level, out)

	tgClient, err := telegramout.NewClient(cfg.Telegram.ApiUrl, cfg.Telegram.Token, cfg.Telegram.Timeout)
	if err != nil {
		slogger.Error("Error creating telegram client",
			slog.String("context", "main"),
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	notifyService := application.NewNotifierService(slogger, tgClient)
	var apiServer ApiServer
	var scrapperApi domain.Scrapper
	switch cfg.Server.Protocol {
	case config.HTTP:
		apiServer = http.NewServer(cfg.Server.Port, notifyService, slogger)
	case config.GRPC:
		apiServer = grpc.NewServer(cfg.Server.Port, notifyService, slogger)
	default:
		slogger.Error("unsupported protocol", "protocol", cfg.Server.Protocol)
		os.Exit(1)
	}

	switch cfg.ScrapperService.Protocol {
	case config.HTTP:
		scrapperApi = httpscrapper.NewClient(cfg.ScrapperService.URL)
	case config.GRPC:
		scrapperApi, err = grpcscrapper.NewClient(cfg.ScrapperService.URL)
		if err != nil {
			slogger.Error("error creating grpc scrapper",
				slog.String("context", "main"),
				slog.String("error", err.Error()),
			)
			os.Exit(1)
		}
	default:
		slogger.Error("unsupported protocol", "protocol", cfg.ScrapperService.Protocol)
		os.Exit(1)
	}

	dbPool, err := pgxpool.New(context.Background(), cfg.Database.URL)
	if err != nil {
		slogger.Error("unable to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer dbPool.Close()

	var sessionRepo domain.SessionRepository
	switch cfg.Database.AccessType {
	case config.AccessTypeSQL:
		sessionRepo = sql.NewSessionRepository(dbPool)
	case config.AccessTypeBUILDER:
		sessionRepo = sqlbuilder.NewSessionRepository(dbPool)
	}

	helpCommand := commands.NewHelpCommand()
	startCommand := commands.NewStartCommand(scrapperApi, slogger)
	listCommand := commands.NewListCommand(scrapperApi, slogger)
	trackCommand := commands.NewTrackCommand(sessionRepo, slogger)
	untrackCommand := commands.NewUntrackCommand(sessionRepo, slogger)
	cancelCommand := commands.NewCancelCommand(sessionRepo, slogger)

	cmds := []domain.Command{
		helpCommand,
		startCommand,
		listCommand,
		trackCommand,
		untrackCommand,
		cancelCommand,
	}

	helpCommand.SetCommands(cmds)

	commandService := application.NewCommandService(scrapperApi, sessionRepo, cmds)

	dialogService := application.NewDialogService(scrapperApi, sessionRepo, slogger)

	poller, err := telegramin.NewPoller(commandService, dialogService, tgClient, slogger, cfg.Telegram.Timeout)
	if err != nil {
		slogger.Error("Failed to create bot",
			slog.String("context", "main"),
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		slogger.Info("Starting bot API server...")
		return apiServer.Start(gCtx)
	})

	g.Go(func() error {
		slogger.Info("Starting Telegram poller...")
		poller.Start(gCtx)
		return nil
	})

	if err := g.Wait(); err != nil {
		slogger.Error("Bot stopped with error", slog.String("error", err.Error()))
	} else {
		slogger.Info("Bot successfully stopped")
	}
}
