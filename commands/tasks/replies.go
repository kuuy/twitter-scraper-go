package tasks

import (
  "context"
  "crypto/sha1"
  "encoding/hex"
  "encoding/json"
  "fmt"
  "log"
  "sort"
  "strconv"
  "time"

  "github.com/go-redis/redis/v8"
  "github.com/nats-io/nats.go"
  "github.com/urfave/cli/v2"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/config"
  "scraper.local/twitter-scraper/repositories"
  mediaRepositories "scraper.local/twitter-scraper/repositories/media"
)

type RepliesHandler struct {
  Db                    *gorm.DB
  Rdb                   *redis.Client
  Ctx                   context.Context
  Nats                  *nats.Conn
  Repository            *repositories.RepliesRepository
  UsersRepository       *repositories.UsersRepository
  MediaPhotosRepository *mediaRepositories.PhotosRepository
  MediaVideosRepository *mediaRepositories.VideosRepository
}

func NewRepliesCommand() *cli.Command {
  var h RepliesHandler
  return &cli.Command{
    Name:  "replies",
    Usage: "",
    Before: func(c *cli.Context) error {
      h = RepliesHandler{
        Db:  common.NewDB(),
        Rdb: common.NewRedis(),
        Ctx: context.Background(),
      }
      h.Repository = &repositories.RepliesRepository{
        Db: h.Db,
      }
      h.UsersRepository = &repositories.UsersRepository{
        Db: h.Db,
      }
      h.MediaPhotosRepository = &mediaRepositories.PhotosRepository{
        Db: h.Db,
      }
      h.MediaVideosRepository = &mediaRepositories.VideosRepository{
        Db: h.Db,
      }
      return nil
    },
    Subcommands: []*cli.Command{
      {
        Name:  "flush",
        Usage: "",
        Action: func(c *cli.Context) error {
          limit, _ := strconv.Atoi(c.Args().Get(0))
          if limit < 20 {
            limit = 20
          }
          if err := h.Flush(limit); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return nil
        },
      },
    },
  }
}

func (h *RepliesHandler) Flush(limit int) error {
  log.Println("replies flushing...")
  replies := h.Repository.Ranking(
    []string{
      "id",
      "user_id",
      "media",
    },
    map[string]interface{}{
      "status": []int{1, 2},
    },
    "timestamp",
    1,
    limit,
  )
  for _, reply := range replies {
    mutex := common.NewMutex(
      h.Rdb,
      h.Ctx,
      fmt.Sprintf(config.LOCKS_TASKS_REPLIES_FLUSH, reply.ID),
    )
    if !mutex.Lock(30 * time.Second) {
      return nil
    }

    var mediaInfo *MediaInfo
    buf, _ := reply.Media.MarshalJSON()
    json.Unmarshal(buf, &mediaInfo)

    isSynced := true

    if mediaInfo.Photos != nil {
      for _, item := range mediaInfo.Photos {
        url := item.Url
        hash := sha1.Sum([]byte(url))
        urlSha1 := hex.EncodeToString(hash[:])
        if photo, err := h.MediaPhotosRepository.Get(url, urlSha1); err == nil && photo.Status == 1 {
          if photo.Status != 1 {
            h.Repository.Update(reply, "status", 5)
            continue
          }
          if !photo.IsSynced {
            isSynced = false
          }
        } else {
          isSynced = false
        }
      }
    }

    if mediaInfo.Videos != nil {
      for _, item := range mediaInfo.Videos {
        sort.Slice(item.Variants, func(i, j int) bool {
          return item.Variants[i].Bitrate > item.Variants[j].Bitrate
        })
        if item.Variants[0].Bitrate == 0 {
          continue
        }
        url := item.Cover
        hash := sha1.Sum([]byte(url))
        urlSha1 := hex.EncodeToString(hash[:])
        if photo, err := h.MediaPhotosRepository.Get(url, urlSha1); err == nil && photo.Status == 1 {
          if photo.Status != 1 {
            h.Repository.Update(reply, "status", 5)
            continue
          }
          if !photo.IsSynced {
            isSynced = false
          }
        } else {
          isSynced = false
        }
        url = item.Variants[0].Url
        hash = sha1.Sum([]byte(url))
        urlSha1 = hex.EncodeToString(hash[:])
        if video, err := h.MediaVideosRepository.Get(url, urlSha1); err == nil && video.Status == 1 {
          if video.Status != 1 {
            h.Repository.Update(reply, "status", 5)
            continue
          }
          if !video.IsSynced {
            isSynced = false
          }
        } else {
          isSynced = false
        }
      }
    }

    if user, err := h.UsersRepository.Find(reply.UserID); err == nil {
      url := user.Avatar
      hash := sha1.Sum([]byte(url))
      urlSha1 := hex.EncodeToString(hash[:])
      if photo, err := h.MediaPhotosRepository.Get(url, urlSha1); err == nil && photo.Status == 1 {
        if photo.Status != 1 {
          h.Repository.Update(reply, "status", 5)
          continue
        }
        if !photo.IsSynced {
          isSynced = false
        }
      } else {
        isSynced = false
      }
    }
    log.Println("reply", reply.ID, isSynced)

    if isSynced {
      h.Repository.Update(reply, "status", 3)
    }

    mutex.Unlock()
  }
  return nil
}
