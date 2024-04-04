package media

import (
  "context"
  "errors"
  "fmt"
  "log"
  "time"

  "github.com/go-redis/redis/v8"
  "github.com/urfave/cli/v2"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/config"
  cloudsRepositories "scraper.local/twitter-scraper/repositories/clouds/media"
  repositories "scraper.local/twitter-scraper/repositories/media"
)

type VideosHandler struct {
  Db               *gorm.DB
  Rdb              *redis.Client
  Ctx              context.Context
  VideosRepository *repositories.VideosRepository
  Repository       *cloudsRepositories.VideosRepository
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
      h.VideosRepository = &repositories.VideosRepository{
        Db: h.Db,
      }
      h.Repository = &cloudsRepositories.VideosRepository{
        Db: h.Db,
      }
      h.Repository.VideosRepository = h.VideosRepository
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
          if err = h.Sync(id); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return
        },
      },
    },
  }
}

func (h *VideosHandler) Sync(id string) (err error) {
  log.Println(fmt.Sprintf("video[%v] start syncing...", id))
  video, err := h.VideosRepository.Find(id)
  if errors.Is(err, gorm.ErrRecordNotFound) {
    return
  }
  if video.Node != common.GetEnvInt("SCRAPER_STORAGE_NODE") {
    err = errors.New("scraper storage node mot match")
    return
  }
  err = h.Repository.Sync(video)
  if err == nil {
    timestamp := time.Now().UnixMicro()
    h.Rdb.ZAdd(
      h.Ctx,
      config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_VIDEOS,
      &redis.Z{
        float64(timestamp),
        video.ID,
      },
    )
    log.Println(fmt.Sprintf("video[%v] sync success", video.ID))
  }
  return
}
