package media

import (
  "bufio"
  "context"
  "crypto/sha1"
  "encoding/hex"
  "errors"
  models "scraper.local/twitter-scraper/models/media"
  "fmt"
  "hash/crc32"
  "log"
  "os"
  "os/exec"
  "path/filepath"
  "strconv"
  "strings"
  "syscall"
  "time"

  "github.com/go-redis/redis/v8"
  "github.com/urfave/cli/v2"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/config"
  repositories "scraper.local/twitter-scraper/repositories/media"
)

type VideosHandler struct {
  Db         *gorm.DB
  Rdb        *redis.Client
  Ctx        context.Context
  Repository *repositories.VideosRepository
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
      h.Repository = &repositories.VideosRepository{
        Db:  h.Db,
        Rdb: h.Rdb,
        Ctx: h.Ctx,
      }
      return nil
    },
    Subcommands: []*cli.Command{
      {
        Name:  "find",
        Usage: "",
        Action: func(c *cli.Context) error {
          id := c.Args().Get(0)
          if id == "" {
            log.Fatal("video id can not be empty")
            return nil
          }
          if err := h.Find(id); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return nil
        },
      },
      {
        Name:  "info",
        Usage: "",
        Action: func(c *cli.Context) error {
          url := c.Args().Get(0)
          if url == "" {
            log.Fatal("video url can not be empty")
            return nil
          }
          if err := h.Info(url); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return nil
        },
      },
      {
        Name:  "scan",
        Usage: "",
        Action: func(c *cli.Context) error {
          if err := h.Scan(); err != nil {
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

func (h *VideosHandler) Find(id string) (err error) {
  log.Println(fmt.Sprintf("video %v info...", id))

  video, err := h.Repository.Find(id)
  if err != nil {
    return
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
  storageUrl := fmt.Sprintf(
    "%s/videos/%d/%d/%s.%s",
    common.GetEnvString(fmt.Sprintf("SCRAPER_STORAGE_URL_%v", video.Node)),
    i/233%50,
    i/89%50,
    video.Filehash,
    video.Extension,
  )

  log.Println("video info", localfile, storageUrl)

  return nil
}

func (h *VideosHandler) Info(url string) (err error) {
  log.Println(fmt.Sprintf("video %v info...", url))

  hash := sha1.Sum([]byte(url))
  urlSha1 := hex.EncodeToString(hash[:])
  video, err := h.Repository.Get(url, urlSha1)
  if err != nil {
    return
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
  storageUrl := fmt.Sprintf(
    "%s/videos/%d/%d/%s.%s",
    common.GetEnvString(fmt.Sprintf("SCRAPER_STORAGE_URL_%v", video.Node)),
    i/233%50,
    i/89%50,
    video.Filehash,
    video.Extension,
  )

  log.Println("video info", localfile, storageUrl)

  return nil
}

func (h *VideosHandler) Scan() (err error) {
  log.Println(fmt.Sprintf("media videos scan"))
  var args []string
  args = append(args, fmt.Sprintf("%s/videos/", common.GetEnvString("SCRAPER_STORAGE_PATH")))
  args = append(args, "-type")
  args = append(args, "f")
  cmd := exec.Command("/usr/bin/find", args...)
  stdout, err := cmd.StdoutPipe()
  cmd.Stderr = cmd.Stdout
  if err != nil {
    return err
  }
  if err = cmd.Start(); err != nil {
    return err
  }
  pid := cmd.Process.Pid
  defer func() {
    syscall.Kill(pid, syscall.SIGKILL)
  }()
  scanner := bufio.NewScanner(stdout)
  node := common.GetEnvInt("SCRAPER_STORAGE_NODE")
  for scanner.Scan() {
    path := scanner.Text()
    filename := filepath.Base(path)
    filehash := strings.TrimSuffix(filename, filepath.Ext(filename))
    video, err := h.Repository.GetByNodeAndFilehash(node, filehash)
    if errors.Is(err, gorm.ErrRecordNotFound) {
      log.Println("file not exists filehash", filehash)
      os.Remove(path)
      continue
    }
    if h.Repository.IsCorrupted(path) {
      h.Repository.Update(video, "status", 4)
      os.Remove(path)
      log.Println("video is corrupted", path)
    }
  }
  return
}

func (h *VideosHandler) Clean() (err error) {
  log.Println(fmt.Sprintf("media videos clean"))

  conditions := map[string]interface{}{
    "node":      common.GetEnvInt("SCRAPER_STORAGE_NODE"),
    "mime":      "video/quicktime",
    "is_synced": false,
  }
  videos := h.Repository.Ranking(
    []string{"id", "node", "filehash", "extension"},
    conditions,
    "size",
    1,
    1000,
  )
  for _, video := range videos {
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
    h.Repository.Update(video, "status", 4)
    h.Rdb.ZRem(h.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_VIDEOS, video.ID)
  }

  return nil
}

func (h *VideosHandler) Fix(limit int) (err error) {
  log.Println(fmt.Sprintf("media videos fix"))

  day := time.Now().UTC().Format("0102")

  conditions := map[string]interface{}{
    "node":      common.GetEnvInt("SCRAPER_STORAGE_NODE"),
    "is_synced": false,
  }
  videos := h.Repository.Ranking(
    []string{"id", "filehash"},
    conditions,
    "size",
    1,
    limit,
  )
  for _, video := range videos {
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
      h.Repository.Updates(video, map[string]interface{}{
        "cloud_url": video.CloudUrl,
        "is_synced": true,
      })
      os.Remove(localfile)
      h.Rdb.ZRem(h.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_VIDEOS, video.ID)
      h.Rdb.Del(h.Ctx, fmt.Sprintf(config.REDIS_KEY_MEDIA_VIDEOS, video.UrlSha1, day))
    }
  }

  return nil
}
