package media

import (
  "context"
  "crypto/sha1"
  "encoding/hex"
  "scraper.local/twitter-scraper/config"
  models "scraper.local/twitter-scraper/models/media"
  "fmt"
  "hash/crc32"
  "log"
  "os"
  "strconv"
  "time"

  "github.com/go-redis/redis/v8"
  "github.com/urfave/cli/v2"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/common"
  repositories "scraper.local/twitter-scraper/repositories/media"
)

type PhotosHandler struct {
  Db         *gorm.DB
  Rdb        *redis.Client
  Ctx        context.Context
  Repository *repositories.PhotosRepository
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
      h.Repository = &repositories.PhotosRepository{
        Db:  h.Db,
        Rdb: h.Rdb,
        Ctx: h.Ctx,
      }
      return nil
    },
    Subcommands: []*cli.Command{
      {
        Name:  "info",
        Usage: "",
        Action: func(c *cli.Context) error {
          url := c.Args().Get(0)
          if url == "" {
            log.Fatal("photo url can not be empty")
            return nil
          }
          if err := h.Info(url); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return nil
        },
      },
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

func (h *PhotosHandler) Info(url string) (err error) {
  log.Println(fmt.Sprintf("photo %v info...", url))

  hash := sha1.Sum([]byte(url))
  urlSha1 := hex.EncodeToString(hash[:])
  photo, err := h.Repository.Get(url, urlSha1)
  if err != nil {
    return
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
  storageUrl := fmt.Sprintf(
    "%s/photos/%d/%d/%s.%s",
    common.GetEnvString(fmt.Sprintf("SCRAPER_STORAGE_URL_%v", photo.Node)),
    i/233%50,
    i/89%50,
    photo.Filehash,
    photo.Extension,
  )

  log.Println("photo info", localfile, storageUrl)

  return nil
}

func (h *PhotosHandler) Fix(limit int) (err error) {
  log.Println(fmt.Sprintf("media photos fix"))

  day := time.Now().UTC().Format("0102")

  conditions := map[string]interface{}{
    "node":      common.GetEnvInt("SCRAPER_STORAGE_NODE"),
    "is_synced": false,
  }
  photos := h.Repository.Ranking(
    []string{"id", "filehash"},
    conditions,
    "size",
    1,
    limit,
  )
  for _, photo := range photos {
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
      h.Repository.Updates(photo, map[string]interface{}{
        "cloud_url": photo.CloudUrl,
        "is_synced": true,
      })
      os.Remove(localfile)
      h.Rdb.ZRem(h.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_PHOTOS, photo.ID)
      h.Rdb.Del(h.Ctx, fmt.Sprintf(config.REDIS_KEY_MEDIA_PHOTOS, photo.UrlSha1, day))
    }
  }

  return nil
}
