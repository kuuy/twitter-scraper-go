package scrapers

import (
  "context"
  "errors"
  "scraper.local/twitter-scraper/models"
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
  "scraper.local/twitter-scraper/repositories"
  scrapersRepositories "scraper.local/twitter-scraper/repositories/scrapers"
)

type RepliesHandler struct {
  Db                      *gorm.DB
  Rdb                     *redis.Client
  Ctx                     context.Context
  Nats                    *nats.Conn
  Repository              *repositories.TasksRepository
  UsersRepository         *repositories.UsersRepository
  SessionsRepository      *repositories.SessionsRepository
  PostsRepository         *repositories.PostsRepository
  PostsScrapersRepository *scrapersRepositories.PostsRepository
  ScrapersRepository      *scrapersRepositories.RepliesRepository
}

type TopRepliesUsers struct {
  UserID       string `json:"user_id"`
  RepliesCount string `json:"replies_count"`
}

func NewRepliesCommand() *cli.Command {
  var h RepliesHandler
  return &cli.Command{
    Name:  "replies",
    Usage: "",
    Before: func(c *cli.Context) error {
      h = RepliesHandler{
        Db:   common.NewDB(),
        Rdb:  common.NewRedis(),
        Ctx:  context.Background(),
        Nats: common.NewNats(),
      }
      h.Repository = &repositories.TasksRepository{
        Db: h.Db,
      }
      h.UsersRepository = &repositories.UsersRepository{
        Db: h.Db,
      }
      h.SessionsRepository = &repositories.SessionsRepository{
        Db: h.Db,
      }
      h.PostsRepository = &repositories.PostsRepository{
        Db: h.Db,
      }
      h.PostsScrapersRepository = &scrapersRepositories.PostsRepository{
        Db: h.Db,
      }
      h.ScrapersRepository = &scrapersRepositories.RepliesRepository{
        Db: h.Db,
      }
      h.ScrapersRepository.SessionsRepository = h.SessionsRepository
      h.ScrapersRepository.UsersRepository = &repositories.UsersRepository{
        Db:   h.Db,
        Nats: h.Nats,
      }
      h.ScrapersRepository.RepliesRepository = &repositories.RepliesRepository{
        Db:   h.Db,
        Nats: h.Nats,
      }
      return nil
    },
    Subcommands: []*cli.Command{
      {
        Name:  "init",
        Usage: "",
        Action: func(c *cli.Context) (err error) {
          limit, _ := strconv.Atoi(c.Args().Get(0))
          if limit < 20 {
            limit = 20
          }
          if err := h.Init(limit); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return
        },
      },
      {
        Name:  "apply",
        Usage: "",
        Action: func(c *cli.Context) error {
          postId := c.Args().Get(0)
          if postId == "" {
            log.Fatal("post id can not be empty")
            return nil
          }
          if err := h.Apply(postId); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return nil
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

func (h *RepliesHandler) Init(limit int) (err error) {
  log.Println(fmt.Sprintf("tasks replies init..."))
  var entities []TopRepliesUsers
  h.Db.Model(&models.Reply{}).Select(
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
    user, err := h.UsersRepository.Find(entity.UserID)
    if err != nil {
      log.Println("user not exists", entity.UserID)
      continue
    }
    conditions := map[string]interface{}{
      "user_id": user.ID,
    }
    posts := h.PostsRepository.Ranking(
      []string{"id", "timestamp"},
      conditions,
      "timestamp",
      -1,
      limit,
    )
    for _, post := range posts {
      name := fmt.Sprintf("%v@replies", post.ID)
      action := config.TASK_ACTION_SCRAPERS_REPLIES
      params := map[string]interface{}{
        "post_id": post.ID,
      }
      h.Repository.Apply(name, action, params)
    }
  }
  return nil
}

func (h *RepliesHandler) Apply(postID string) error {
  log.Println(fmt.Sprintf("tasks replies apply..."))
  name := fmt.Sprintf("%v@replies", postID)
  action := config.TASK_ACTION_SCRAPERS_REPLIES
  params := map[string]interface{}{
    "post_id": postID,
  }
  return h.Repository.Apply(name, action, params)
}

func (h *RepliesHandler) Flush(limit int) error {
  log.Println(fmt.Sprintf("tasks replies flushing..."))
  tasks := h.Repository.Ranking(
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
    mutex := common.NewMutex(
      h.Rdb,
      h.Ctx,
      fmt.Sprintf(config.LOCKS_TASKS_SCRAPERS_REPLIES_FLUSH, task.ID),
    )
    if !mutex.Lock(30 * time.Second) {
      continue
    }

    timestamp := time.Now().UnixMicro()
    if timestamp-task.Timestamp < 30000000 {
      log.Println("waiting for next process")
      mutex.Unlock()
      continue
    }
    h.Repository.Update(task, "timestamp", timestamp)
    postID := task.Params["post_id"].(string)
    post, err := h.PostsRepository.Find(postID)
    if err != nil {
      mutex.Unlock()
      continue
    }
    session := h.SessionsRepository.Current()
    if session == nil {
      mutex.Unlock()
      return errors.New("current session is empty")
    }
    h.SessionsRepository.Update(session, "timestamp", timestamp)
    if cursor, count, err := h.ScrapersRepository.Process(session, post, task.Params); err == nil {
      log.Println("scrapers replies flush result", cursor, count)
    } else {
      log.Println("error", err)
    }

    mutex.Unlock()
  }
  return nil
}

func (h *RepliesHandler) Process(limit int) error {
  log.Println(fmt.Sprintf("tasks replies processing..."))
  count, _ := h.Rdb.ZCard(h.Ctx, config.REDIS_KEY_TASKS_REPLIES_TARGET).Result()
  conditions := make(map[string]interface{})
  if count < config.SCRAPERS_REPLIES_TARGET_LIMIT {
    conditions["action"] = config.TASK_ACTION_SCRAPERS_REPLIES
  } else {
    conditions["ids"], _ = h.Rdb.ZRange(h.Ctx, config.REDIS_KEY_TASKS_REPLIES_TARGET, 0, -1).Result()
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
      fmt.Sprintf(config.LOCKS_TASKS_SCRAPERS_REPLIES_PROCESS, task.ID),
    )
    if !mutex.Lock(30 * time.Second) {
      continue
    }

    timestamp := time.Now().UnixMicro()
    score, _ := h.Rdb.ZScore(h.Ctx, config.REDIS_KEY_TASKS_REPLIES_TARGET, task.ID).Result()
    if score == 0 && count < config.SCRAPERS_REPLIES_TARGET_LIMIT {
      h.Rdb.ZAdd(
        h.Ctx,
        config.REDIS_KEY_TASKS_REPLIES_TARGET,
        &redis.Z{
          float64(timestamp),
          task.ID,
        },
      )
      count++
    }

    if timestamp-task.Timestamp < 30000000 {
      log.Println("waiting for next process")
      mutex.Unlock()
      continue
    }
    h.Repository.Update(task, "timestamp", timestamp)
    if _, ok := task.Params["post_id"]; !ok {
      mutex.Unlock()
      continue
    }
    postID := task.Params["post_id"].(string)
    post, err := h.PostsRepository.Find(postID)
    if err != nil {
      mutex.Unlock()
      continue
    }
    var session *models.Session
    if _, ok := task.Params["cursors"]; ok {
      cursors := task.Params["cursors"].(map[string]interface{})
      for account, _ := range cursors {
        session, _ = h.SessionsRepository.Get(account)
        if session.Status == 1 {
          break
        }
      }
      if session.ID == "" {
        session = h.SessionsRepository.Current()
      }
    } else {
      session = h.SessionsRepository.Current()
    }
    if session.ID == "" {
      mutex.Unlock()
      return errors.New("current session is empty")
    }
    h.SessionsRepository.Update(session, "timestamp", timestamp)
    if cursor, count, err := h.ScrapersRepository.Process(session, post, task.Params); err == nil {
      if cursor == "" {
        delete(task.Params, "cursors")
        h.Rdb.ZRem(h.Ctx, config.REDIS_KEY_TASKS_REPLIES_TARGET, task.ID)
        h.Repository.Updates(task, map[string]interface{}{
          "params": task.Params,
          "status": 2,
        })
        mutex.Unlock()
        continue
      }

      if count < 10 {
        if score == 0 || timestamp-int64(score) < config.SCRAPERS_CURSOR_WAITING_TIMEOUT {
          log.Println("waiting for cursor change", timestamp-int64(score), config.SCRAPERS_CURSOR_WAITING_TIMEOUT)
          mutex.Unlock()
          continue
        }
      }

      if score > 0 {
        h.Rdb.ZAdd(
          h.Ctx,
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
      h.Repository.Update(task, "params", task.Params)
    } else {
      log.Println("error", err)
    }

    mutex.Unlock()
  }
  return nil
}
