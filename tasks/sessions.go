package tasks

import (
  "log"
  "time"

  "github.com/hibiken/asynq"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/config"
  "scraper.local/twitter-scraper/queue/asynq/jobs"
)

type SessionsTask struct {
  Job         *jobs.Sessions
  AnsqContext *common.AnsqClientContext
}

func NewSessionsTask(ansqContext *common.AnsqClientContext) *SessionsTask {
  return &SessionsTask{
    AnsqContext: ansqContext,
  }
}

func (t *SessionsTask) Flush() (err error) {
  log.Println("tasks sessions flush")
  if job, err := t.Job.Flush(); err == nil {
    t.AnsqContext.Conn.Enqueue(
      job,
      asynq.Queue(config.ASYNQ_QUEUE_SESSIONS),
      asynq.MaxRetry(0),
      asynq.Timeout(5*time.Minute),
    )
  }
  return
}
