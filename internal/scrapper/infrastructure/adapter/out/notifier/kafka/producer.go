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
	SchemaString string
	ParseFunc    func(payload []byte) (map[string]any, error)
}

type topicSerializer struct {
	schemaID int
	codec    *goavro.Codec
	parse    func(payload []byte) (map[string]any, error)
}

type Producer struct {
	sync     sarama.SyncProducer
	outRepo  domain.OutboxRepository
	registry map[string]topicSerializer

	limit int
	log   *slog.Logger
}

func NewProducer(
	brokers []string,
	registryURL string,
	configs map[string]TopicConfig,
	outRepo domain.OutboxRepository,
	limit int,
	log *slog.Logger,
) (*Producer, error) {
	srClient := srclient.NewSchemaRegistryClient(registryURL)

	registry := make(map[string]topicSerializer, len(configs))
	for topic, cfg := range configs {
		schema, err := getSchema(srClient, topic, cfg.SchemaString, log)
		if err != nil {
			return nil, fmt.Errorf("failed to get schema for topic %s: %w", topic, err)
		}
		registry[topic] = topicSerializer{
			schemaID: schema.ID(),
			codec:    schema.Codec(),
			parse:    cfg.ParseFunc,
		}
	}

	config := newConfig()

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}

	return &Producer{
		sync:     producer,
		outRepo:  outRepo,
		registry: registry,
		limit:    limit,
		log:      log,
	}, nil
}

func (producer *Producer) SendEvents(ctx context.Context) error {
	events, err := producer.outRepo.GetPending(ctx, producer.limit)
	if err != nil {
		return fmt.Errorf("cannot get pending messages: %w", err)
	}

	producer.log.Debug("Sending events", "events_count", len(events))

	for _, event := range events {
		serializer, exists := producer.registry[event.Topic]
		if !exists {
			producer.log.Error("No schema config found for topic", "topic", event.Topic, "event_id", event.ID)
			// dlq?
			continue
		}

		nativeData, err := serializer.parse(event.Payload)
		if err != nil {
			producer.log.Error("Failed to parse payload", "error", err, "event_id", event.ID)
			continue
		}

		avroBinary, err := serializer.codec.BinaryFromNative(nil, nativeData)
		if err != nil {
			producer.log.Error("Failed to encode avro", "error", err, "event_id", event.ID)
			continue
		}

		schemaIDBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(schemaIDBytes, uint32(serializer.schemaID))

		var recordValue []byte
		recordValue = append(recordValue, byte(0))
		recordValue = append(recordValue, schemaIDBytes...)
		recordValue = append(recordValue, avroBinary...)

		msg := &sarama.ProducerMessage{
			Topic: event.Topic,
			Key:   sarama.StringEncoder(strconv.FormatInt(event.ID, 10)),
			Value: sarama.ByteEncoder(recordValue),
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
