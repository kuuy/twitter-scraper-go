package workers

import (
  "scraper.local/twitter-scraper/common"
  workers "scraper.local/twitter-scraper/queue/asynq/workers/scrapers"
)

type Scrapers struct {
  AnsqContext *common.AnsqServerContext
}

func NewScrapers(ansqContext *common.AnsqServerContext) *Scrapers {
  return &Scrapers{
    AnsqContext: ansqContext,
  }
}

func (h *Scrapers) Register() error {
  workers.NewPosts(h.AnsqContext).Register()
  workers.NewReplies(h.AnsqContext).Register()
  workers.NewUsers(h.AnsqContext).Register()
  return nil
}
