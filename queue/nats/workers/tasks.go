package workers

import (
  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/queue/nats/workers/tasks"
)

type Tasks struct {
  NatsContext *common.NatsContext
}

func NewTasks(natsContext *common.NatsContext) *Tasks {
  return &Tasks{
    NatsContext: natsContext,
  }
}

func (h *Tasks) Subscribe() error {
  tasks.NewReplies(h.NatsContext).Subscribe()
  tasks.NewMedia(h.NatsContext).Subscribe()
  return nil
}
