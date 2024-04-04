package media

import (
  "context"
  "crypto/sha1"
  "encoding/hex"
  "errors"
  "fmt"
  "log"
  "strconv"
  "strings"
  "time"

  "github.com/go-redis/redis/v8"
  "github.com/nats-io/nats.go"
  "github.com/urfave/cli/v2"
  "golang.org/x/sys/unix"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/config"
  "scraper.local/twitter-scraper/repositories"
  mediaRepositories "scraper.local/twitter-scraper/repositories/media"
  scrapersRepositories "scraper.local/twitter-scraper/repositories/scrapers/media"
)

type UsersHandler struct {
  Db                       *gorm.DB
  Rdb                      *redis.Client
  Ctx                      context.Context
  Nats                     *nats.Conn
  Repository               *repositories.TasksRepository
  UsersRepository          *repositories.UsersRepository
  PhotosRepository         *mediaRepositories.PhotosRepository
  ScrapersPhotosRepository *scrapersRepositories.PhotosRepository
}

func NewUsersCommand() *cli.Command {
  var h UsersHandler
  return &cli.Command{
    Name:  "users",
    Usage: "",
    Before: func(c *cli.Context) error {
      h = UsersHandler{
        Db:  common.NewDB(),
        Rdb: common.NewRedis(),
        Ctx: context.Background(),
      }
      h.Repository = &repositories.TasksRepository{
        Db: h.Db,
      }
      h.UsersRepository = &repositories.UsersRepository{
        Db: h.Db,
      }
      h.PhotosRepository = &mediaRepositories.PhotosRepository{
        Db: h.Db,
      }
      h.ScrapersPhotosRepository = &scrapersRepositories.PhotosRepository{
        Db: h.Db,
      }
      return nil
    },
    Subcommands: []*cli.Command{
      {
        Name:  "init",
        Usage: "",
        Action: func(c *cli.Context) error {
          limit, _ := strconv.Atoi(c.Args().Get(0))
          if limit < 20 {
            limit = 20
          }
          if err := h.Init(limit); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return nil
        },
      },
      {
        Name:  "apply",
        Usage: "",
        Action: func(c *cli.Context) (err error) {
          id := c.Args().Get(0)
          if id == "" {
            log.Fatal("twitter users id can not be empty")
            return nil
          }
          if err = h.Apply(id); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return
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

func (h *UsersHandler) Init(limit int) error {
  log.Println(fmt.Sprintf("twitter media users tasks init..."))
  conditions := map[string]interface{}{}
  users := h.UsersRepository.Ranking(
    []string{"id", "timestamp"},
    conditions,
    "timestamp",
    -1,
    limit,
  )
  for _, user := range users {
    name := fmt.Sprintf("%v@media.users", user.ID)
    action := config.TASK_ACTION_SCRAPERS_MEDIA_USERS
    params := map[string]interface{}{
      "id": user.ID,
    }
    h.Repository.Apply(name, action, params)
  }
  return nil
}

func (h *UsersHandler) Apply(id string) (err error) {
  log.Println(fmt.Sprintf("tasks media users apply..."))
  user, err := h.UsersRepository.Find(id)
  if errors.Is(err, gorm.ErrRecordNotFound) {
    return
  }
  name := fmt.Sprintf("%v@media", user.ID)
  action := config.TASK_ACTION_SCRAPERS_MEDIA_USERS
  params := map[string]interface{}{
    "id": user.ID,
  }
  return h.Repository.Apply(name, action, params)
}

func (h *UsersHandler) Process(limit int) error {
  log.Println(fmt.Sprintf("tasks media users processing..."))

  var stat unix.Statfs_t
  unix.Statfs(common.GetEnvString("SCRAPER_STORAGE_PATH"), &stat)
  freeGB := int(stat.Bavail * uint64(stat.Bsize) / 1073741824)

  tasks := h.Repository.Ranking(
    []string{"id", "params", "timestamp"},
    map[string]interface{}{
      "action": config.TASK_ACTION_SCRAPERS_MEDIA_USERS,
    },
    "timestamp",
    -1,
    limit,
  )
  for _, task := range tasks {
    mutex := common.NewMutex(
      h.Rdb,
      h.Ctx,
      fmt.Sprintf(config.LOCKS_TASKS_SCRAPERS_MEDIA_USERS_PROCESS, task.ID),
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
    if _, ok := task.Params["id"]; !ok {
      mutex.Unlock()
      continue
    }
    id := task.Params["id"].(string)
    user, err := h.UsersRepository.Find(id)
    if errors.Is(err, gorm.ErrRecordNotFound) {
      h.Repository.Delete(task.ID)
    }
    if err != nil {
      mutex.Unlock()
      continue
    }
    if !strings.HasPrefix(user.Avatar, "https://") {
      h.Repository.Update(task, "status", 4)
      mutex.Unlock()
      continue
    }
    url := user.Avatar
    hash := sha1.Sum([]byte(url))
    urlSha1 := hex.EncodeToString(hash[:])
    if !h.PhotosRepository.IsExists(url, urlSha1) && freeGB > common.GetEnvInt("SCRAPER_DISK_MIN_PHOTOS_GB") {
      if err := h.ScrapersPhotosRepository.Download(url, urlSha1); err != nil {
        log.Println("error", err)
        h.Repository.Update(task, "status", 3)
        continue
      }
    }
    h.Repository.Update(task, "status", 2)

    mutex.Unlock()
  }
  return nil
}
