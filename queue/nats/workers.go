package nats

import (
  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/queue/nats/workers"
)

type Workers struct {
  NatsContext *common.NatsContext
}

func NewWorkers(natsContext *common.NatsContext) *Workers {
  return &Workers{
    NatsContext: natsContext,
  }
}

func (h *Workers) Subscribe() error {
  workers.NewTasks(h.NatsContext).Subscribe()
  return nil
}
