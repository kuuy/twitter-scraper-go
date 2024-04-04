package workers

import (
  "context"
  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/config"
  "scraper.local/twitter-scraper/repositories"
  "github.com/hibiken/asynq"
)

type Sessions struct {
  AnsqContext *common.AnsqServerContext
  Repository  *repositories.SessionsRepository
}

func NewSessions(ansqContext *common.AnsqServerContext) *Sessions {
  h := &Sessions{
    AnsqContext: ansqContext,
  }
  h.Repository = &repositories.SessionsRepository{
    Db: h.AnsqContext.Db,
  }
  return h
}

func (h *Sessions) Flush(ctx context.Context, t *asynq.Task) error {
  if session := h.Repository.Current(); session != nil {
    h.Repository.Flush(session)
  }
  return nil
}

func (h *Sessions) Register() error {
  h.AnsqContext.Mux.HandleFunc(config.ASYNQ_JOBS_SESSIONS_FLUSH, h.Flush)
  return nil
}
