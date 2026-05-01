package application

import (
	"context"
	"fmt"
	"golang.org/x/sync/errgroup"
	"io"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/commands"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/listener/kafka"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/listener/server/grpc"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/listener/server/http"
	intelegram "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/telegram"
	scrappergrpc "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/out/scrapper/grpc"
	scrapperhttp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/out/scrapper/http"
	outtelegram "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/out/telegram"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/logger"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/repository/sql"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/repository/sqlbuilder"
)

type UpdateListener interface {
	Start() error
	Stop(ctx context.Context) error
}

type Poller interface {
	Start(ctx context.Context)
}

type ScrapperClient interface {
	domain.Scrapper
	Close() error
}

type App struct {
	server UpdateListener
	poller Poller
	log    *slog.Logger

	closers []func() error
}

func (app *App) onClose(f func() error) {
	app.closers = append(app.closers, f)
}

func NewApp(configPath string, out io.Writer) (*App, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("error loading config: %v", err)
	}

	app := &App{}

	if cfg.Logger.File != "" {
		file, err := os.OpenFile(cfg.Logger.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("error opening file: %v", err)
		}
		out = file
		app.onClose(file.Close)
	}

	log := logger.NewLogger(cfg.Logger.Level, out)
	app.log = log

	tgClient, err := outtelegram.NewClient(cfg.Telegram.ApiUrl, cfg.Telegram.Token, cfg.Telegram.Timeout)
	if err != nil {
		return nil, fmt.Errorf("error creating telegram client: %v", err)
	}

	notifyService := NewNotifierService(log, tgClient)

	listener, err := buildListener(cfg, notifyService, log)
	if err != nil {
		return nil, fmt.Errorf("error creation update listener: %v", err)
	}
	app.server = listener

	scrapper, err := buildScrapperClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("error creation scrapper client: %v", err)
	}
	app.onClose(scrapper.Close)

	dbPool, err := pgxpool.New(context.Background(), cfg.Database.URL)
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %v", err)
	}
	app.onClose(func() error {
		dbPool.Close()
		return nil
	})

	sessionRepo, err := buildRepos(cfg, dbPool)
	if err != nil {
		return nil, fmt.Errorf("error creating session repository: %v", err)
	}

	cmds := buildCommands(scrapper, sessionRepo, log)

	commandService := NewCommandService(scrapper, cmds)

	dialogService := NewDialogService(scrapper, sessionRepo, log)

	poller, err := intelegram.NewPoller(commandService, dialogService, tgClient, log, cfg.Telegram.Timeout)
	if err != nil {
		return nil, fmt.Errorf("error creating telegram poller: %v", err)
	}
	app.poller = poller

	return app, nil
}

func (app *App) Start(ctx context.Context) error {
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		app.log.Info("Starting bot API server...")
		return app.server.Start()
	})

	g.Go(func() error {
		app.log.Info("Starting Telegram poller...")
		app.poller.Start(gCtx)
		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	} else {
		return nil
	}
}

func (app *App) Shutdown(ctx context.Context) {
	app.log.Info("shutting down bot...")

	err := app.server.Stop(ctx)
	if err != nil {
		app.log.Error("failed to stop bot", slog.String("error", err.Error()))
	}

	for i := len(app.closers) - 1; i >= 0; i-- {
		if err := app.closers[i](); err != nil {
			app.log.Error("error during cleanup", slog.String("error", err.Error()))
		}
	}

	app.log.Info("bot successfully stopped")
}

func buildListener(cfg *config.Config, notifier *NotifierService, log *slog.Logger) (UpdateListener, error) {
	if cfg.UseQueue {
		kafka, err := kafka.NewListener(cfg.Kafka.Brokers, cfg.Kafka.ConsumerGroup, cfg.Kafka.Topic, notifier, log)
		if err != nil {
			return nil, fmt.Errorf("error creating kafka listener: %v", err)
		}
		return kafka, nil
	}
	switch cfg.Server.Protocol {
	case config.HTTP:
		server := http.NewServer(cfg.Server.Port, notifier, log)
		return server, nil
	case config.GRPC:
		server := grpc.NewServer(cfg.Server.Port, notifier, log)
		return server, nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %v", cfg.Server.Protocol)
	}
}

func buildScrapperClient(cfg *config.Config) (ScrapperClient, error) {
	switch cfg.ScrapperService.Protocol {
	case config.HTTP:
		scrapper := scrapperhttp.NewClient(cfg.ScrapperService.URL)
		return scrapper, nil
	case config.GRPC:
		scrapper, err := scrappergrpc.NewClient(cfg.ScrapperService.URL)
		if err != nil {
			return nil, fmt.Errorf("error creating scrapper: %v", err)
		}
		return scrapper, nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %v", cfg.ScrapperService.Protocol)
	}
}

func buildRepos(cfg *config.Config, dbPool *pgxpool.Pool) (domain.SessionRepository, error) {
	switch cfg.Database.AccessType {
	case config.AccessTypeSQL:
		sessionRepo := sql.NewSessionRepository(dbPool)
		return sessionRepo, nil
	case config.AccessTypeBUILDER:
		sessionRepo := sqlbuilder.NewSessionRepository(dbPool)
		return sessionRepo, nil
	default:
		return nil, fmt.Errorf("unsupported database access type: %v", cfg.Database.AccessType)
	}
}

func buildCommands(scrapper domain.Scrapper, sessionRepo domain.SessionRepository, log *slog.Logger) []domain.Command {
	helpCommand := commands.NewHelpCommand()
	startCommand := commands.NewStartCommand(scrapper, log)
	listCommand := commands.NewListCommand(scrapper, log)
	trackCommand := commands.NewTrackCommand(sessionRepo, log)
	untrackCommand := commands.NewUntrackCommand(sessionRepo, log)
	cancelCommand := commands.NewCancelCommand(sessionRepo, log)

	cmds := []domain.Command{
		helpCommand,
		startCommand,
		listCommand,
		trackCommand,
		untrackCommand,
		cancelCommand,
	}

	helpCommand.SetCommands(cmds)

	return cmds
}
