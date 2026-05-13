package kafka

import (
	"context"
	"log/slog"
	"time"
)

type Worker struct {
	ctx      context.Context
	producer *Producer
	interval time.Duration

	log *slog.Logger
}

func NewWorker(ctx context.Context, producer *Producer, interval time.Duration, log *slog.Logger) *Worker {
	return &Worker{
		ctx:      ctx,
		producer: producer,
		interval: interval,
		log:      log,
	}
}

func (worker *Worker) Start() {
	ticker := time.NewTicker(worker.interval)
	defer ticker.Stop()

	for {
		select {
		case <-worker.ctx.Done():
			return
		case <-ticker.C:
			err := worker.producer.SendEvents(worker.ctx)
			if err != nil {
				worker.log.Error("Failed to send events to Kafka", "error", err)
			}
		}
	}
}
