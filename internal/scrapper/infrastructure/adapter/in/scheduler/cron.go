package scheduler

import (
	"github.com/go-co-op/gocron/v2"
	"time"
)

type Cron struct {
	scheduler gocron.Scheduler
}

func NewCron() (*Cron, error) {
	s, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}

	cron := &Cron{
		scheduler: s,
	}

	return cron, nil
}

func (cron *Cron) Schedule(dur time.Duration, function any, parameters ...any) error {
	_, err := cron.scheduler.NewJob(
		gocron.DurationJob(dur),
		gocron.NewTask(function, parameters...),
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
	err := cron.scheduler.Shutdown()
	if err != nil {
		return err
	}
	return nil
}
