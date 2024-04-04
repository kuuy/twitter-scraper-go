package queue

import (
  "context"
  "gorm.io/gorm"
  "log"
  "sync"

  "github.com/go-redis/redis/v8"
  "github.com/urfave/cli/v2"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/queue/nats"
)

type NatsHandler struct {
  Db  *gorm.DB
  Rdb *redis.Client
  Ctx context.Context
}

func NewNatsCommand() *cli.Command {
  var h NatsHandler
  return &cli.Command{
    Name:  "nats",
    Usage: "",
    Before: func(c *cli.Context) error {
      h = NatsHandler{
        Db:  common.NewDB(),
        Rdb: common.NewRedis(),
        Ctx: context.Background(),
      }
      return nil
    },
    Action: func(c *cli.Context) error {
      if err := h.Run(); err != nil {
        return cli.Exit(err.Error(), 1)
      }
      return nil
    },
  }
}

func (h *NatsHandler) Run() error {
  log.Println("nats running...")

  wg := &sync.WaitGroup{}
  wg.Add(1)

  nc := common.NewNats()
  defer nc.Close()

  natsContext := &common.NatsContext{
    Db:   h.Db,
    Rdb:  h.Rdb,
    Ctx:  h.Ctx,
    Conn: nc,
  }
  nats.NewWorkers(natsContext).Subscribe()

  <-h.wait(wg)

  return nil
}

func (h *NatsHandler) wait(wg *sync.WaitGroup) chan bool {
  ch := make(chan bool)
  go func() {
    wg.Wait()
    ch <- true
  }()
  return ch
}
