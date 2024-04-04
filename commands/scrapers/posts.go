package scrapers

import (
  "context"
  "errors"
  "fmt"
  "github.com/go-redis/redis/v8"
  "github.com/nats-io/nats.go"
  "github.com/urfave/cli/v2"
  "gorm.io/gorm"
  "log"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/repositories"
  scrapersRepositories "scraper.local/twitter-scraper/repositories/scrapers"
)

type PostsHandler struct {
  Db                 *gorm.DB
  Rdb                *redis.Client
  Ctx                context.Context
  Nats               *nats.Conn
  Repository         *scrapersRepositories.PostsRepository
  SessionsRepository *repositories.SessionsRepository
  UsersRepository    *repositories.UsersRepository
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
      h.Repository = &scrapersRepositories.PostsRepository{
        Db: h.Db,
      }
      h.SessionsRepository = &repositories.SessionsRepository{
        Db: h.Db,
      }
      h.UsersRepository = &repositories.UsersRepository{
        Db:   h.Db,
        Nats: h.Nats,
      }
      h.Repository.UsersRepository = h.UsersRepository
      h.Repository.PostsRepository = &repositories.PostsRepository{
        Db:   h.Db,
        Nats: h.Nats,
      }
      return nil
    },
    Action: func(c *cli.Context) error {
      account := c.Args().Get(0)
      if account == "" {
        log.Fatal("account is empty")
        return nil
      }
      cursor := c.Args().Get(1)
      if err := h.Process(account, cursor); err != nil {
        return cli.Exit(err.Error(), 1)
      }
      return nil
    },
  }
}

func (h *PostsHandler) Process(account string, cursor string) (err error) {
  log.Println(fmt.Sprintf("user %v posts scrapping...", account))
  user, err := h.UsersRepository.Get(account)
  if err != nil {
    return err
  }
  params := map[string]interface{}{
    "user_id": user.ID,
  }
  session := h.SessionsRepository.Special()
  if session == nil {
    return errors.New("special session is empty")
  }
  if cursor != "" {
    params["cursors"] = map[string]interface{}{
      session.Account: cursor,
    }
  }
  cursor, count, err := h.Repository.Process(session, user, params)
  log.Println("posts scraper cursor", cursor, count)
  return
}
