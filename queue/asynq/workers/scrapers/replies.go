package scrapers

import (
  "context"
  "encoding/json"
  "errors"
  "fmt"
  "log"
  "time"

  "gorm.io/gorm"

  "github.com/go-redis/redis/v8"
  "github.com/hibiken/asynq"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/config"
  "scraper.local/twitter-scraper/models"
  "scraper.local/twitter-scraper/repositories"
  scrapersRepositories "scraper.local/twitter-scraper/repositories/scrapers"
)

type Replies struct {
  AnsqContext        *common.AnsqServerContext
  Repository         *scrapersRepositories.RepliesRepository
  UsersRepository    *repositories.UsersRepository
  SessionsRepository *repositories.SessionsRepository
  PostsRepository    *repositories.PostsRepository
  TasksRepository    *repositories.TasksRepository
}

func NewReplies(ansqContext *common.AnsqServerContext) *Replies {
  h := &Replies{
    AnsqContext: ansqContext,
  }
  h.Repository = &scrapersRepositories.RepliesRepository{
    Db: h.AnsqContext.Db,
  }
  h.UsersRepository = &repositories.UsersRepository{
    Db: h.AnsqContext.Db,
  }
  h.SessionsRepository = &repositories.SessionsRepository{
    Db: h.AnsqContext.Db,
  }
  h.PostsRepository = &repositories.PostsRepository{
    Db: h.AnsqContext.Db,
  }
  h.Repository.SessionsRepository = h.SessionsRepository
  h.Repository.UsersRepository = &repositories.UsersRepository{
    Db:   h.AnsqContext.Db,
    Nats: h.AnsqContext.Nats,
  }
  h.Repository.RepliesRepository = &repositories.RepliesRepository{
    Db:   h.AnsqContext.Db,
    Nats: h.AnsqContext.Nats,
  }
  h.TasksRepository = &repositories.TasksRepository{
    Db: h.AnsqContext.Db,
  }
  return h
}

func (h *Replies) Init(ctx context.Context, t *asynq.Task) error {
  var payload InitPayload
  json.Unmarshal(t.Payload(), &payload)

  mutex := common.NewMutex(
    h.AnsqContext.Rdb,
    h.AnsqContext.Ctx,
    fmt.Sprintf(config.LOCKS_TASKS_SCRAPERS_REPLIES_INIT, payload.UserID),
  )
  if !mutex.Lock(30 * time.Second) {
    return nil
  }
  defer mutex.Unlock()

  user, err := h.UsersRepository.Find(payload.UserID)
  if err != nil {
    log.Println("user not exists", payload.UserID)
    return nil
  }
  conditions := map[string]interface{}{
    "user_id": user.ID,
  }
  posts := h.PostsRepository.Ranking(
    []string{"id", "timestamp"},
    conditions,
    "timestamp",
    -1,
    10000,
  )
  for _, post := range posts {
    name := fmt.Sprintf("%v@replies", post.ID)
    action := config.TASK_ACTION_SCRAPERS_REPLIES
    params := map[string]interface{}{
      "post_id": post.ID,
    }
    h.TasksRepository.Apply(name, action, params)
  }
  return nil
}

func (h *Replies) Flush(ctx context.Context, t *asynq.Task) error {
  var payload ProcessPayload
  json.Unmarshal(t.Payload(), &payload)

  mutex := common.NewMutex(
    h.AnsqContext.Rdb,
    h.AnsqContext.Ctx,
    fmt.Sprintf(config.LOCKS_TASKS_SCRAPERS_REPLIES_FLUSH, payload.TaskID),
  )
  if !mutex.Lock(30 * time.Second) {
    return nil
  }
  defer mutex.Unlock()

  if task, err := h.TasksRepository.Find(payload.TaskID); err == nil {
    if _, ok := task.Params["post_id"]; !ok {
      return nil
    }
    postID := task.Params["post_id"].(string)
    post, err := h.PostsRepository.Find(postID)
    if errors.Is(err, gorm.ErrRecordNotFound) {
      h.TasksRepository.Delete(task.ID)
      return nil
    }
    if err != nil {
      return err
    }
    session := h.SessionsRepository.Current()
    if session == nil {
      log.Println("current session is empty")
      return nil
    }
    h.Repository.Process(session, post, task.Params)
  }
  return nil
}

