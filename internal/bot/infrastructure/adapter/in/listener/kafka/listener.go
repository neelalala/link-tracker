package kafka

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/IBM/sarama"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type Listener struct {
	consumer sarama.ConsumerGroup
	producer sarama.SyncProducer
	handler  sarama.ConsumerGroupHandler
	topic    string

	retries int

	cancel context.CancelFunc
	done   chan struct{}
	log    *slog.Logger
}

func NewListener(
	brokers []string,
	consumerGroup,
	topic string,
	dlqTopic string,
	retries int,
	updateHandler domain.LinkUpdateHandler,
	log *slog.Logger,
) (*Listener, error) {
	config := newConfig()

	consumer, err := sarama.NewConsumerGroup(brokers, consumerGroup, config)
	if err != nil {
		return nil, err
	}

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		consumer.Close()
		return nil, fmt.Errorf("failed to create producer for dlq: %w", err)
	}

	handler := NewHandler(updateHandler, producer, dlqTopic, retries, log)

	return &Listener{
		consumer: consumer,
		producer: producer,
		handler:  handler,
		topic:    topic,
		retries:  retries,
		done:     make(chan struct{}),
		log:      log,
	}, nil
}

func (listener *Listener) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	listener.cancel = cancel

	defer close(listener.done)

	for {
		if err := listener.consumer.Consume(ctx, []string{listener.topic}, listener.handler); err != nil {
			return err
		}

		if ctx.Err() != nil {
			return nil
		}
	}

}

func (listener *Listener) Stop(ctx context.Context) error {
	listener.log.Info("Shutting down kafka listener...")

	if listener.cancel != nil {
		listener.cancel()
	}

	select {
	case <-listener.done:
	case <-ctx.Done():
		listener.log.Warn("Kafka listener shutdown context exceeded")
	}

	if err := listener.consumer.Close(); err != nil {
		return fmt.Errorf("failed to close kafka consumer: %w", err)
	}

	if err := listener.producer.Close(); err != nil {
		return fmt.Errorf("failed to close kafka producer: %w", err)
	}

	return nil
}
