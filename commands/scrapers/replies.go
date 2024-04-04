package scrapers

import (
  "context"
  "errors"
  "fmt"
  "log"

  "github.com/go-redis/redis/v8"
  "github.com/nats-io/nats.go"
  "github.com/urfave/cli/v2"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/repositories"
  scrapersRepositories "scraper.local/twitter-scraper/repositories/scrapers"
)

type RepliesHandler struct {
  Db                 *gorm.DB
  Rdb                *redis.Client
  Ctx                context.Context
  Nats               *nats.Conn
  Repository         *scrapersRepositories.RepliesRepository
  PostsRepository    *repositories.PostsRepository
  SessionsRepository *repositories.SessionsRepository
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
      h.Repository = &scrapersRepositories.RepliesRepository{
        Db: h.Db,
      }
      h.PostsRepository = &repositories.PostsRepository{
        Db: h.Db,
      }
      h.SessionsRepository = &repositories.SessionsRepository{
        Db: h.Db,
      }
      h.Repository.UsersRepository = &repositories.UsersRepository{
        Db:   h.Db,
        Nats: h.Nats,
      }
      h.Repository.RepliesRepository = &repositories.RepliesRepository{
        Db:   h.Db,
        Nats: h.Nats,
      }
      return nil
    },
    Action: func(c *cli.Context) error {
      postID := c.Args().Get(0)
      if postID == "" {
        log.Fatal("post id can not be empty")
        return nil
      }
      if err := h.Process(postID); err != nil {
        return cli.Exit(err.Error(), 1)
      }
      return nil
    },
  }
}

func (h *RepliesHandler) Process(postID string) (err error) {
  log.Println(fmt.Sprintf("post[%v] replies scraper processing...", postID))
  post, err := h.PostsRepository.Find(postID)
  if err != nil {
    return err
  }
  params := map[string]interface{}{}
  session := h.SessionsRepository.Current()
  if session == nil {
    return errors.New("current session is empty")
  }
  cursor, count, err := h.Repository.Process(session, post, params)
  log.Println("replies scraper cursor", cursor, count)
  return
}
