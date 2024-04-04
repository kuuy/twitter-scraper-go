package tasks

import (
  "scraper.local/twitter-scraper/common"
  tasks "scraper.local/twitter-scraper/queue/nats/workers/tasks/media"
)

type Media struct {
  NatsContext *common.NatsContext
}

func NewMedia(natsContext *common.NatsContext) *Media {
  return &Media{
    NatsContext: natsContext,
  }
}

func (h *Media) Subscribe() error {
  tasks.NewUsers(h.NatsContext).Subscribe()
  tasks.NewPosts(h.NatsContext).Subscribe()
  tasks.NewReplies(h.NatsContext).Subscribe()
  return nil
}
