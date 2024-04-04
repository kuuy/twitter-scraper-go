package scrapers

import (
  "context"
  "errors"
  "fmt"
  "log"
  "strconv"
  "time"

  "github.com/go-redis/redis/v8"
  "github.com/nats-io/nats.go"
  "github.com/urfave/cli/v2"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/config"
  "scraper.local/twitter-scraper/models"
  "scraper.local/twitter-scraper/repositories"
  scrapersRepositories "scraper.local/twitter-scraper/repositories/scrapers"
)

type PostsHandler struct {
  Db                 *gorm.DB
  Rdb                *redis.Client
  Ctx                context.Context
  Nats               *nats.Conn
  Repository         *repositories.TasksRepository
  SessionsRepository *repositories.SessionsRepository
  UsersRepository    *repositories.UsersRepository
  ScrapersRepository *scrapersRepositories.PostsRepository
}

func NewPostsCommand() *cli.Command {
  var h PostsHandler
  return &cli.Command{
    Name:  "posts",
    Usage: "",
    Before: func(c *cli.Context) error {
      h = PostsHandler{
        Db:   common.NewDB(),
        Rdb:  common.NewRedis(),
        Ctx:  context.Background(),
        Nats: common.NewNats(),
      }
      h.Repository = &repositories.TasksRepository{
        Db: h.Db,
      }
      h.SessionsRepository = &repositories.SessionsRepository{
        Db: h.Db,
      }
      h.UsersRepository = &repositories.UsersRepository{
        Db:   h.Db,
        Nats: h.Nats,
      }
      h.ScrapersRepository = &scrapersRepositories.PostsRepository{
        Db: h.Db,
      }
      h.ScrapersRepository.SessionsRepository = h.SessionsRepository
      h.ScrapersRepository.UsersRepository = h.UsersRepository
      h.ScrapersRepository.PostsRepository = &repositories.PostsRepository{
        Db:   h.Db,
        Nats: h.Nats,
      }
      return nil
    },
    Subcommands: []*cli.Command{
      {
        Name:  "apply",
        Usage: "",
        Action: func(c *cli.Context) (err error) {
          account := c.Args().Get(0)
          if account == "" {
            log.Fatal("twitter account can not be empty")
            return nil
          }
          userID, err := strconv.ParseInt(c.Args().Get(1), 10, 64)
          if err != nil {
            log.Fatal("twitter user_id not valid")
            return nil
          }
          if err = h.Apply(account, userID); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return
        },
      },
      {
        Name:  "flush",
        Usage: "",
        Action: func(c *cli.Context) error {
          limit, _ := strconv.Atoi(c.Args().Get(0))
          if limit < 20 {
            limit = 20
          }
          if err := h.Flush(limit); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return nil
        },
      },
      {
        Name:  "process",
        Usage: "",
        Action: func(c *cli.Context) error {
          limit, _ := strconv.Atoi(c.Args().Get(0))
          if limit < 20 {
            limit = 20
          }
          if err := h.Process(limit); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return nil
        },
      },
    },
  }
}

func (h *PostsHandler) Apply(account string, userID int64) error {
  log.Println(fmt.Sprintf("tasks posts apply..."))
  user, err := h.UsersRepository.GetByUserID(userID)
  if errors.Is(err, gorm.ErrRecordNotFound) {
    user.ID, _ = h.UsersRepository.Create(
      account,
      userID,
      "",
      "",
      "",
      0,
      0,
      0,
      0,
      0,
      0,
    )
  }
  name := fmt.Sprintf("%v@posts", user.ID)
  action := config.TASK_ACTION_SCRAPERS_POSTS
  params := map[string]interface{}{
    "user_id": user.ID,
  }
  return h.Repository.Apply(name, action, params)
}

