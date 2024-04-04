package scrapers

import (
  "encoding/json"
  "scraper.local/twitter-scraper/config"
  "github.com/hibiken/asynq"
)

type Replies struct{}

func (h *Replies) Init(userID string) (*asynq.Task, error) {
  payload, err := json.Marshal(InitPayload{userID})
  if err != nil {
    return nil, err
  }
  return asynq.NewTask(config.ASYNQ_JOBS_SCRAPERS_REPLIES_INIT, payload), nil
}

func (h *Replies) Flush(taskID string) (*asynq.Task, error) {
  payload, err := json.Marshal(ProcessPayload{taskID})
  if err != nil {
    return nil, err
  }
  return asynq.NewTask(config.ASYNQ_JOBS_SCRAPERS_REPLIES_FLUSH, payload), nil
}

func (h *Replies) Process(taskID string) (*asynq.Task, error) {
  payload, err := json.Marshal(ProcessPayload{taskID})
  if err != nil {
    return nil, err
  }
  return asynq.NewTask(config.ASYNQ_JOBS_SCRAPERS_REPLIES_PROCESS, payload), nil
}
