package telegram

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/adapter/out/telegram"
	"log/slog"
	"strings"
)

type Poller struct {
	tgClient *telegram.Client
	router   *application.Router
	logger   *slog.Logger
}

func NewPoller(tgClient *telegram.Client, cmds []domain.Command, logger *slog.Logger) (*Poller, error) {
	err := tgClient.SetMyCommands(cmds)
	if err != nil {
		return nil, err
	}
	router := application.NewRouter(cmds)

	return &Poller{
		tgClient: tgClient,
		router:   router,
		logger:   logger,
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
	if strings.HasPrefix(msg.Text, "/") {
		ss := strings.Split(msg.Text, " ")
		resp := poller.router.Handle(ss[0], ss[1:], msg.From, msg.ChatID)
		return poller.tgClient.SendMessage(msg.ChatID, resp)
	}
	return nil
}
