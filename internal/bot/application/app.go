package application

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/commands"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/server/grpc"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/server/http"
	telegramin "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/telegram"
	grpcscrapper "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/out/scrapper/grpc"
	httpscrapper "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/out/scrapper/http"
	telegramout "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/out/telegram"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/logger"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/repository/sql"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/repository/sqlbuilder"
	"golang.org/x/sync/errgroup"
	"io"
	"log"
	"log/slog"
	"os"
)

type APIServer interface {
	Start(ctx context.Context) error
}

type Poller interface {
	Start(ctx context.Context)
}

type App struct {
	server  APIServer
	poller  Poller
	slogger *slog.Logger
}

func NewApp(configPath string, out io.Writer) *App {
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("error loading config: %v", err)
	}

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

	notifyService := NewNotifierService(slogger, tgClient)

	var server APIServer
	switch cfg.Server.Protocol {
	case config.HTTP:
		server = http.NewServer(cfg.Server.Port, notifyService, slogger)
	case config.GRPC:
		server = grpc.NewServer(cfg.Server.Port, notifyService, slogger)
	default:
		slogger.Error("unsupported protocol", "protocol", cfg.Server.Protocol)
		os.Exit(1)
	}

	var scrapper domain.Scrapper
	switch cfg.ScrapperService.Protocol {
	case config.HTTP:
		scrapper = httpscrapper.NewClient(cfg.ScrapperService.URL)
	case config.GRPC:
		scrapper, err = grpcscrapper.NewClient(cfg.ScrapperService.URL)
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
	startCommand := commands.NewStartCommand(scrapper, slogger)
	listCommand := commands.NewListCommand(scrapper, slogger)
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

	commandService := NewCommandService(scrapper, cmds)

	dialogService := NewDialogService(scrapper, sessionRepo, slogger)

	poller, err := telegramin.NewPoller(commandService, dialogService, tgClient, slogger, cfg.Telegram.Timeout)
	if err != nil {
		slogger.Error("Failed to create bot",
			slog.String("context", "main"),
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	return &App{
		server:  server,
		poller:  poller,
		slogger: slogger,
	}
}

func (app *App) Start(ctx context.Context) {
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		app.slogger.Info("Starting bot API server...")
		return app.server.Start(gCtx)
	})

	g.Go(func() error {
		app.slogger.Info("Starting Telegram poller...")
		app.poller.Start(gCtx)
		return nil
	})

	if err := g.Wait(); err != nil {
		app.slogger.Error("Bot stopped with error", slog.String("error", err.Error()))
	} else {
		app.slogger.Info("Bot successfully stopped")
	}
}
