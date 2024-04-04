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

type UsersHandler struct {
  Db                 *gorm.DB
  Rdb                *redis.Client
  Ctx                context.Context
  Nats               *nats.Conn
  Repository         *scrapersRepositories.UsersRepository
  UsersRepository    *repositories.UsersRepository
  SessionsRepository *repositories.SessionsRepository
}

func NewUsersCommand() *cli.Command {
  var h UsersHandler
  return &cli.Command{
    Name:  "users",
    Usage: "",
    Before: func(c *cli.Context) error {
      h = UsersHandler{
        Db:   common.NewDB(),
        Rdb:  common.NewRedis(),
        Ctx:  context.Background(),
        Nats: common.NewNats(),
      }
      h.Repository = &scrapersRepositories.UsersRepository{
        Db: h.Db,
      }
      h.UsersRepository = &repositories.UsersRepository{
        Db: h.Db,
      }
      h.SessionsRepository = &repositories.SessionsRepository{
        Db: h.Db,
      }
      h.Repository.UsersRepository = &repositories.UsersRepository{
        Db:   h.Db,
        Nats: h.Nats,
      }
      return nil
    },
    Action: func(c *cli.Context) error {
      account := c.Args().Get(0)
      if account == "" {
        log.Fatal("account can not be empty")
        return nil
      }
      if err := h.Process(account); err != nil {
        return cli.Exit(err.Error(), 1)
      }
      return nil
    },
  }
}

func (h *UsersHandler) Process(account string) (err error) {
  log.Println(fmt.Sprintf("account[%v] users scraper processing...", account))

  session := h.SessionsRepository.Special()
  if session == nil {
    return errors.New("current session is empty")
  }
  user, err := h.Repository.Process(session, account)
  log.Println("user", user)
  return
}
