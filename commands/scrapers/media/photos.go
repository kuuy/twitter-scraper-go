package media

import (
  "context"
  "crypto/sha1"
  "encoding/hex"
  "fmt"
  "github.com/go-redis/redis/v8"
  "github.com/urfave/cli/v2"
  "gorm.io/gorm"
  "log"
  "strconv"

  "scraper.local/twitter-scraper/common"
  repositories "scraper.local/twitter-scraper/repositories/media"
  scrapersRepositories "scraper.local/twitter-scraper/repositories/scrapers/media"
)

type PhotosHandler struct {
  Db               *gorm.DB
  Rdb              *redis.Client
  Ctx              context.Context
  Repository       *scrapersRepositories.PhotosRepository
  PhotosRepository *repositories.PhotosRepository
}

func NewPhotosCommand() *cli.Command {
  var h PhotosHandler
  return &cli.Command{
    Name:  "photos",
    Usage: "",
    Before: func(c *cli.Context) error {
      h = PhotosHandler{
        Db:  common.NewDB(),
        Rdb: common.NewRedis(),
        Ctx: context.Background(),
      }
      h.Repository = &scrapersRepositories.PhotosRepository{
        Db: h.Db,
      }
      h.PhotosRepository = &repositories.PhotosRepository{
        Db: h.Db,
      }
      return nil
    },
    Action: func(c *cli.Context) error {
      url := c.Args().Get(0)
      if url == "" {
        log.Fatal("photo url can not be empty")
        return nil
      }
      if err := h.Download(url); err != nil {
        return cli.Exit(err.Error(), 1)
      }
      return nil
    },
    Subcommands: []*cli.Command{
      {
        Name:  "fix",
        Usage: "",
        Action: func(c *cli.Context) error {
          limit, _ := strconv.Atoi(c.Args().Get(0))
          if limit < 20 {
            limit = 20
          }
          if err := h.Fix(limit); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return nil
        },
      },
    },
  }
}

func (h *PhotosHandler) Download(url string) (err error) {
  log.Println(fmt.Sprintf("photo %v scrapping...", url))
  hash := sha1.Sum([]byte(url))
  urlSha1 := hex.EncodeToString(hash[:])
  if h.PhotosRepository.IsExists(url, urlSha1) {
    log.Println(fmt.Sprintf("photo %v exists", url))
    return
  }
  return h.Repository.Download(url, urlSha1)
}

func (h *PhotosHandler) Fix(limit int) (err error) {
  log.Println(fmt.Sprintf("photos fix..."))
  conditions := map[string]interface{}{
    "width": 0,
  }
  photos := h.PhotosRepository.Listings(conditions, 1, limit)
  log.Println("photos", len(photos))
  for _, photo := range photos {
    config, err := h.Repository.Config(photo.Url)
    if err == nil {
      h.PhotosRepository.Updates(photo, map[string]interface{}{
        "width":  config.Width,
        "height": config.Height,
      })
    }
  }
  return
}
