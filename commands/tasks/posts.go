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

type PostsHandler struct {
  Db                    *gorm.DB
  Rdb                   *redis.Client
  Ctx                   context.Context
  Nats                  *nats.Conn
  Repository            *repositories.PostsRepository
  UsersRepository       *repositories.UsersRepository
  MediaPhotosRepository *mediaRepositories.PhotosRepository
  MediaVideosRepository *mediaRepositories.VideosRepository
}

func NewPostsCommand() *cli.Command {
  var h PostsHandler
  return &cli.Command{
    Name:  "posts",
    Usage: "",
    Before: func(c *cli.Context) error {
      h = PostsHandler{
        Db:  common.NewDB(),
        Rdb: common.NewRedis(),
        Ctx: context.Background(),
      }
      h.Repository = &repositories.PostsRepository{
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

func (h *PostsHandler) Flush(limit int) error {
  log.Println("posts flushing...")
  posts := h.Repository.Ranking(
    []string{
      "id",
      "user_id",
      "content",
      "media",
    },
    map[string]interface{}{
      "status": []int{1, 2},
    },
    "timestamp",
    1,
    limit,
  )
  for _, post := range posts {
    mutex := common.NewMutex(
      h.Rdb,
      h.Ctx,
      fmt.Sprintf(config.LOCKS_TASKS_POSTS_FLUSH, post.ID),
    )
    if !mutex.Lock(30 * time.Second) {
      return nil
    }

    var mediaInfo *MediaInfo
    buf, _ := post.Media.MarshalJSON()
    json.Unmarshal(buf, &mediaInfo)

    isSynced := true

    if mediaInfo.Photos != nil {
      for _, item := range mediaInfo.Photos {
        url := item.Url
        hash := sha1.Sum([]byte(url))
        urlSha1 := hex.EncodeToString(hash[:])
        if photo, err := h.MediaPhotosRepository.Get(url, urlSha1); err == nil && photo.Status == 1 {
          if photo.Status != 1 {
            h.Repository.Update(post, "status", 5)
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
            h.Repository.Update(post, "status", 5)
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
            h.Repository.Update(post, "status", 5)
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

    if user, err := h.UsersRepository.Find(post.UserID); err == nil {
      url := user.Avatar
      hash := sha1.Sum([]byte(url))
      urlSha1 := hex.EncodeToString(hash[:])
      if photo, err := h.MediaPhotosRepository.Get(url, urlSha1); err == nil && photo.Status == 1 {
        if photo.Status != 1 {
          h.Repository.Update(post, "status", 5)
          continue
        }
        if !photo.IsSynced {
          isSynced = false
        }
      } else {
        isSynced = false
      }
    }
    log.Println("post", post.ID, isSynced)

    if isSynced {
      h.Repository.Update(post, "status", 3)
    }

    mutex.Unlock()
  }
  return nil
}
