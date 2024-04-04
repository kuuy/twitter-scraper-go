package scrapers

import (
  "scraper.local/twitter-scraper/common"
  workers "scraper.local/twitter-scraper/queue/asynq/workers/scrapers/users"
)

type Users struct {
  AnsqContext *common.AnsqServerContext
}

func NewUsers(ansqContext *common.AnsqServerContext) *Users {
  return &Users{
    AnsqContext: ansqContext,
  }
}

func (h *Users) Register() error {
  workers.NewPosts(h.AnsqContext).Register()
  return nil
}
