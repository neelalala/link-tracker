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
	handler  sarama.ConsumerGroupHandler
	topic    string

	cancel context.CancelFunc
	done   chan struct{}
	log    *slog.Logger
}

func NewListener(brokers []string, consumerGroup, topic string, updateHandler domain.LinkUpdateHandler, log *slog.Logger) (*Listener, error) {
	config := newConfig()

	consumer, err := sarama.NewConsumerGroup(brokers, consumerGroup, config)
	if err != nil {
		return nil, err
	}

	handler := NewHandler(updateHandler, log)

	return &Listener{
		consumer: consumer,
		handler:  handler,
		topic:    topic,
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
	return nil
}
