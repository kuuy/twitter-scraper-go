package queue

import (
  "context"
  "log"

  "github.com/go-redis/redis/v8"
  "github.com/hibiken/asynq"
  "github.com/urfave/cli/v2"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/queue/asynq/workers"
)

type AsynqHandler struct {
  Db  *gorm.DB
  Rdb *redis.Client
  Ctx context.Context
}

func NewAsynqCommand() *cli.Command {
  var h AsynqHandler
  return &cli.Command{
    Name:  "asynq",
    Usage: "",
    Before: func(c *cli.Context) error {
      h = AsynqHandler{
        Db:  common.NewDB(),
        Rdb: common.NewRedis(),
        Ctx: context.Background(),
      }
      return nil
    },
    Action: func(c *cli.Context) error {
      if err := h.run(); err != nil {
        return cli.Exit(err.Error(), 1)
      }
      return nil
    },
  }
}

func (h *AsynqHandler) run() error {
  log.Println("asynq queue running...")

  mux := asynq.NewServeMux()
  worker := common.NewAsynqServer()

  ansqContext := &common.AnsqServerContext{
    Db:   h.Db,
    Rdb:  h.Rdb,
    Ctx:  h.Ctx,
    Mux:  mux,
    Nats: common.NewNats(),
  }

  workers.NewScrapers(ansqContext).Register()
  workers.NewSessions(ansqContext).Register()

  if err := worker.Run(mux); err != nil {
    return err
  }

  return nil
}
