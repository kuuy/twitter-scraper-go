package scrapers

import (
  "encoding/json"
  "scraper.local/twitter-scraper/config"
  "github.com/hibiken/asynq"
)

type Posts struct{}

func (h *Posts) Flush(taskID string) (*asynq.Task, error) {
  payload, err := json.Marshal(ProcessPayload{taskID})
  if err != nil {
    return nil, err
  }
  return asynq.NewTask(config.ASYNQ_JOBS_SCRAPERS_POSTS_FLUSH, payload), nil
}

func (h *Posts) Process(taskID string) (*asynq.Task, error) {
  payload, err := json.Marshal(ProcessPayload{taskID})
  if err != nil {
    return nil, err
  }
  return asynq.NewTask(config.ASYNQ_JOBS_SCRAPERS_POSTS_PROCESS, payload), nil
}
