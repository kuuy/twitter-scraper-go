package commands

import (
  "context"
  "log"
  "sync"

  "github.com/go-redis/redis/v8"
  "github.com/hibiken/asynq"
  "github.com/robfig/cron/v3"
  "github.com/urfave/cli/v2"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/tasks"
)

type CronHandler struct {
  Db    *gorm.DB
  Rdb   *redis.Client
  Asynq *asynq.Client
  Ctx   context.Context
}

func NewCronCommand() *cli.Command {
  var h CronHandler
  return &cli.Command{
    Name:  "cron",
    Usage: "",
    Before: func(c *cli.Context) error {
      h = CronHandler{
        Db:    common.NewDB(),
        Rdb:   common.NewRedis(),
        Asynq: common.NewAsynqClient(),
        Ctx:   context.Background(),
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

func (h *CronHandler) run() error {
  log.Println("cron running...")

  wg := &sync.WaitGroup{}
  wg.Add(1)

  ansqContext := &common.AnsqClientContext{
    Db:   h.Db,
    Rdb:  h.Rdb,
    Ctx:  h.Ctx,
    Conn: h.Asynq,
  }

  sessions := tasks.NewSessionsTask(ansqContext)
  scrapers := tasks.NewScrapersTask(ansqContext)

  c := cron.New()
  c.AddFunc("@every 30s", func() {
    scrapers.Posts().Flush(5)
    scrapers.Posts().Process(5)
    scrapers.Replies().Flush(30)
    scrapers.Replies().Process(30)
  })
  c.AddFunc("@every 15m", func() {
    sessions.Flush()
  })
  c.AddFunc("30 23 * * * *", func() {
    scrapers.Replies().Init(1000)
  })
  c.Start()

  <-h.wait(wg)

  return nil
}

func (h *CronHandler) wait(wg *sync.WaitGroup) chan bool {
  ch := make(chan bool)
  go func() {
    wg.Wait()
    ch <- true
  }()
  return ch
}
