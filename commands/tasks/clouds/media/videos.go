package media

import (
  "context"
  "errors"
  "fmt"
  "hash/crc32"
  "log"
  "os"
  "strconv"
  "time"

  "github.com/go-redis/redis/v8"
  "github.com/nats-io/nats.go"
  "github.com/urfave/cli/v2"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/config"
  models "scraper.local/twitter-scraper/models/media"
  cloudsRepositories "scraper.local/twitter-scraper/repositories/clouds/media"
  repositories "scraper.local/twitter-scraper/repositories/media"
)

type VideosHandler struct {
  Db               *gorm.DB
  Rdb              *redis.Client
  Ctx              context.Context
  Nats             *nats.Conn
  VideosRepository *repositories.VideosRepository
  CloudsRepository *cloudsRepositories.VideosRepository
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
      h.CloudsRepository = &cloudsRepositories.VideosRepository{
        Db: h.Db,
      }
      h.CloudsRepository.VideosRepository = h.VideosRepository
      return nil
    },
    Subcommands: []*cli.Command{
      {
        Name:  "sync",
        Usage: "",
        Action: func(c *cli.Context) error {
          limit, _ := strconv.Atoi(c.Args().Get(0))
          if limit < 20 {
            limit = 20
          }
          if err := h.Sync(limit); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return nil
        },
      },
      {
        Name:  "clean",
        Usage: "",
        Action: func(c *cli.Context) error {
          if err := h.Clean(); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return nil
        },
      },
    },
  }
}

func (h *VideosHandler) Sync(limit int) error {
  log.Println(fmt.Sprintf("tasks media videos syncing..."))
  node := common.GetEnvInt("SCRAPER_STORAGE_NODE")
  count, _ := h.Rdb.ZCard(h.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_VIDEOS).Result()
  conditions := make(map[string]interface{})
  if count < config.CLOUDS_SYNCING_MEDIA_VIDEOS_LIMIT {
    conditions["node"] = node
    conditions["is_synced"] = false
  } else {
    conditions["ids"], _ = h.Rdb.ZRange(h.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_VIDEOS, 0, -1).Result()
  }

  videos := h.VideosRepository.Ranking(
    []string{"id", "node", "mime", "filehash", "extension"},
    conditions,
    "size",
    1,
    limit,
  )
  for _, video := range videos {
    timestamp := time.Now().UnixMicro()

    if video.Mime != "video/mp4" {
      h.VideosRepository.Update(video, "status", 4)
      continue
    }

    if video.Node != node {
      continue
    }

    mutex := common.NewMutex(
      h.Rdb,
      h.Ctx,
      fmt.Sprintf(config.LOCKS_TASKS_CLOUDS_MEDIA_VIDEOS_SYNC, video.Filehash),
    )
    if !mutex.Lock(30 * time.Second) {
      continue
    }

    score, _ := h.Rdb.ZScore(h.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_VIDEOS, video.ID).Result()
    if score > 0 {
      if timestamp-int64(score) > 86400*1e6 {
        log.Println("waiting over 24hr", video.ID)
        crc32q := crc32.MakeTable(0xD5828281)
        i := crc32.Checksum([]byte(video.Filehash), crc32q)
        localpath := fmt.Sprintf(
          "%s/videos/%d/%d",
          common.GetEnvString("SCRAPER_STORAGE_PATH"),
          i/233%50,
          i/89%50,
        )
        localfile := fmt.Sprintf(
          "%s/%s.%s",
          localpath,
          video.Filehash,
          video.Extension,
        )
        os.Remove(localfile)
        h.VideosRepository.Update(video, "status", 4)
        h.Rdb.ZRem(h.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_VIDEOS, video.ID)
      }
      log.Println("clouds media videos syncing now", video.ID)
      mutex.Unlock()
      continue
    }

    err := h.CloudsRepository.Sync(video)
    if errors.Is(err, os.ErrNotExist) {
      log.Println("local file not exists", video.ID)
      h.VideosRepository.Update(video, "status", 4)
      h.Rdb.ZRem(h.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_VIDEOS, video.ID)
    }
    if err != nil {
      log.Println("clouds media videos syncing failed", err, video.ID)
      mutex.Unlock()
      continue
    }
    h.Rdb.ZAdd(
      h.Ctx,
      config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_VIDEOS,
      &redis.Z{
        float64(timestamp),
        video.ID,
      },
    )
    count++
    mutex.Unlock()
  }
  return nil
}

func (h *VideosHandler) Clean() error {
  log.Println(fmt.Sprintf("tasks media videos clean..."))
  day := time.Now().UTC().Format("0102")

  node := common.GetEnvInt("SCRAPER_STORAGE_NODE")
  timestamp := time.Now().UnixMicro()
  ids, _ := h.Rdb.ZRangeByScore(
    h.Ctx,
    config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_VIDEOS,
    &redis.ZRangeBy{
      Min: "-inf",
      Max: strconv.FormatInt(timestamp, 10),
    },
  ).Result()
  for _, id := range ids {
    log.Println("clean overtime tasks", id, timestamp)
    video, err := h.VideosRepository.Find(id)
    if err != nil {
      if errors.Is(err, gorm.ErrRecordNotFound) {
        h.Rdb.ZRem(h.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_VIDEOS, video.ID)
      }
      continue
    }
    if video.IsSynced {
      h.Rdb.ZRem(h.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_VIDEOS, video.ID)
      continue
    }
    if video.Node != node {
      continue
    }
    crc32q := crc32.MakeTable(0xD5828281)
    i := crc32.Checksum([]byte(video.Filehash), crc32q)
    localpath := fmt.Sprintf(
      "%s/videos/%d/%d",
      common.GetEnvString("SCRAPER_STORAGE_PATH"),
      i/233%50,
      i/89%50,
    )
    localfile := fmt.Sprintf(
      "%s/%s.%s",
      localpath,
      video.Filehash,
      video.Extension,
    )
    var syncedVideo *models.Video
    if err := h.Db.Where("filehash=? AND is_synced=?", video.Filehash, true).Take(&syncedVideo).Error; err == nil {
      h.VideosRepository.Updates(video, map[string]interface{}{
        "cloud_url": video.CloudUrl,
        "is_synced": true,
      })
      os.Remove(localfile)
      h.Rdb.ZRem(h.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_VIDEOS, video.ID)
      h.Rdb.Del(h.Ctx, fmt.Sprintf(config.REDIS_KEY_MEDIA_VIDEOS, video.UrlSha1, day))
      continue
    }
    if _, err = os.Stat(localfile); err != nil {
      if errors.Is(err, os.ErrNotExist) {
        h.VideosRepository.Update(video, "status", 4)
        h.Rdb.ZRem(h.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_VIDEOS, video.ID)
      }
    }
  }
  return nil
}