func (h *PostsHandler) Flush(limit int) error {
  log.Println(fmt.Sprintf("tasks posts flushing..."))
  tasks := h.Repository.Ranking(
    []string{"id", "params", "timestamp"},
    map[string]interface{}{
      "action": config.TASK_ACTION_SCRAPERS_POSTS,
      "status": 2,
    },
    "timestamp",
    1,
    limit,
  )
  for _, task := range tasks {
    mutex := common.NewMutex(
      h.Rdb,
      h.Ctx,
      fmt.Sprintf(config.LOCKS_TASKS_SCRAPERS_POSTS_FLUSH, task.ID),
    )
    if !mutex.Lock(30 * time.Second) {
      return nil
    }

    timestamp := time.Now().UnixMicro()
    if timestamp-task.Timestamp < 30000000 {
      log.Println("waiting for next process")
      mutex.Unlock()
      continue
    }
    h.Repository.Update(task, "timestamp", timestamp)
    if _, ok := task.Params["user_id"]; !ok {
      mutex.Unlock()
      return errors.New("posts user_id is empty")
    }
    user, err := h.UsersRepository.Find(task.Params["user_id"].(string))
    if errors.Is(err, gorm.ErrRecordNotFound) {
      h.Repository.Delete(task.ID)
    }
    if err != nil {
      mutex.Unlock()
      return err
    }
    session := h.SessionsRepository.Current()
    if session.ID == "" {
      mutex.Unlock()
      return errors.New("current session is empty")
    }
    h.SessionsRepository.Update(session, "timestamp", timestamp)
    if cursor, count, err := h.ScrapersRepository.Process(session, user, task.Params); err == nil {
      log.Println("scrapers posts flush result", cursor, count)
    } else {
      log.Println("error", err)
    }

    mutex.Unlock()
  }
  return nil
}

func (h *PostsHandler) Process(limit int) error {
  log.Println(fmt.Sprintf("tasks posts processing..."))
  count, _ := h.Rdb.ZCard(h.Ctx, config.REDIS_KEY_TASKS_POSTS_TARGET).Result()
  conditions := make(map[string]interface{})
  if count < config.SCRAPERS_POSTS_TARGET_LIMIT {
    conditions["action"] = config.TASK_ACTION_SCRAPERS_POSTS
  } else {
    conditions["ids"], _ = h.Rdb.ZRange(h.Ctx, config.REDIS_KEY_TASKS_POSTS_TARGET, 0, -1).Result()
  }
  tasks := h.Repository.Ranking(
    []string{"id", "params", "timestamp"},
    conditions,
    "timestamp",
    1,
    limit,
  )
  for _, task := range tasks {
    mutex := common.NewMutex(
      h.Rdb,
      h.Ctx,
      fmt.Sprintf(config.LOCKS_TASKS_SCRAPERS_POSTS_PROCESS, task.ID),
    )
    if !mutex.Lock(30 * time.Second) {
      return nil
    }
    defer mutex.Unlock()

    timestamp := time.Now().UnixMicro()
    score, _ := h.Rdb.ZScore(h.Ctx, config.REDIS_KEY_TASKS_POSTS_TARGET, task.ID).Result()
    if score == 0 && count < config.SCRAPERS_POSTS_TARGET_LIMIT {
      h.Rdb.ZAdd(
        h.Ctx,
        config.REDIS_KEY_TASKS_POSTS_TARGET,
        &redis.Z{
          float64(timestamp),
          task.ID,
        },
      )
      count++
    }

    if timestamp-task.Timestamp < 30000000 {
      log.Println("waiting for next process")
      continue
    }

    h.Repository.Update(task, "timestamp", timestamp)
    if _, ok := task.Params["user_id"]; !ok {
      log.Println("task params user_id is empty", task.ID)
      continue
    }
    user, err := h.UsersRepository.Find(task.Params["user_id"].(string))
    if err != nil {
      log.Println("user not found", task.Params["user_id"])
      continue
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
      return errors.New("special session is empty")
    }
    h.SessionsRepository.Update(session, "timestamp", timestamp)
    if cursor, count, err := h.ScrapersRepository.Process(session, user, task.Params); err == nil {
      if cursor == "" {
        delete(task.Params, "cursors")
        h.Rdb.ZRem(h.Ctx, config.REDIS_KEY_TASKS_POSTS_TARGET, task.ID)
        h.Repository.Updates(task, map[string]interface{}{
          "params": task.Params,
          "status": 2,
        })
        continue
      }

      if count < 20 {
        if score == 0 || timestamp-int64(score) < config.SCRAPERS_CURSOR_WAITING_TIMEOUT {
          log.Println("waiting for cursor change", timestamp-int64(score), config.SCRAPERS_CURSOR_WAITING_TIMEOUT)
          continue
        }
      }

      if score > 0 {
        h.Rdb.ZAdd(
          h.Ctx,
          config.REDIS_KEY_TASKS_POSTS_TARGET,
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
      h.Repository.Updates(task, map[string]interface{}{
        "params": task.Params,
        "status": 1,
      })
    } else {
      log.Println("error", err)
    }
  }

  return nil
}
