package jobs

import (
  "scraper.local/twitter-scraper/config"
  "github.com/hibiken/asynq"
)

type Sessions struct{}

func (h *Sessions) Flush() (*asynq.Task, error) {
  return asynq.NewTask(config.ASYNQ_JOBS_SESSIONS_FLUSH, nil), nil
}
