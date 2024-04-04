package asynq

import (
  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/queue/asynq/workers"
)

type Workers struct {
  AnsqContext *common.AnsqServerContext
}

func NewWorkers(ansqContext *common.AnsqServerContext) *Workers {
  return &Workers{
    AnsqContext: ansqContext,
  }
}

func (h *Workers) Register() error {
  workers.NewScrapers(h.AnsqContext).Register()
  workers.NewSessions(h.AnsqContext).Register()
  return nil
}
