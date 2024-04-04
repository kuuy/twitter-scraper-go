package media

import (
  "context"
  "errors"
  "fmt"
  "log"
  "strconv"
  "time"

  "github.com/go-redis/redis/v8"
  "github.com/urfave/cli/v2"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/config"
  cloudsRepositories "scraper.local/twitter-scraper/repositories/clouds/media"
  repositories "scraper.local/twitter-scraper/repositories/media"
)

type PhotosHandler struct {
  Db               *gorm.DB
  Rdb              *redis.Client
  Ctx              context.Context
  Repository       *cloudsRepositories.PhotosRepository
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
      h.Repository = &cloudsRepositories.PhotosRepository{
        Db: h.Db,
      }
      h.PhotosRepository = &repositories.PhotosRepository{
        Db: h.Db,
      }
      return nil
    },
    Subcommands: []*cli.Command{
      {
        Name:  "sync",
        Usage: "",
        Action: func(c *cli.Context) (err error) {
          id := c.Args().Get(0)
          if id == "" {
            log.Fatal("id can not be empty")
            return nil
          }
          mode, _ := strconv.Atoi(c.Args().Get(1))
          if err = h.Sync(id, mode); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return
        },
      },
    },
  }
}

func (h *PhotosHandler) Sync(id string, mode int) (err error) {
  log.Println(fmt.Sprintf("photo[%v] start syncing...", id))
  photo, err := h.PhotosRepository.Find(id)
  if errors.Is(err, gorm.ErrRecordNotFound) {
    return
  }
  if photo.Node != common.GetEnvInt("SCRAPER_STORAGE_NODE") {
    err = errors.New("scraper storage node mot match")
    return
  }
  cloudUrl, err := h.Repository.Sync(photo, mode)
  if err == nil {
    log.Println(fmt.Sprintf("photo[%v] sync success", id), cloudUrl)
    if mode == 0 {
      timestamp := time.Now().UnixMicro()
      h.Rdb.ZAdd(
        h.Ctx,
        config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_PHOTOS,
        &redis.Z{
          float64(timestamp),
          photo.ID,
        },
      )
    }
  }
  return
}
