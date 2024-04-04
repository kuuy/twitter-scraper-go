package users

import (
  "context"
  "encoding/json"
  "fmt"
  "log"
  "time"

  "github.com/go-redis/redis/v8"
  "github.com/hibiken/asynq"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/config"
  "scraper.local/twitter-scraper/models"
  "scraper.local/twitter-scraper/repositories"
  scrapersRepositories "scraper.local/twitter-scraper/repositories/scrapers"
)

type Posts struct {
  AnsqContext        *common.AnsqServerContext
  Repository         *scrapersRepositories.PostsRepository
  SessionsRepository *repositories.SessionsRepository
  UsersRepository    *repositories.UsersRepository
  TasksRepository    *repositories.TasksRepository
}

func NewPosts(ansqContext *common.AnsqServerContext) *Posts {
  h := &Posts{
    AnsqContext: ansqContext,
  }
  h.Repository = &scrapersRepositories.PostsRepository{
    Db: h.AnsqContext.Db,
  }
  h.SessionsRepository = &repositories.SessionsRepository{
    Db: h.AnsqContext.Db,
  }
  h.UsersRepository = &repositories.UsersRepository{
    Db: h.AnsqContext.Db,
  }
  h.Repository.SessionsRepository = h.SessionsRepository
  h.Repository.UsersRepository = h.UsersRepository
  h.Repository.PostsRepository = &repositories.PostsRepository{
    Db:   h.AnsqContext.Db,
    Nats: h.AnsqContext.Nats,
  }
  h.TasksRepository = &repositories.TasksRepository{
    Db: h.AnsqContext.Db,
  }
  return h
}

func (h *Posts) Flush(ctx context.Context, t *asynq.Task) error {
  var payload ProcessPayload
  json.Unmarshal(t.Payload(), &payload)

  mutex := common.NewMutex(
    h.AnsqContext.Rdb,
    h.AnsqContext.Ctx,
    fmt.Sprintf(config.LOCKS_TASKS_SCRAPERS_USERS_POSTS_FLUSH, payload.TaskID),
  )
  if !mutex.Lock(30 * time.Second) {
    return nil
  }
  defer mutex.Unlock()

  if task, err := h.TasksRepository.Find(payload.TaskID); err == nil {
    user, err := h.UsersRepository.Find(task.Params["user_id"].(string))
    if err != nil {
      log.Println("user can not be found", err)
      return nil
    }
    session := h.SessionsRepository.Current()
    if session == nil {
      log.Println("current session is empty")
      return nil
    }
    h.Repository.Process(session, user, task.Params)
  }
  return nil
}

func (h *Posts) Process(ctx context.Context, t *asynq.Task) error {
  var payload ProcessPayload
  json.Unmarshal(t.Payload(), &payload)

  mutex := common.NewMutex(
    h.AnsqContext.Rdb,
    h.AnsqContext.Ctx,
    fmt.Sprintf(config.LOCKS_TASKS_SCRAPERS_USERS_POSTS_PROCESS, payload.TaskID),
  )
  if !mutex.Lock(30 * time.Second) {
    return nil
  }
  defer mutex.Unlock()

  if task, err := h.TasksRepository.Find(payload.TaskID); err == nil {
    timestamp := time.Now().UnixMicro()

    user, err := h.UsersRepository.Find(task.Params["user_id"].(string))
    if err != nil {
      return err
    }
    var session *models.Session
    if _, ok := task.Params["cursors"]; ok {
      cursors := task.Params["cursors"].(map[string]interface{})
      for account, _ := range cursors {
        session, _ = h.SessionsRepository.Get(account)
        if session != nil && session.Status == 8 {
          break
        }
      }
      if session == nil {
        session = h.SessionsRepository.Special()
      }
    } else {
      session = h.SessionsRepository.Special()
    }
    if session == nil {
      log.Println("special session is empty")
      return nil
    }
    h.SessionsRepository.Update(session, "timestamp", timestamp)
    if cursor, count, err := h.Repository.Process(session, user, task.Params); err == nil {
      if cursor == "" {
        delete(task.Params, "cursors")
        h.AnsqContext.Rdb.ZRem(h.AnsqContext.Ctx, config.REDIS_KEY_TASKS_USERS_POSTS_TARGET, task.ID)
        h.TasksRepository.Updates(task, map[string]interface{}{
          "params": task.Params,
          "status": 2,
        })
        return nil
      }

      score, _ := h.AnsqContext.Rdb.ZScore(h.AnsqContext.Ctx, config.REDIS_KEY_TASKS_USERS_POSTS_TARGET, task.ID).Result()
      if count < 20 {
        if score == 0 || timestamp-int64(score) < config.SCRAPERS_CURSOR_WAITING_TIMEOUT {
          log.Println("waiting for cursor change", timestamp-int64(score))
          return nil
        }
      }

      if score > 0 {
        h.AnsqContext.Rdb.ZAdd(
          h.AnsqContext.Ctx,
          config.REDIS_KEY_TASKS_USERS_POSTS_TARGET,
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

func (h *Posts) Register() error {
  h.AnsqContext.Mux.HandleFunc(config.ASYNQ_JOBS_SCRAPERS_USERS_POSTS_FLUSH, h.Flush)
  h.AnsqContext.Mux.HandleFunc(config.ASYNQ_JOBS_SCRAPERS_USERS_POSTS_PROCESS, h.Process)
  return nil
}
