package telegram

import (
	"context"
	"errors"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/out/http/telegram"
	"log/slog"
	"time"
)

type Poller struct {
	tgClient       *telegram.Client
	commandService *application.CommandService
	dialogService  *application.DialogService
	logger         *slog.Logger
	timeout        time.Duration
}

func NewPoller(
	commandService *application.CommandService,
	dialogService *application.DialogService,
	tgClient *telegram.Client,
	logger *slog.Logger,
	timeout time.Duration,
) (*Poller, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	commands := commandService.GetCommandsInfo()
	err := tgClient.SetMyCommands(ctx, commands)
	if err != nil {
		return nil, err
	}

	return &Poller{
		tgClient:       tgClient,
		commandService: commandService,
		dialogService:  dialogService,
		logger:         logger,
		timeout:        timeout,
	}, nil
}

func (poller *Poller) Start(ctx context.Context) {
	poller.logger.Info("Poller started listening for telegram updates", slog.String("context", "poller.Start"))

	for {
		select {
		case <-ctx.Done():
			poller.logger.Info("Poller gracefully stopped")
			return
		default:
		}

		requestCtx, cancel := context.WithTimeout(ctx, poller.timeout)

		updates, err := poller.tgClient.GetUpdates(requestCtx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				cancel()
				return
			}

			poller.logger.Error("Failed to get updates",
				slog.String("error", err.Error()),
				slog.String("context", "tgClient.GetUpdates"),
			)
			cancel()
			//time.Sleep(1 * time.Second)
			continue
		}

		for _, update := range updates {
			err := poller.handleMessage(requestCtx, update)
			if err != nil {
				poller.logger.Error("Failed to handle update",
					slog.String("error", err.Error()),
					slog.String("context", "poller.handleMessage"),
				)
			}
		}
		cancel()
	}
}

func (poller *Poller) handleMessage(ctx context.Context, msg domain.Message) error {
	var response string
	var processErr error

	if msg.IsCommand() {
		response, processErr = poller.commandService.HandleCommand(ctx, msg)
	} else {
		response, processErr = poller.dialogService.HandleMessage(ctx, msg)
	}

	if processErr != nil {
		poller.logger.Error("failed to process message",
			slog.String("error", processErr.Error()),
			slog.Int64("chat_id", msg.ChatID),
			slog.String("user_message", msg.Text),
		)

		if response == "" {
			response = "Something went wrong. Try again later.️"
		}
	}

	if response != "" {
		return poller.tgClient.SendMessage(ctx, msg.ChatID, response)
	}

	return nil
}
