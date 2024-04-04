package media

import (
  "context"
  "crypto/sha1"
  "encoding/hex"
  "fmt"
  "log"

  "github.com/go-redis/redis/v8"
  "github.com/urfave/cli/v2"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/common"
  repositories "scraper.local/twitter-scraper/repositories/media"
  scrapersRepositories "scraper.local/twitter-scraper/repositories/scrapers/media"
)

type VideosHandler struct {
  Db               *gorm.DB
  Rdb              *redis.Client
  Ctx              context.Context
  Repository       *scrapersRepositories.VideosRepository
  VideosRepository *repositories.VideosRepository
}

func NewVideosCommand() *cli.Command {
  var h VideosHandler
  return &cli.Command{
    Name:  "videos",
    Usage: "",
    Before: func(c *cli.Context) error {
      h = VideosHandler{
        Db:  common.NewDB(),
        Rdb: common.NewRedis(),
        Ctx: context.Background(),
      }
      h.Repository = &scrapersRepositories.VideosRepository{
        Db: h.Db,
      }
      h.VideosRepository = &repositories.VideosRepository{
        Db: h.Db,
      }
      return nil
    },
    Action: func(c *cli.Context) error {
      url := c.Args().Get(0)
      if url == "" {
        log.Fatal("video url can not be empty")
        return nil
      }
      if err := h.Download(url); err != nil {
        return cli.Exit(err.Error(), 1)
      }
      return nil
    },
  }
}

func (h *VideosHandler) Download(url string) (err error) {
  log.Println(fmt.Sprintf("video %v scrapping...", url))
  hash := sha1.Sum([]byte(url))
  urlSha1 := hex.EncodeToString(hash[:])
  if h.VideosRepository.IsExists(url, urlSha1) {
    log.Println(fmt.Sprintf("video %v exists", url))
    return
  }
  return h.Repository.Download(url, urlSha1)
}