func (h *Replies) Process(ctx context.Context, t *asynq.Task) error {
  var payload ProcessPayload
  json.Unmarshal(t.Payload(), &payload)

  mutex := common.NewMutex(
    h.AnsqContext.Rdb,
    h.AnsqContext.Ctx,
    fmt.Sprintf(config.LOCKS_TASKS_SCRAPERS_REPLIES_PROCESS, payload.TaskID),
  )
  if !mutex.Lock(30 * time.Second) {
    return nil
  }
  defer mutex.Unlock()

  if task, err := h.TasksRepository.Find(payload.TaskID); err == nil {
    timestamp := time.Now().UnixMicro()

    postID := task.Params["post_id"].(string)
    post, err := h.PostsRepository.Find(postID)
    if errors.Is(err, gorm.ErrRecordNotFound) {
      h.TasksRepository.Delete(task.ID)
      return nil
    }
    if err != nil {
      return err
    }
    var session *models.Session
    if _, ok := task.Params["cursors"]; ok {
      cursors := task.Params["cursors"].(map[string]interface{})
      for account, _ := range cursors {
        session, _ = h.SessionsRepository.Get(account)
        if session != nil && session.Status == 1 {
          break
        }
      }
      if session == nil {
        session = h.SessionsRepository.Current()
      }
    } else {
      session = h.SessionsRepository.Current()
    }
    if session == nil {
      log.Println("current session is empty")
      return nil
    }
    h.SessionsRepository.Update(session, "timestamp", timestamp)
    if cursor, count, err := h.Repository.Process(session, post, task.Params); err == nil {
      if cursor == "" {
        delete(task.Params, "cursors")
        h.AnsqContext.Rdb.ZRem(h.AnsqContext.Ctx, config.REDIS_KEY_TASKS_REPLIES_TARGET, task.ID)
        h.TasksRepository.Updates(task, map[string]interface{}{
          "params": task.Params,
          "status": 2,
        })
        return nil
      }

      score, _ := h.AnsqContext.Rdb.ZScore(h.AnsqContext.Ctx, config.REDIS_KEY_TASKS_REPLIES_TARGET, task.ID).Result()
      if count < 10 {
        if score == 0 || timestamp-int64(score) < config.SCRAPERS_CURSOR_WAITING_TIMEOUT {
          log.Println("waiting for cursor change", timestamp-int64(score))
          return nil
        }
      }

      if score > 0 {
        h.AnsqContext.Rdb.ZAdd(
          h.AnsqContext.Ctx,
          config.REDIS_KEY_TASKS_REPLIES_TARGET,
          &redis.Z{
            float64(timestamp),
            task.ID,
          },
        )
      }

      cursors := make(map[string]interface{})
      if _, ok := task.Params["cursors"]; ok {
        cursors = task.Params["cursors"].(map[string]interface{})
      }
      cursors[session.Account] = cursor
      task.Params["cursors"] = cursors
      h.TasksRepository.Update(task, "params", task.Params)
    }
  }
  return nil
}

func (h *Replies) Register() error {
  h.AnsqContext.Mux.HandleFunc(config.ASYNQ_JOBS_SCRAPERS_REPLIES_INIT, h.Init)
  h.AnsqContext.Mux.HandleFunc(config.ASYNQ_JOBS_SCRAPERS_REPLIES_FLUSH, h.Flush)
  h.AnsqContext.Mux.HandleFunc(config.ASYNQ_JOBS_SCRAPERS_REPLIES_PROCESS, h.Process)
  return nil
}
