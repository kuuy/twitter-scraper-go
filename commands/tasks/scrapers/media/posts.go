package media

import (
  "context"
  "crypto/sha1"
  "encoding/hex"
  "encoding/json"
  "errors"
  "fmt"
  "log"
  "sort"
  "strconv"
  "strings"
  "time"

  "github.com/go-redis/redis/v8"
  "github.com/nats-io/nats.go"
  "github.com/urfave/cli/v2"
  "golang.org/x/sys/unix"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/config"
  "scraper.local/twitter-scraper/repositories"
  mediaRepositories "scraper.local/twitter-scraper/repositories/media"
  "scraper.local/twitter-scraper/repositories/scrapers"
  scrapersRepositories "scraper.local/twitter-scraper/repositories/scrapers/media"
)

type PostsHandler struct {
  Db                       *gorm.DB
  Rdb                      *redis.Client
  Ctx                      context.Context
  Nats                     *nats.Conn
  Repository               *repositories.TasksRepository
  PostsRepository          *repositories.PostsRepository
  PhotosRepository         *mediaRepositories.PhotosRepository
  VideosRepository         *mediaRepositories.VideosRepository
  ScrapersPhotosRepository *scrapersRepositories.PhotosRepository
  ScrapersVideosRepository *scrapersRepositories.VideosRepository
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
      h.Repository = &repositories.TasksRepository{
        Db: h.Db,
      }
      h.PostsRepository = &repositories.PostsRepository{
        Db: h.Db,
      }
      h.PhotosRepository = &mediaRepositories.PhotosRepository{
        Db: h.Db,
      }
      h.VideosRepository = &mediaRepositories.VideosRepository{
        Db: h.Db,
      }
      h.ScrapersPhotosRepository = &scrapersRepositories.PhotosRepository{
        Db: h.Db,
      }
      h.ScrapersVideosRepository = &scrapersRepositories.VideosRepository{
        Db: h.Db,
      }
      return nil
    },
    Subcommands: []*cli.Command{
      {
        Name:  "init",
        Usage: "",
        Action: func(c *cli.Context) (err error) {
          limit, _ := strconv.Atoi(c.Args().Get(0))
          if limit < 20 {
            limit = 20
          }
          if err := h.Init(limit); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return
        },
      },
      {
        Name:  "apply",
        Usage: "",
        Action: func(c *cli.Context) (err error) {
          id := c.Args().Get(0)
          if id == "" {
            log.Fatal("twitter posts id can not be empty")
            return
          }
          if err = h.Apply(id); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return
        },
      },
      {
        Name:  "process",
        Usage: "",
        Action: func(c *cli.Context) error {
          limit, _ := strconv.Atoi(c.Args().Get(0))
          if limit < 20 {
            limit = 20
          }
          if err := h.Process(limit); err != nil {
            return cli.Exit(err.Error(), 1)
          }
          return nil
        },
      },
    },
  }
}

func (h *PostsHandler) Init(limit int) (err error) {
  log.Println(fmt.Sprintf("tasks media posts init..."))
  conditions := map[string]interface{}{}
  posts := h.PostsRepository.Ranking(
    []string{"id", "timestamp"},
    conditions,
    "timestamp",
    -1,
    limit,
  )
  for _, post := range posts {
    name := fmt.Sprintf("%v@media.posts", post.ID)
    action := config.TASK_ACTION_SCRAPERS_MEDIA_POSTS
    params := map[string]interface{}{
      "id": post.ID,
    }
    h.Repository.Apply(name, action, params)
  }
  return
}

func (h *PostsHandler) Apply(id string) (err error) {
  log.Println(fmt.Sprintf("twitter tasks media posts apply..."))
  post, err := h.PostsRepository.Find(id)
  if errors.Is(err, gorm.ErrRecordNotFound) {
    return
  }
  name := fmt.Sprintf("%v@media", post.ID)
  action := config.TASK_ACTION_SCRAPERS_MEDIA_POSTS
  params := map[string]interface{}{
    "id": post.ID,
  }
  return h.Repository.Apply(name, action, params)
}

