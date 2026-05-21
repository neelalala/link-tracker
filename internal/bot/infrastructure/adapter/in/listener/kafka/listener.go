package kafka

import (
	"context"
	"errors"
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

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	log    *slog.Logger
}

func NewListener(
	brokers []string,
	consumerGroup string,
	topic string,
	dlqTopic string,
	retries int,
	registryURL string,
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

	handler := NewHandler(
		updateHandler,
		producer,
		dlqTopic,
		retries,
		registryURL,
		log,
	)

	ctx, cancel := context.WithCancel(context.Background())

	return &Listener{
		consumer: consumer,
		producer: producer,
		handler:  handler,
		topic:    topic,
		ctx:      ctx,
		cancel:   cancel,
		done:     make(chan struct{}),
		log:      log,
	}, nil
}

func (listener *Listener) Start() error {
	defer close(listener.done)

	for {
		if err := listener.consumer.Consume(listener.ctx, []string{listener.topic}, listener.handler); err != nil {
			return err
		}

		if listener.ctx.Err() != nil {
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

	errConsumer := listener.consumer.Close()

	errProducer := listener.producer.Close()

	err := errors.Join(errConsumer, errProducer)

	return err
}
