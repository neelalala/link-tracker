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

type Producer struct {
	sync  sarama.SyncProducer
	topic string
	log   *slog.Logger
}

func NewProducer(brokers []string, topic string, log *slog.Logger) (*Producer, error) {
	config := newConfig()

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}

	return &Producer{
		sync:  producer,
		topic: topic,
		log:   log,
	}, nil
}

func (producer *Producer) SendUpdate(ctx context.Context, update domain.LinkUpdate) error {
	producer.log.Info("Sending update",
		"update", update,
		"topic", producer.topic,
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
		Topic: producer.topic,
		Key:   sarama.StringEncoder(strconv.FormatInt(update.ID, 10)),
		Value: sarama.ByteEncoder(bytes),
	}

	partition, offset, err := producer.sync.SendMessage(msg)
	if err != nil {
		return err
	}

	producer.log.Info("Update sent",
		"topic", producer.topic,
		"partition", partition,
		"offset", offset)
	return nil
}

func (producer *Producer) Close() error {
	if err := producer.sync.Close(); err != nil {
		return fmt.Errorf("failed to close kafka sync producer: %w", err)
	}
	return nil
}
