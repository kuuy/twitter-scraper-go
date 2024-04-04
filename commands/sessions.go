package commands

import (
  "context"
  "errors"
  "fmt"
  "github.com/go-redis/redis/v8"
  "github.com/nats-io/nats.go"
  "github.com/urfave/cli/v2"
  "gorm.io/gorm"
  "log"
  "strconv"
  "time"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/repositories"
)

type SessionsHandler struct {
  Db         *gorm.DB
  Rdb        *redis.Client
  Ctx        context.Context
  Nats       *nats.Conn
  Repository *repositories.SessionsRepository
}

func NewSessionsCommand() *cli.Command {
  var h SessionsHandler
  return &cli.Command{
    Name:  "sessions",
    Usage: "",
    Before: func(c *cli.Context) error {
      h = SessionsHandler{
        Db:  common.NewDB(),
        Rdb: common.NewRedis(),
        Ctx: context.Background(),
      }
      h.Repository = &repositories.SessionsRepository{
        Db:  h.Db,
        Ctx: h.Ctx,
      }
      return nil
    },
    Subcommands: []*cli.Command{
      {
        Name:  "apply",
        Usage: "",
        Action: func(c *cli.Context) error {
          account := c.Args().Get(0)
          if account == "" {
            log.Fatal("twitter account can not be empty")
            return nil
          }
          cookie := c.Args().Get(1)
          if cookie == "" {
            log.Fatal("twitter cookie can not be empty")
            return nil
          }
          slot, _ := strconv.Atoi(c.Args().Get(2))
          if err := h.Apply(account, cookie, slot); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return nil
        },
      },
      {
        Name:  "current",
        Usage: "",
        Action: func(c *cli.Context) error {
          if err := h.Current(); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return nil
        },
      },
      {
        Name:  "flush",
        Usage: "",
        Action: func(c *cli.Context) error {
          id := c.Args().Get(0)
          if id == "" {
            log.Fatal("twitter sessions id can not be empty")
            return nil
          }
          if err := h.Flush(id); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return nil
        },
      },
    },
  }
}

func (h *SessionsHandler) Apply(account string, cookie string, slot int) (err error) {
  log.Println(fmt.Sprintf("twitters sessions apply..."))
  session, err := h.Repository.Apply(account, cookie, slot)
  if err == nil {
    h.Repository.Flush(session)
  }
  return
}

func (h *SessionsHandler) Current() error {
  log.Println(fmt.Sprintf("twitters sessions current..."))
  timestamp := time.Now().UnixMicro()
  session := h.Repository.Current()
  if session == nil {
    return errors.New("current session is empty")
  }
  if session.UnblockedAt > timestamp {
    return errors.New("current session has been blocked")
  }
  log.Println("current session", session)
  return nil
}

func (h *SessionsHandler) Flush(id string) error {
  log.Println(fmt.Sprintf("twitters sessions flushing..."))
  session, err := h.Repository.Find(id)
  if err != nil {
    return err
  }
  return h.Repository.Flush(session)
}
