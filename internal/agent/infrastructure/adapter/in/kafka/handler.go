package kafka

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/riferrei/srclient"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/infrastructure/adapter/in/kafka/mapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/validation"
)

const avroMagicByte = 0x00

type Handler struct {
	updateHandler UpdateHandler
	producer      sarama.SyncProducer
	dlqTopic      string

	delay         time.Duration
	maxDelay      time.Duration
	backoffFactor float64
	retries       int

	ready     chan struct{}
	readyOnce sync.Once
	srClient  *srclient.SchemaRegistryClient

	log *slog.Logger
}

func NewHandler(
	updateHandler UpdateHandler,
	producer sarama.SyncProducer,
	dlqTopic string,
	delay time.Duration,
	maxDelay time.Duration,
	backoffFactor float64,
	retries int,
	registryURL string,
	log *slog.Logger,
) *Handler {
	return &Handler{
		updateHandler: updateHandler,
		producer:      producer,
		dlqTopic:      dlqTopic,
		delay:         delay,
		maxDelay:      maxDelay,
		backoffFactor: backoffFactor,
		retries:       retries,
		ready:         make(chan struct{}),
		srClient:      srclient.NewSchemaRegistryClient(registryURL),
		log:           log,
	}
}

func (handler *Handler) Setup(sarama.ConsumerGroupSession) error {
	handler.readyOnce.Do(func() {
		close(handler.ready)
	})
	return nil
}

func (handler *Handler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (handler *Handler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}

			handler.log.Info("message received",
				"key", string(message.Key),
				"topic", message.Topic,
				"partition", message.Partition,
				"offset", message.Offset,
			)

			update, err := handler.bytesToLinkUpdate(message.Value)
			if err != nil {
				handler.log.Error("failed to unmarshall update", "error", err)
				handler.sendToDLQ(message, fmt.Sprintf("unmarshal error: %v", err))
				session.MarkMessage(message, "")
				continue
			}

			if err := validation.Check(update); err != nil {
				handler.log.Error("failed to validate update", "error", err)
				handler.sendToDLQ(message, fmt.Sprintf("validate error: %v", err))
				session.MarkMessage(message, "")
				continue
			}

			err = handler.handleUpdate(session.Context(), update)
			if err != nil {
				handler.log.Error("failed to handle update", "error", err)
				handler.sendToDLQ(message, fmt.Sprintf("handle update error: %v", err))
			}

			session.MarkMessage(message, "")
		case <-session.Context().Done():
			return session.Context().Err()
		}

	}
}

func (handler *Handler) handleUpdate(ctx context.Context, update domain.LinkUpdate) error {
	var processErr error
	backoff := handler.delay
	for i := range handler.retries {
		if err := handler.updateHandler.HandleUpdate(ctx, update); err != nil {
			processErr = err
			handler.log.Warn("failed to handle update",
				slog.String("error", err.Error()),
				slog.Int("attempt", i+1),
				slog.Int64("link_id", update.ID))

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}

			backoff = time.Duration(float64(handler.delay) * math.Pow(handler.backoffFactor, float64(i+1)))
			if backoff > handler.maxDelay {
				backoff = handler.maxDelay
			}

			continue
		}
		return nil
	}
	return processErr
}

func (handler *Handler) sendToDLQ(msg *sarama.ConsumerMessage, reason string) {
	dlqMsg := &sarama.ProducerMessage{
		Topic: handler.dlqTopic,
		Key:   sarama.ByteEncoder(msg.Key),
		Value: sarama.ByteEncoder(msg.Value),
		Headers: []sarama.RecordHeader{
			{Key: []byte("reason"), Value: []byte(reason)},
			{Key: []byte("topic"), Value: []byte(msg.Topic)},
			{Key: []byte("partition"), Value: []byte(fmt.Sprintf("%d", msg.Partition))},
			{Key: []byte("offset"), Value: []byte(fmt.Sprintf("%d", msg.Offset))},
		},
	}

	partition, offset, err := handler.producer.SendMessage(dlqMsg)
	if err != nil {
		handler.log.Error("failed to send message to DLQ", "error", err)
		return
	}

	handler.log.Info("message sent to DLQ successfully",
		"dlq_partition", partition,
		"dlq_offset", offset,
	)
}

func (handler *Handler) bytesToLinkUpdate(value []byte) (domain.LinkUpdate, error) {
	if len(value) <= 5 || value[0] != avroMagicByte {
		return domain.LinkUpdate{}, errors.New("invalid link update serialization")
	}

	schemaID := binary.BigEndian.Uint32(value[1:5])
	schema, err := handler.srClient.GetSchema(int(schemaID))
	if err != nil {
		return domain.LinkUpdate{}, fmt.Errorf("error getting the schema: %w", err)
	}

	native, _, err := schema.Codec().NativeFromBinary(value[5:])
	if err != nil {
		return domain.LinkUpdate{}, fmt.Errorf("error decoding native from bytes: %w", err)
	}

	linkUpdate, err := mapper.LinkUpdateFromNative(native)
	if err != nil {
		return domain.LinkUpdate{}, fmt.Errorf("error mapping native to domain LinkUpdate: %w", err)
	}

	return linkUpdate, nil
}
