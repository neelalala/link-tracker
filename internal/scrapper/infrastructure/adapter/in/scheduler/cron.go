package scheduler

import (
	"context"
	"time"

	"github.com/go-co-op/gocron/v2"
)

type Scheduler struct {
	cron gocron.Scheduler

	baseCtx context.Context
	cancel  context.CancelFunc
}

func New(ctx context.Context) (*Scheduler, error) {
	cron, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}

	cronCtx, cancel := context.WithCancel(ctx)

	scheduler := &Scheduler{
		cron:    cron,
		baseCtx: cronCtx,
		cancel:  cancel,
	}

	return scheduler, nil
}

func (scheduler *Scheduler) Schedule(interval time.Duration, jobTimeout time.Duration, jobFunc func(ctx context.Context)) error {
	wrapperFunc := func() {
		ctx, cancel := context.WithTimeout(scheduler.baseCtx, jobTimeout)
		defer cancel()
		jobFunc(ctx)
	}

	_, err := scheduler.cron.NewJob(
		gocron.DurationJob(interval),
		gocron.NewTask(wrapperFunc),
	)
	if err != nil {
		return err
	}
	return nil
}

func (scheduler *Scheduler) Start() {
	scheduler.cron.Start()
}

func (scheduler *Scheduler) Shutdown() error {
	scheduler.cancel()
	err := scheduler.cron.Shutdown()
	if err != nil {
		return err
	}
	return nil
}
