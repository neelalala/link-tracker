package scheduler

import (
	"context"
	"time"

	"github.com/go-co-op/gocron/v2"
)

type Cron struct {
	scheduler gocron.Scheduler

	baseCtx context.Context
	cancel  context.CancelFunc
}

func NewCron(ctx context.Context) (*Cron, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}

	cronCtx, cancel := context.WithCancel(ctx)

	cron := &Cron{
		scheduler: scheduler,
		baseCtx:   cronCtx,
		cancel:    cancel,
	}

	return cron, nil
}

func (cron *Cron) Schedule(interval time.Duration, jobTimeout time.Duration, jobFunc func(ctx context.Context)) error {
	wrapperFunc := func() {
		ctx, cancel := context.WithTimeout(cron.baseCtx, jobTimeout)
		defer cancel()
		jobFunc(ctx)
	}

	_, err := cron.scheduler.NewJob(
		gocron.DurationJob(interval),
		gocron.NewTask(wrapperFunc),
	)
	if err != nil {
		return err
	}
	return nil
}

func (cron *Cron) Start() {
	cron.scheduler.Start()
}

func (cron *Cron) Shutdown() error {
	cron.cancel()
	err := cron.scheduler.Shutdown()
	if err != nil {
		return err
	}
	return nil
}
