package application

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/infrastructure/adapter/in/kafka"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/infrastructure/adapter/out/sender"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/infrastructure/filters"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/infrastructure/logger"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/infrastructure/repository/sql"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/infrastructure/repository/sqlbuilder"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/infrastructure/transformers"
)

type UpdateListener interface {
	Start() error
	Stop(ctx context.Context) error
}

type App struct {
	listener UpdateListener
	log      *slog.Logger

	closers []func() error
}

func (app *App) onClose(f func() error) {
	app.closers = append(app.closers, f)
}

func NewApp(cfgPath string, out io.Writer) (*App, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}

	fmt.Printf("config: %+v\n", cfg)

	app := &App{}

	if cfg.Logger.File != "" {
		file, err := os.OpenFile(cfg.Logger.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("error opening log file: %v", err)
		}
		out = file
		app.onClose(file.Close)
	}

	log := logger.NewLogger(cfg.Logger.Level, out)

	log.Info("Info messages are enabled")
	log.Debug("Debug messages are enabled")

	app.log = log

	log.Debug("Creating db pool")
	dbPool, err := pgxpool.New(context.Background(), cfg.Database.URL)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}
	app.onClose(func() error {
		dbPool.Close()
		return nil
	})

	log.Debug("Building outbox repository")
	outboxRepo, err := buildRepo(cfg, dbPool)
	if err != nil {
		return nil, fmt.Errorf("error creating repository: %v", err)
	}

	filter := buildFilters(cfg.Filters)
	transformer := buildTransformer(cfg.Transformers)

	outboxSender := sender.NewOutbox(outboxRepo, cfg.Kafka.ProcessedUpdateTopic, log)

	service := NewService(filter, transformer, outboxSender, log)

	listener, err := buildListener(cfg.Kafka, service, log)
	if err != nil {
		return nil, fmt.Errorf("error creating listener: %v", err)
	}

	app.listener = listener

	return app, nil
}

func (app *App) Start() error {
	return app.listener.Start()
}

func (app *App) Shutdown(ctx context.Context) error {
	return app.listener.Stop(ctx)
}

func buildRepo(cfg config.Config, dbPool *pgxpool.Pool) (domain.OutboxRepository, error) {
	switch cfg.Database.AccessType {
	case config.AccessTypeSQL:
		outboxRepo := sql.NewOutboxRepository(dbPool)
		return outboxRepo, nil
	case config.AccessTypeBUILDER:
		outboxRepo := sqlbuilder.NewOutboxRepository(dbPool)
		return outboxRepo, nil
	default:
		return nil, fmt.Errorf("unsupported database access type: %s", cfg.Database.AccessType)
	}
}

func buildFilters(cfg config.FiltersConfig) []domain.Filter {
	filter := make([]domain.Filter, 0)

	if cfg.Author.Enabled {
		author := filters.NewAuthor(cfg.Author.Excluded)
		filter = append(filter, author)
	}

	if cfg.StopWords.Enabled {
		words := filters.NewWords(cfg.StopWords.StopWords)
		filter = append(filter, words)
	}

	if cfg.Length.Enabled {
		length := filters.NewLength(cfg.Length.MinLength)
		filter = append(filter, length)
	}

	return filter
}

func buildTransformer(cfg config.TransformerConfig) domain.Transformer {
	return transformers.Cutter(cfg.Threshold)
}

func buildListener(cfg config.KafkaConfig, notifier kafka.UpdateHandler, log *slog.Logger) (UpdateListener, error) {
	log.Info("using queue as listener")
	kafka, err := kafka.NewListener(
		cfg.Brokers,
		cfg.ConsumerGroup,
		cfg.RawUpdateTopic,
		cfg.DLQTopic,
		cfg.Retries.Delay,
		cfg.Retries.MaxDelay,
		cfg.Retries.BackoffFactor,
		cfg.Retries.MaxRetries,
		cfg.SchemaRegistryURL,
		notifier,
		log,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating kafka listener: %v", err)
	}
	return kafka, nil
}
