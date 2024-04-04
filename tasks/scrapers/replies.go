package scrapers

import (
  "scraper.local/twitter-scraper/models"
  "github.com/go-redis/redis/v8"
  "log"
  "time"

  "github.com/hibiken/asynq"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/config"
  jobs "scraper.local/twitter-scraper/queue/asynq/jobs/scrapers"
  "scraper.local/twitter-scraper/repositories"
)

type RepliesTask struct {
  Job             *jobs.Replies
  AnsqContext     *common.AnsqClientContext
  TasksRepository *repositories.TasksRepository
}

type TopRepliesUsers struct {
  UserID       string `json:"user_id"`
  RepliesCount string `json:"replies_count"`
}

func NewRepliesTask(ansqContext *common.AnsqClientContext) *RepliesTask {
  return &RepliesTask{
    AnsqContext: ansqContext,
    TasksRepository: &repositories.TasksRepository{
      Db: ansqContext.Db,
    },
  }
}

func (t *RepliesTask) Init(limit int) (err error) {
  log.Println("tasks scrapers replies init")
  var entities []TopRepliesUsers
  t.AnsqContext.Db.Model(&models.Reply{}).Select(
    "user_id, count(id) as replies_count",
  ).Where(
    "status",
    1,
  ).Group(
    "user_id",
  ).Limit(
    limit,
  ).Scan(&entities)
  for _, entity := range entities {
    if job, err := t.Job.Init(entity.UserID); err == nil {
      t.AnsqContext.Conn.Enqueue(
        job,
        asynq.Queue(config.ASYNQ_QUEUE_SCRAPERS_REPLIES),
        asynq.MaxRetry(0),
        asynq.Timeout(5*time.Minute),
      )
    }
  }
  return
}

func (t *RepliesTask) Flush(limit int) (err error) {
  log.Println("tasks scrapers replies flush")
  tasks := t.TasksRepository.Ranking(
    []string{"id", "params", "timestamp"},
    map[string]interface{}{
      "action": config.TASK_ACTION_SCRAPERS_REPLIES,
      "status": 2,
    },
    "timestamp",
    1,
    limit,
  )
  for _, task := range tasks {
    timestamp := time.Now().UnixMicro()
    if timestamp-task.Timestamp < 30000000 {
      continue
    }
    if job, err := t.Job.Flush(task.ID); err == nil {
      t.AnsqContext.Conn.Enqueue(
        job,
        asynq.Queue(config.ASYNQ_QUEUE_SCRAPERS_REPLIES),
        asynq.MaxRetry(0),
        asynq.Timeout(5*time.Minute),
      )
    }
    t.TasksRepository.Update(task, "timestamp", timestamp)
  }
  return
}

func (t *RepliesTask) Process(limit int) (err error) {
  log.Println("tasks scrapers replies process")
  count, _ := t.AnsqContext.Rdb.ZCard(t.AnsqContext.Ctx, config.REDIS_KEY_TASKS_REPLIES_TARGET).Result()
  conditions := make(map[string]interface{})
  if count < config.SCRAPERS_REPLIES_TARGET_LIMIT {
    conditions["action"] = config.TASK_ACTION_SCRAPERS_REPLIES
  } else {
    conditions["ids"], _ = t.AnsqContext.Rdb.ZRange(t.AnsqContext.Ctx, config.REDIS_KEY_TASKS_REPLIES_TARGET, 0, -1).Result()
  }
  tasks := t.TasksRepository.Ranking(
    []string{"id", "params", "timestamp"},
    conditions,
    "timestamp",
    1,
    limit,
  )
  for _, task := range tasks {
    timestamp := time.Now().UnixMicro()

    score, _ := t.AnsqContext.Rdb.ZScore(t.AnsqContext.Ctx, config.REDIS_KEY_TASKS_REPLIES_TARGET, task.ID).Result()
    if score == 0 && count < config.SCRAPERS_REPLIES_TARGET_LIMIT {
      t.AnsqContext.Rdb.ZAdd(
        t.AnsqContext.Ctx,
        config.REDIS_KEY_TASKS_REPLIES_TARGET,
        &redis.Z{
          float64(timestamp),
          task.ID,
        },
      )
      count++
    }

    if timestamp-task.Timestamp < 30000000 {
      continue
    }

    if job, err := t.Job.Process(task.ID); err == nil {
      t.AnsqContext.Conn.Enqueue(
        job,
        asynq.Queue(config.ASYNQ_QUEUE_SCRAPERS_REPLIES),
        asynq.MaxRetry(0),
        asynq.Timeout(5*time.Minute),
      )
    }

    t.TasksRepository.Update(task, "timestamp", timestamp)
  }
  return
}
