package media

import (
  "context"
  "fmt"
  "strconv"
  "time"

  "github.com/go-redis/redis/v8"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/config"
  models "scraper.local/twitter-scraper/models/media"
)

type PhotosRepository struct {
  Db  *gorm.DB
  Rdb *redis.Client
  Ctx context.Context
}

func (r *PhotosRepository) Count(conditions map[string]interface{}) int64 {
  var total int64
  query := r.Db.Model(&models.Photo{})
  if _, ok := conditions["status"]; ok {
    query.Where("status", conditions["status"].(int))
  } else {
    query.Where("status=1")
  }
  query.Count(&total)
  return total
}

func (r *PhotosRepository) Listings(conditions map[string]interface{}, current int, pageSize int) []*models.Photo {
  var orders []*models.Photo
  query := r.Db.Select([]string{
    "id",
    "url",
    "mime",
    "size",
    "node",
    "filehash",
    "extension",
    "timestamp",
  })
  if _, ok := conditions["width"]; ok {
    query.Where("width=?", conditions["width"].(int))
  }
  if _, ok := conditions["is_synced"]; ok {
    query.Where("is_synced", conditions["is_synced"].(bool))
  }
  if _, ok := conditions["status"]; ok {
    query.Where("status", conditions["status"].(int))
  } else {
    query.Where("status=1")
  }
  query.Order("timestamp desc")
  query.Offset((current - 1) * pageSize).Limit(pageSize).Find(&orders)
  return orders
}

func (r *PhotosRepository) Ranking(
  fields []string,
  conditions map[string]interface{},
  sortField string,
  sortType int,
  limit int,
) []*models.Photo {
  var photos []*models.Photo
  query := r.Db.Select(fields)
  if _, ok := conditions["ids"]; ok {
    query.Where("id IN ?", conditions["ids"].([]string))
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
  query.Limit(limit).Find(&photos)
  return photos
}

func (r *PhotosRepository) Find(id string) (photo *models.Photo, err error) {
  err = r.Db.First(&photo, "id=?", id).Error
  return
}

func (r *PhotosRepository) Get(url string, urlSha1 string) (photo *models.Photo, err error) {
  day := time.Now().UTC().Format("0102")
  redisKey := fmt.Sprintf(config.REDIS_KEY_MEDIA_PHOTOS, urlSha1, day)
  values, _ := r.Rdb.HMGet(r.Ctx, redisKey, []string{
    "filehash",
    "extension",
    "is_synced",
    "cloud_url",
  }...).Result()
  if values[0] != nil {
    photo = &models.Photo{
      Url:     url,
      UrlSha1: urlSha1,
      Status:  1,
    }
    photo.Filehash = values[0].(string)
    photo.Extension = values[1].(string)
    photo.IsSynced, _ = strconv.ParseBool(values[2].(string))
    photo.CloudUrl = values[3].(string)
    return
  }
  err = r.Db.Where("url_sha1=? AND url=?", urlSha1, url).Take(&photo).Error
  if err == nil {
    r.Rdb.HMSet(
      r.Ctx,
      redisKey,
      map[string]interface{}{
        "filehash":  photo.Filehash,
        "extension": photo.Extension,
        "is_synced": photo.IsSynced,
        "cloud_url": photo.CloudUrl,
      },
    )
    ttl, _ := r.Rdb.TTL(r.Ctx, redisKey).Result()
    if -1 == ttl.Nanoseconds() {
      r.Rdb.Expire(r.Ctx, redisKey, time.Hour*24)
    }
  }
  return
}

func (r *PhotosRepository) IsExists(url string, urlSha1 string) bool {
  var photo models.Photo
  err := r.Db.Where("url_sha1=? AND url=?", urlSha1, url).Take(&photo).Error
  if err != nil {
    return false
  }
  return true
}

func (r *PhotosRepository) Update(photo *models.Photo, column string, value interface{}) (err error) {
  r.Db.Model(&photo).Update(column, value)
  return nil
}

func (r *PhotosRepository) Updates(photo *models.Photo, values map[string]interface{}) (err error) {
  r.Db.Model(&photo).Updates(values)
  return nil
}
