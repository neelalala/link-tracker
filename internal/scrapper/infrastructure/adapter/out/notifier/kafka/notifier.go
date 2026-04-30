package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/IBM/sarama"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type Notifier struct {
	producer sarama.SyncProducer
	topic    string
	log      *slog.Logger
}

func NewNotifier(brokers []string, topic string, log *slog.Logger) (*Notifier, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}

	return &Notifier{
		producer: producer,
		topic:    topic,
		log:      log,
	}, nil
}

func (n *Notifier) SendUpdate(ctx context.Context, update domain.LinkUpdate) error {
	n.log.Info("Sending update",
		"update", update,
		"topic", n.topic,
	)

	var updateJSON = struct {
		ID          int64   `json:"id"`
		URL         string  `json:"url"`
		Description string  `json:"description"`
		Preview     string  `json:"preview"`
		TgChatIds   []int64 `json:"tgChatIds"`
	}{
		ID:          update.ID,
		URL:         update.URL,
		Description: update.Description,
		Preview:     update.Preview,
		TgChatIds:   update.TgChatIDs,
	}

	bytes, err := json.Marshal(updateJSON)
	if err != nil {
		return err
	}

	msg := &sarama.ProducerMessage{
		Topic: n.topic,
		Key:   sarama.StringEncoder(strconv.FormatInt(update.ID, 10)),
		Value: sarama.ByteEncoder(bytes),
	}

	partition, offset, err := n.producer.SendMessage(msg)
	if err != nil {
		return err
	}

	n.log.Info("Update sent",
		"topic", n.topic,
		"partition", partition,
		"offset", offset)
	return nil
}

func (n *Notifier) Close() error {
	if err := n.producer.Close(); err != nil {
		return fmt.Errorf("failed to close kafka producer: %w", err)
	}
	return nil
}
