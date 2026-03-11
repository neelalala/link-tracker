package telegram

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/out/telegram"
	"log/slog"
)

type Poller struct {
	tgClient       *telegram.Client
	commandService *application.CommandService
	logger         *slog.Logger
}

func NewPoller(tgClient *telegram.Client, scrapper application.Scrapper, logger *slog.Logger) (*Poller, error) {
	commandService := application.NewCommandService(scrapper, logger)

	cmds := commandService.GetCommands()
	err := tgClient.SetMyCommands(cmds)
	if err != nil {
		return nil, err
	}

	return &Poller{
		tgClient:       tgClient,
		commandService: commandService,
		logger:         logger,
	}, nil
}

func (poller *Poller) Start() {
	poller.logger.Info("Poller started listening for telegram updates", slog.String("context", "poller.Start"))

	for {
		updates, err := poller.tgClient.GetUpdates()
		if err != nil {
			poller.logger.Error("Failed to get updated", slog.String("error", err.Error()), slog.String("context", "tgClient.GetUpdates"))
			continue
		}
		for _, update := range updates {
			err := poller.handleMessage(update)
			if err != nil {
				poller.logger.Error("Failed to handle update", slog.String("error", err.Error()), slog.String("context", "poller.handleMessage"))
			}
		}
	}
}

func (poller *Poller) handleMessage(msg domain.Message) error {
	resp := poller.commandService.HandleMessage(msg.ChatID, msg.Text)
	return poller.tgClient.SendMessage(msg.ChatID, resp)
}
