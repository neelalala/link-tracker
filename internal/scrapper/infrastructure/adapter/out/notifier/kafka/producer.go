package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/IBM/sarama"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type Producer struct {
	sync    sarama.SyncProducer
	outRepo domain.OutboxRepository

	limit int
	log   *slog.Logger
}

func NewProducer(brokers []string, outRepo domain.OutboxRepository, limit int, log *slog.Logger) (*Producer, error) {
	config := newConfig()

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}

	return &Producer{
		sync:    producer,
		outRepo: outRepo,
		limit:   limit,
		log:     log,
	}, nil
}

func (producer *Producer) SendEvents(ctx context.Context) error {
	events, err := producer.outRepo.GetPending(ctx, producer.limit)
	if err != nil {
		return fmt.Errorf("cannot get pending messages: %w", err)
	}

	for _, event := range events {
		msg := &sarama.ProducerMessage{
			Topic: event.Topic,
			Key:   sarama.StringEncoder(strconv.FormatInt(event.ID, 10)),
			Value: sarama.ByteEncoder(event.Payload),
		}

		partition, offset, err := producer.sync.SendMessage(msg)
		if err != nil {
			producer.log.Error("Failed to send event to kafka",
				"error", err,
				"topic", event.Topic,
				"event_id", event.ID,
			)
			continue
		}

		err = producer.outRepo.UpdateStatus(ctx, event.ID, domain.OutboxStatusSent)
		if err != nil {
			producer.log.Error("Failed to update status for event",
				"error", err,
				"topic", event.Topic,
				"event_id", event.ID,
			)
			continue
		}

		producer.log.Info("Event sent",
			"topic", event.Topic,
			"partition", partition,
			"offset", offset,
		)
	}

	return nil
}

func (producer *Producer) Close() error {
	if err := producer.sync.Close(); err != nil {
		return fmt.Errorf("failed to close kafka sync producer: %w", err)
	}
	return nil
}
