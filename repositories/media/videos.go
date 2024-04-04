package media

import (
  "bufio"
  "context"
  "fmt"
  "log"
  "os/exec"
  "strconv"
  "strings"
  "syscall"
  "time"

  "github.com/go-redis/redis/v8"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/config"
  models "scraper.local/twitter-scraper/models/media"
)

type VideosRepository struct {
  Db  *gorm.DB
  Rdb *redis.Client
  Ctx context.Context
}

func (r *VideosRepository) Count(conditions map[string]interface{}) int64 {
  var total int64
  query := r.Db.Model(&models.Video{})
  if _, ok := conditions["status"]; ok {
    query.Where("status", conditions["status"].(int))
  } else {
    query.Where("status=1")
  }
  query.Count(&total)
  return total
}

func (r *VideosRepository) Listings(conditions map[string]interface{}, current int, pageSize int) []*models.Video {
  var videos []*models.Video
  query := r.Db.Select([]string{
    "id",
    "url",
    "mime",
    "size",
    "filehash",
    "extension",
    "timestamp",
  })
  if _, ok := conditions["status"]; ok {
    query.Where("status", conditions["status"].(int))
  } else {
    query.Where("status=1")
  }
  query.Order("timestamp desc")
  query.Offset((current - 1) * pageSize).Limit(pageSize).Find(&videos)
  return videos
}

func (r *VideosRepository) Ranking(
  fields []string,
  conditions map[string]interface{},
  sortField string,
  sortType int,
  limit int,
) []*models.Video {
  var videos []*models.Video
  query := r.Db.Select(fields)
  if _, ok := conditions["ids"]; ok {
    query.Where("id IN ?", conditions["ids"].([]string))
  }
  if _, ok := conditions["mime"]; ok {
    query.Where("mime", conditions["mime"].(string))
  }
  if _, ok := conditions["node"]; ok {
    query.Where("node", conditions["node"].(int))
  }
  if _, ok := conditions["is_synced"]; ok {
    query.Where("is_synced", conditions["is_synced"].(bool))
  }
  if _, ok := conditions["status"]; ok {
    query.Where("status", conditions["status"].(int))
  } else {
    query.Where("status=1")
  }
  if sortType == 1 {
    query.Order(fmt.Sprintf("%v ASC", sortField))
  } else if sortType == -1 {
    query.Order(fmt.Sprintf("%v DESC", sortField))
  }
  query.Limit(limit).Find(&videos)
  return videos
}

func (r *VideosRepository) Find(id string) (video *models.Video, err error) {
  err = r.Db.First(&video, "id=?", id).Error
  return
}

func (r *VideosRepository) Get(url string, urlSha1 string) (video *models.Video, err error) {
  day := time.Now().UTC().Format("0102")
  redisKey := fmt.Sprintf(config.REDIS_KEY_MEDIA_VIDEOS, urlSha1, day)
  values, _ := r.Rdb.HMGet(r.Ctx, redisKey, []string{
    "filehash",
    "extension",
    "is_synced",
    "cloud_url",
  }...).Result()
  if values[0] != nil {
    video = &models.Video{
      Url:     url,
      UrlSha1: urlSha1,
      Status:  1,
    }
    video.Filehash = values[0].(string)
    video.Extension = values[1].(string)
    video.IsSynced, _ = strconv.ParseBool(values[2].(string))
    video.CloudUrl = values[3].(string)
    return
  }
  err = r.Db.Where("url_sha1=? AND url=?", urlSha1, url).Take(&video).Error
  if err == nil {
    r.Rdb.HMSet(
      r.Ctx,
      redisKey,
      map[string]interface{}{
        "filehash":  video.Filehash,
        "extension": video.Extension,
        "is_synced": video.IsSynced,
        "cloud_url": video.CloudUrl,
      },
    )
    ttl, _ := r.Rdb.TTL(r.Ctx, redisKey).Result()
    if -1 == ttl.Nanoseconds() {
      r.Rdb.Expire(r.Ctx, redisKey, time.Hour*24)
    }
  }
  return
}

func (r *VideosRepository) GetByNodeAndFilehash(node int, filehash string) (video *models.Video, err error) {
  err = r.Db.Where("node=? AND filehash=?", node, filehash).Take(&video).Error
  return
}

func (r *VideosRepository) IsExists(url string, urlSha1 string) bool {
  var video models.Video
  err := r.Db.Where("url_sha1=? AND url=?", urlSha1, url).Take(&video).Error
  if err != nil {
    return false
  }
  return true
}

func (r *VideosRepository) IsCorrupted(path string) bool {
  var args []string
  args = append(args, "-v")
  args = append(args, "error")
  args = append(args, "-i")
  args = append(args, path)
  cmd := exec.Command("/usr/bin/ffmpeg", args...)
  stdout, err := cmd.StdoutPipe()
  cmd.Stderr = cmd.Stdout
  if err != nil {
    log.Println("error occurred", err.Error())
    return false
  }
  if err = cmd.Start(); err != nil {
    log.Println("error occurred", err.Error())
    return false
  }
  pid := cmd.Process.Pid
  defer func() {
    syscall.Kill(pid, syscall.SIGKILL)
  }()
  scanner := bufio.NewScanner(stdout)
  for scanner.Scan() {
    content := scanner.Text()
    if strings.Contains(content, "Invalid NAL unit size") {
      return true
    }
  }
  return false
}

func (r *VideosRepository) Update(video *models.Video, column string, value interface{}) (err error) {
  r.Db.Model(&video).Update(column, value)
  return nil
}

func (r *VideosRepository) Updates(video *models.Video, values map[string]interface{}) (err error) {
  r.Db.Model(&video).Updates(values)
  return nil
}