func (h *PostsHandler) Process(limit int) error {
  log.Println(fmt.Sprintf("tasks media posts processing..."))

  var stat unix.Statfs_t
  unix.Statfs(common.GetEnvString("SCRAPER_STORAGE_PATH"), &stat)
  freeGB := int(stat.Bavail * uint64(stat.Bsize) / 1073741824)

  tasks := h.Repository.Ranking(
    []string{"id", "params", "timestamp"},
    map[string]interface{}{
      "action": config.TASK_ACTION_SCRAPERS_MEDIA_POSTS,
    },
    "timestamp",
    -1,
    limit,
  )
  for _, task := range tasks {
    mutex := common.NewMutex(
      h.Rdb,
      h.Ctx,
      fmt.Sprintf(config.LOCKS_TASKS_SCRAPERS_MEDIA_POSTS_PROCESS, task.ID),
    )
    if !mutex.Lock(15 * time.Minute) {
      continue
    }

    timestamp := time.Now().UnixMicro()
    if timestamp-task.Timestamp < 900*1e6 {
      log.Println("waiting for next process")
      mutex.Unlock()
      continue
    }
    id := task.Params["id"].(string)
    post, err := h.PostsRepository.Find(id)
    if errors.Is(err, gorm.ErrRecordNotFound) {
      h.Repository.Delete(task.ID)
    }
    if err != nil {
      mutex.Unlock()
      continue
    }

    var mediaInfo *scrapers.MediaInfo
    buf, _ := post.Media.MarshalJSON()
    json.Unmarshal(buf, &mediaInfo)

    if mediaInfo.Photos != nil && freeGB > common.GetEnvInt("SCRAPER_DISK_MIN_PHOTOS_GB") {
      for _, item := range mediaInfo.Photos {
        if !strings.HasPrefix(item.Url, "https://") {
          h.Repository.Update(task, "status", 4)
          continue
        }
        url := item.Url
        hash := sha1.Sum([]byte(url))
        urlSha1 := hex.EncodeToString(hash[:])
        if !h.PhotosRepository.IsExists(url, urlSha1) {
          if err := h.ScrapersPhotosRepository.Download(url, urlSha1); err != nil {
            log.Println("error", err)
            h.Repository.Update(task, "status", 3)
            continue
          }
        }
      }
    }

    if mediaInfo.Videos != nil && freeGB > common.GetEnvInt("SCRAPER_DISK_MIN_VIDEOS_GB") {
      for _, item := range mediaInfo.Videos {
        sort.Slice(item.Variants, func(i, j int) bool {
          return item.Variants[i].Bitrate > item.Variants[j].Bitrate
        })
        if item.Variants[0].Bitrate == 0 {
          continue
        }
        if strings.HasPrefix(item.Cover, "https://") {
          url := item.Cover
          hash := sha1.Sum([]byte(url))
          urlSha1 := hex.EncodeToString(hash[:])
          if !h.PhotosRepository.IsExists(url, urlSha1) {
            if err := h.ScrapersPhotosRepository.Download(url, urlSha1); err != nil {
              log.Println("error", err)
              h.Repository.Update(task, "status", 3)
              continue
            }
          }
        }
        url := item.Variants[0].Url
        hash := sha1.Sum([]byte(url))
        urlSha1 := hex.EncodeToString(hash[:])
        if !h.VideosRepository.IsExists(url, urlSha1) {
          if err := h.ScrapersVideosRepository.Download(url, urlSha1); err != nil {
            log.Println("error", err)
            h.Repository.Update(task, "status", 3)
            continue
          }
        }
      }
    }

    h.Repository.Update(task, "status", 2)

    mutex.Unlock()
  }
  return nil
}
