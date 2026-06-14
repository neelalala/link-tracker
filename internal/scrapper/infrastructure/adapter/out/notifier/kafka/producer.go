package kafka

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/IBM/sarama"
	"github.com/linkedin/goavro/v2"
	"github.com/riferrei/srclient"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type TopicConfig struct {
	Topic        string
	SchemaString string
	ParseFunc    func(payload []byte) (map[string]any, error)
}

type topicSerializer struct {
	schemaID int
	codec    *goavro.Codec
	parse    func(payload []byte) (map[string]any, error)
}

type Producer struct {
	sync    sarama.SyncProducer
	outRepo domain.OutboxRepository

	topic      string
	serializer topicSerializer

	limit      int
	maxRetries int
	log        *slog.Logger
}

func NewProducer(
	brokers []string,
	registryURL string,
	topic TopicConfig,
	outRepo domain.OutboxRepository,
	limit int,
	maxRetries int,
	log *slog.Logger,
) (*Producer, error) {
	srClient := srclient.NewSchemaRegistryClient(registryURL)

	schema, err := getSchema(srClient, topic.Topic, topic.SchemaString, log)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema for topic %s: %w", topic.Topic, err)
	}
	serializer := topicSerializer{
		schemaID: schema.ID(),
		codec:    schema.Codec(),
		parse:    topic.ParseFunc,
	}

	config := newConfig()

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}

	return &Producer{
		sync:       producer,
		outRepo:    outRepo,
		topic:      topic.Topic,
		serializer: serializer,
		limit:      limit,
		maxRetries: maxRetries,
		log:        log,
	}, nil
}

func (producer *Producer) SendEvents(ctx context.Context) error {
	events, err := producer.outRepo.GetPending(ctx, producer.topic, producer.limit)
	if err != nil {
		return fmt.Errorf("cannot get pending messages: %w", err)
	}

	producer.log.Debug("Sending events", "events_count", len(events))

	for _, event := range events {
		value, err := eventToBytes(producer.serializer, event)
		if err != nil {
			producer.log.Error("Failed to serialize event",
				"topic", event.Topic,
				"event_id", event.ID,
				"error", err,
			)
			producer.markFailed(ctx, event)
			continue
		}

		msg := &sarama.ProducerMessage{
			Topic: event.Topic,
			Key:   sarama.StringEncoder(strconv.FormatInt(event.ID, 10)),
			Value: sarama.ByteEncoder(value),
		}

		partition, offset, err := producer.sync.SendMessage(msg)
		if err != nil {
			producer.log.Error("Failed to send event to kafka",
				"error", err,
				"topic", event.Topic,
				"event_id", event.ID,
			)
			producer.incrementRetries(ctx, event)
			continue
		}

		producer.markSent(ctx, event)

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

func (producer *Producer) incrementRetries(ctx context.Context, event domain.Outbox) {
	retries, err := producer.outRepo.IncrementRetries(ctx, event.ID)
	if err != nil {
		producer.log.Error("Failed to increment retries",
			"error", err,
			"event_id", event.ID,
		)
		return
	}

	if retries >= producer.maxRetries {
		producer.log.Warn("Event exceeded max retries, marking as FAILED",
			"event_id", event.ID,
			"retries", retries,
			"max_retries", producer.maxRetries,
		)
		if err := producer.outRepo.UpdateStatus(ctx, event.ID, domain.OutboxStatusFailed); err != nil {
			producer.log.Error("Failed to mark event as FAILED",
				"error", err,
				"event_id", event.ID,
			)
		}
	}
}

func (producer *Producer) markSent(ctx context.Context, event domain.Outbox) {
	if err := producer.outRepo.UpdateStatus(ctx, event.ID, domain.OutboxStatusSent); err != nil {
		producer.log.Error("Failed to mark event as SENT",
			"error", err,
			"event_id", event.ID,
		)
	}
}

func (producer *Producer) markFailed(ctx context.Context, event domain.Outbox) {
	if err := producer.outRepo.UpdateStatus(ctx, event.ID, domain.OutboxStatusFailed); err != nil {
		producer.log.Error("Failed to mark event as FAILED",
			"error", err,
			"event_id", event.ID,
		)
	}
}

func getSchema(srClient *srclient.SchemaRegistryClient, topic, schemaString string, log *slog.Logger) (*srclient.Schema, error) {
	schema, err := srClient.GetLatestSchema(topic)
	if schema == nil {
		if err != nil {
			log.Debug("Error getting schema, creating new one",
				"topic", topic,
				"error", err,
			)
		} else {
			log.Debug("No schema found for topic, creating new one",
				"topic", topic,
			)
		}

		schema, err = srClient.CreateSchema(topic, schemaString, srclient.Avro)
		if err != nil {
			return nil, fmt.Errorf("failed to register schema for topic %s: %w", topic, err)
		}
	}

	return schema, nil
}

func eventToBytes(serializer topicSerializer, event domain.Outbox) ([]byte, error) {
	nativeData, err := serializer.parse(event.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to parse payload: %v", err)
	}

	avroBinary, err := serializer.codec.BinaryFromNative(nil, nativeData)
	if err != nil {
		return nil, fmt.Errorf("failed to encode avro: %v", err)
	}

	schemaIDBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(schemaIDBytes, uint32(serializer.schemaID))

	var recordValue []byte
	recordValue = append(recordValue, byte(0))
	recordValue = append(recordValue, schemaIDBytes...)
	recordValue = append(recordValue, avroBinary...)

	return recordValue, nil
}
