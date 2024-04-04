package commands

import (
  "log"

  "github.com/urfave/cli/v2"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/models"
)

type DbHandler struct {
  Db    *gorm.DB
  TorDb *gorm.DB
}

func NewDbCommand() *cli.Command {
  var h DbHandler
  return &cli.Command{
    Name:  "db",
    Usage: "",
    Before: func(c *cli.Context) error {
      h = DbHandler{
        Db:    common.NewDB(),
        TorDb: common.NewTorDB(),
      }
      return nil
    },
    Subcommands: []*cli.Command{
      {
        Name:  "migrate",
        Usage: "",
        Action: func(c *cli.Context) error {
          if err := h.migrate(); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return nil
        },
      },
    },
  }
}

func (h *DbHandler) migrate() error {
  log.Println("process migrator")
  h.Db.AutoMigrate(
    &models.User{},
    &models.Post{},
    &models.Reply{},
    &models.Task{},
    &models.Session{},
    &models.Admin{},
  )
  models.NewMedia().AutoMigrate(h.Db)
  models.NewPlatform().AutoMigrate(h.Db)
  models.NewTor().AutoMigrate(h.TorDb)
  return nil
}
