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

type PhotosHandler struct {
  Db               *gorm.DB
  Rdb              *redis.Client
  Ctx              context.Context
  Nats             *nats.Conn
  PhotosRepository *repositories.PhotosRepository
  CloudsRepository *cloudsRepositories.PhotosRepository
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
      h.PhotosRepository = &repositories.PhotosRepository{
        Db: h.Db,
      }
      h.CloudsRepository = &cloudsRepositories.PhotosRepository{
        Db: h.Db,
      }
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
          mode, _ := strconv.Atoi(c.Args().Get(1))
          if err := h.Sync(limit, mode); err != nil {
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

func (h *PhotosHandler) Sync(limit int, mode int) error {
  log.Println(fmt.Sprintf("tasks media photos syncing..."))
  node := common.GetEnvInt("SCRAPER_STORAGE_NODE")
  count, _ := h.Rdb.ZCard(h.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_PHOTOS).Result()
  conditions := make(map[string]interface{})
  if count < config.CLOUDS_SYNCING_MEDIA_PHOTOS_LIMIT {
    conditions["node"] = node
    conditions["is_synced"] = false
  } else {
    conditions["ids"], _ = h.Rdb.ZRange(h.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_PHOTOS, 0, -1).Result()
  }

  photos := h.PhotosRepository.Ranking(
    []string{"id", "node", "filehash", "extension"},
    conditions,
    "size",
    1,
    limit,
  )
  for _, photo := range photos {
    timestamp := time.Now().UnixMicro()

    if photo.Node != node {
      continue
    }

    mutex := common.NewMutex(
      h.Rdb,
      h.Ctx,
      fmt.Sprintf(config.LOCKS_TASKS_CLOUDS_MEDIA_PHOTOS_SYNC, photo.Filehash),
    )
    if !mutex.Lock(30 * time.Second) {
      continue
    }

    score, _ := h.Rdb.ZScore(h.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_PHOTOS, photo.ID).Result()
    if score > 0 {
      log.Println("clouds media photos syncing now", photo.ID)
      mutex.Unlock()
      continue
    }

    cloudUrl, err := h.CloudsRepository.Sync(photo, mode)
    if errors.Is(err, os.ErrNotExist) {
      log.Println("local file not exists", photo.ID)
      h.PhotosRepository.Update(photo, "status", 4)
      h.Rdb.ZRem(h.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_PHOTOS, photo.ID)
    }
    if err != nil {
      log.Println("clouds media photos syncing failed", err, photo.ID)
      mutex.Unlock()
      continue
    }

    if mode == 1 {
      h.PhotosRepository.Updates(photo, map[string]interface{}{
        "cloud_url": cloudUrl,
        "is_synced": true,
      })
      h.Rdb.ZRem(h.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_PHOTOS, photo.ID)
    } else {
      h.Rdb.ZAdd(
        h.Ctx,
        config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_PHOTOS,
        &redis.Z{
          float64(timestamp),
          photo.ID,
        },
      )
    }
    count++
    mutex.Unlock()
  }
  return nil
}

func (h *PhotosHandler) Clean() error {
  log.Println(fmt.Sprintf("tasks media photos clean..."))
  day := time.Now().UTC().Format("0102")

  node := common.GetEnvInt("SCRAPER_STORAGE_NODE")
  timestamp := time.Now().UnixMicro()
  ids, _ := h.Rdb.ZRangeByScore(
    h.Ctx,
    config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_PHOTOS,
    &redis.ZRangeBy{
      Min: "-inf",
      Max: strconv.FormatInt(timestamp, 10),
    },
  ).Result()
  for _, id := range ids {
    log.Println("clean overtime tasks", id, timestamp)
    photo, err := h.PhotosRepository.Find(id)
    if err != nil {
      if errors.Is(err, gorm.ErrRecordNotFound) {
        h.Rdb.ZRem(h.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_PHOTOS, photo.ID)
      }
      continue
    }
    if photo.IsSynced {
      h.Rdb.ZRem(h.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_PHOTOS, photo.ID)
      continue
    }
    if photo.Node != node {
      continue
    }
    crc32q := crc32.MakeTable(0xD5828281)
    i := crc32.Checksum([]byte(photo.Filehash), crc32q)
    localpath := fmt.Sprintf(
      "%s/photos/%d/%d",
      common.GetEnvString("SCRAPER_STORAGE_PATH"),
      i/233%50,
      i/89%50,
    )
    localfile := fmt.Sprintf(
      "%s/%s.%s",
      localpath,
      photo.Filehash,
      photo.Extension,
    )
    var syncedPhoto *models.Photo
    if err := h.Db.Where("filehash=? AND is_synced=?", photo.Filehash, true).Take(&syncedPhoto).Error; err == nil {
      h.PhotosRepository.Updates(photo, map[string]interface{}{
        "cloud_url": photo.CloudUrl,
        "is_synced": true,
      })
      os.Remove(localfile)
      h.Rdb.ZRem(h.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_VIDEOS, photo.ID)
      h.Rdb.Del(h.Ctx, fmt.Sprintf(config.REDIS_KEY_MEDIA_PHOTOS, photo.UrlSha1, day))
      continue
    }
    if _, err = os.Stat(localfile); err != nil {
      if errors.Is(err, os.ErrNotExist) {
        h.PhotosRepository.Update(photo, "status", 4)
        h.Rdb.ZRem(h.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_PHOTOS, photo.ID)
      }
    }
  }
  return nil
}
