package users

import (
  "log"
  "time"

  "github.com/go-redis/redis/v8"
  "github.com/hibiken/asynq"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/config"
  jobs "scraper.local/twitter-scraper/queue/asynq/jobs/scrapers/users"
  "scraper.local/twitter-scraper/repositories"
)

type PostsTask struct {
  Job             *jobs.Posts
  AnsqContext     *common.AnsqClientContext
  TasksRepository *repositories.TasksRepository
}

func NewPostsTask(ansqContext *common.AnsqClientContext) *PostsTask {
  return &PostsTask{
    AnsqContext: ansqContext,
    TasksRepository: &repositories.TasksRepository{
      Db: ansqContext.Db,
    },
  }
}

func (t *PostsTask) Flush(limit int) (err error) {
  log.Println("tasks scrapers users posts flush")
  tasks := t.TasksRepository.Ranking(
    []string{"id", "params", "timestamp"},
    map[string]interface{}{
      "action": config.TASK_ACTION_SCRAPERS_USERS_POSTS,
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
        asynq.Queue(config.ASYNQ_QUEUE_SCRAPERS_USERS_POSTS),
        asynq.MaxRetry(0),
        asynq.Timeout(5*time.Minute),
      )
    }
    t.TasksRepository.Update(task, "timestamp", timestamp)
  }
  return
}

func (t *PostsTask) Process(limit int) (err error) {
  log.Println("tasks scrapers users posts process")
  count, _ := t.AnsqContext.Rdb.ZCard(t.AnsqContext.Ctx, config.REDIS_KEY_TASKS_USERS_POSTS_TARGET).Result()
  conditions := make(map[string]interface{})
  if count < config.SCRAPERS_USERS_POSTS_TARGET_LIMIT {
    conditions["action"] = config.TASK_ACTION_SCRAPERS_USERS_POSTS
  } else {
    conditions["ids"], _ = t.AnsqContext.Rdb.ZRange(t.AnsqContext.Ctx, config.REDIS_KEY_TASKS_USERS_POSTS_TARGET, 0, -1).Result()
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

    score, _ := t.AnsqContext.Rdb.ZScore(t.AnsqContext.Ctx, config.REDIS_KEY_TASKS_USERS_POSTS_TARGET, task.ID).Result()
    if score == 0 && count < config.SCRAPERS_USERS_POSTS_TARGET_LIMIT {
      t.AnsqContext.Rdb.ZAdd(
        t.AnsqContext.Ctx,
        config.REDIS_KEY_TASKS_USERS_POSTS_TARGET,
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
        asynq.Queue(config.ASYNQ_QUEUE_SCRAPERS_USERS_POSTS),
        asynq.MaxRetry(0),
        asynq.Timeout(5*time.Minute),
      )
    }

    t.TasksRepository.Update(task, "timestamp", timestamp)
  }
  return
}
