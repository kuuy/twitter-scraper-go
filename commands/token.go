package commands

import (
  "context"
  "scraper.local/twitter-scraper/common"
  "fmt"
  "log"

  "github.com/go-redis/redis/v8"
  "github.com/urfave/cli/v2"

  "scraper.local/twitter-scraper/repositories"
)

type TokenHandler struct {
  Rdb        *redis.Client
  Ctx        context.Context
  Repository *repositories.TokenRepository
}

func NewTokenCommand() *cli.Command {
  var h TokenHandler
  return &cli.Command{
    Name:  "token",
    Usage: "",
    Before: func(c *cli.Context) error {
      h = TokenHandler{
        Rdb: common.NewRedis(),
        Ctx: context.Background(),
      }
      h.Repository = &repositories.TokenRepository{
        Rdb: h.Rdb,
        Ctx: h.Ctx,
      }
      return nil
    },
    Subcommands: []*cli.Command{
      {
        Name:  "refresh",
        Usage: "",
        Action: func(c *cli.Context) error {
          if err := h.Refresh(); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return nil
        },
      },
    },
  }
}

func (h *TokenHandler) Refresh() error {
  log.Println(fmt.Sprintf("token refreshing..."))
  return h.Repository.Refresh()
}
