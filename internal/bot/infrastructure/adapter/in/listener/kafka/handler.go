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

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/adapter/in/listener/kafka/mapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/validation"
)

const avroMagicByte = 0x00

type Handler struct {
	updateHandler domain.LinkUpdateHandler
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
	updateHandler domain.LinkUpdateHandler,
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

func (h *Handler) Setup(sarama.ConsumerGroupSession) error {
	h.readyOnce.Do(func() {
		close(h.ready)
	})
	return nil
}

func (h *Handler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *Handler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}

			h.log.Info("message received",
				"key", string(message.Key),
				"topic", message.Topic,
				"partition", message.Partition,
				"offset", message.Offset,
			)

			update, err := h.bytesToLinkUpdate(message.Value)
			if err != nil {
				h.log.Error("failed to unmarshall update", "error", err)
				h.sendToDLQ(message, fmt.Sprintf("unmarshal error: %v", err))
				session.MarkMessage(message, "")
				continue
			}

			if err := validation.Check(update); err != nil {
				h.log.Error("failed to validate update", "error", err)
				h.sendToDLQ(message, fmt.Sprintf("validate error: %v", err))
				session.MarkMessage(message, "")
				continue
			}

			err = h.handleUpdate(session.Context(), update)
			if err != nil {
				h.log.Error("failed to handle update", "error", err)
				h.sendToDLQ(message, fmt.Sprintf("handle update error: %v", err))
			}

			session.MarkMessage(message, "")
		case <-session.Context().Done():
			return session.Context().Err()
		}

	}
}

func (h *Handler) handleUpdate(ctx context.Context, update domain.LinkUpdate) error {
	var processErr error
	backoff := h.delay
	for i := range h.retries {
		if err := h.updateHandler.HandleUpdate(ctx, update); err != nil {
			processErr = err
			h.log.Warn("failed to handle update",
				slog.String("error", err.Error()),
				slog.Int("attempt", i+1),
				slog.Int64("link_id", update.ID))

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}

			backoff = time.Duration(float64(h.delay) * math.Pow(h.backoffFactor, float64(i+1)))
			if backoff > h.maxDelay {
				backoff = h.maxDelay
			}

			continue
		}
		return nil
	}
	return processErr
}

func (h *Handler) sendToDLQ(msg *sarama.ConsumerMessage, reason string) {
	dlqMsg := &sarama.ProducerMessage{
		Topic: h.dlqTopic,
		Key:   sarama.ByteEncoder(msg.Key),
		Value: sarama.ByteEncoder(msg.Value),
		Headers: []sarama.RecordHeader{
			{Key: []byte("reason"), Value: []byte(reason)},
			{Key: []byte("topic"), Value: []byte(msg.Topic)},
			{Key: []byte("partition"), Value: []byte(fmt.Sprintf("%d", msg.Partition))},
			{Key: []byte("offset"), Value: []byte(fmt.Sprintf("%d", msg.Offset))},
		},
	}

	partition, offset, err := h.producer.SendMessage(dlqMsg)
	if err != nil {
		h.log.Error("failed to send message to DLQ", "error", err)
		return
	}

	h.log.Info("message sent to DLQ successfully",
		"dlq_partition", partition,
		"dlq_offset", offset,
	)
}

func (h *Handler) bytesToLinkUpdate(value []byte) (domain.LinkUpdate, error) {
	if len(value) <= 5 || value[0] != avroMagicByte {
		return domain.LinkUpdate{}, errors.New("invalid link update serialization")
	}

	schemaID := binary.BigEndian.Uint32(value[1:5])
	schema, err := h.srClient.GetSchema(int(schemaID))
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
