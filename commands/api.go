package commands

import (
  "context"
  "fmt"
  "log"
  "net/http"
  "os"

  "github.com/go-chi/chi/v5"
  "github.com/go-redis/redis/v8"
  "github.com/urfave/cli/v2"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/api/v1"
  "scraper.local/twitter-scraper/common"
)

type ApiHandler struct {
  Db  *gorm.DB
  Rdb *redis.Client
  Ctx context.Context
}

func NewApiCommand() *cli.Command {
  var h ApiHandler
  return &cli.Command{
    Name:  "api",
    Usage: "",
    Before: func(c *cli.Context) error {
      h = ApiHandler{
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

func (h *ApiHandler) Run() error {
  log.Println("api running...")

  apiContext := &common.ApiContext{
    Db:  h.Db,
    Rdb: h.Rdb,
    Ctx: h.Ctx,
  }

  r := chi.NewRouter()
  r.Route("/v1", func(r chi.Router) {
    r.Mount("/clouds", v1.NewCloudsRouter(apiContext))
    r.Mount("/scrapers", v1.NewScrapersRouter(apiContext))
    r.Mount("/login", v1.NewLoginRouter(apiContext))
    r.Mount("/tasks", v1.NewTasksRouter(apiContext))
  })

  err := http.ListenAndServe(
    fmt.Sprintf("127.0.0.1:%v", os.Getenv("SCRAPER_API_PORT")),
    r,
  )
  if err != nil {
    return err
  }

  return nil
}
