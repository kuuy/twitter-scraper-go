package repositories

import (
  "database/sql"
  "encoding/json"
  "errors"
  "fmt"
  "github.com/nats-io/nats.go"
  "github.com/rs/xid"
  "gorm.io/datatypes"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/config"
  "scraper.local/twitter-scraper/models"
)

type PostsRepository struct {
  Db   *gorm.DB
  Nats *nats.Conn
}

func (r *PostsRepository) Count(conditions map[string]interface{}) int64 {
  var total int64
  query := r.Db.Model(&models.Post{})
  if _, ok := conditions["account"]; ok {
    subQuery := r.Db.Model(&models.User{}).Select([]string{"id"})
    subQuery.Where("account=?", conditions["account"].(string))
    query.Where("user_id IN(?)", subQuery)
  }
  if _, ok := conditions["status"]; ok {
    query.Where("status", conditions["status"].(int))
  } else {
    query.Where("status IN (1,2,3)")
  }
  query.Count(&total)
  return total
}

func (r *PostsRepository) Listings(conditions map[string]interface{}, current int, pageSize int) []*models.Post {
  var posts []*models.Post
  query := r.Db.Select([]string{
    "id",
    "user_id",
    "twitter_id",
    "status_id",
    "content",
    "media",
    "timestamp",
  })
  if _, ok := conditions["account"]; ok {
    subQuery := r.Db.Model(&models.User{}).Select([]string{"id"})
    subQuery.Where("account=?", conditions["account"].(string))
    query.Where("user_id IN(?)", subQuery)
  }
  if _, ok := conditions["timestamp"]; ok {
    query.Where("timestamp BETWEEN ? AND ?", conditions["timestamp"].([]int64)[0], conditions["timestamp"].([]int64)[1])
  }
  if _, ok := conditions["status"]; ok {
    query.Where("status", conditions["status"].(int))
  } else {
    query.Where("status IN (1,2,3)")
  }
  query.Order("timestamp desc")
  query.Offset((current - 1) * pageSize).Limit(pageSize).Find(&posts)
  return posts
}

func (r *PostsRepository) Find(id string) (entity *models.Post, err error) {
  err = r.Db.First(&entity, "id=?", id).Error
  return
}

func (r *PostsRepository) Get(twitterID int64) (entity *models.Post, err error) {
  err = r.Db.Where("twitter_id", twitterID).Take(&entity).Error
  return
}

func (r *PostsRepository) GetByLinkID(linkID int64) (entity *models.Post, err error) {
  err = r.Db.Where("twitter_id=@linkID OR status_id=@linkID", sql.Named("linkID", linkID)).Take(&entity).Error
  return
}

func (r *PostsRepository) Ranking(
  fields []string,
  conditions map[string]interface{},
  sortField string,
  sortType int,
  limit int,
) []*models.Post {
  var posts []*models.Post
  query := r.Db.Select(fields)
  if _, ok := conditions["user_id"]; ok {
    query.Where("user_id", conditions["user_id"].(string))
  }
  if _, ok := conditions["timestamp"]; ok {
    if sortType == 1 {
      query.Where("timestamp>?", conditions["timestamp"].(int64))
    } else if sortType == -1 {
      query.Where("timestamp<?", conditions["timestamp"].(int64))
    }
  }
  if _, ok := conditions["status"]; ok {
    switch conditions["status"].(type) {
    case []int:
      query.Where("status IN ?", conditions["status"].([]int))
    default:
      query.Where("status", conditions["status"].(int))
    }
  } else {
    query.Where("status=1")
  }
  if sortType == 1 {
    query.Order(fmt.Sprintf("%v ASC", sortField))
  } else if sortType == -1 {
    query.Order(fmt.Sprintf("%v DESC", sortField))
  }
  query.Limit(limit).Find(&posts)
  return posts
}

func (r *PostsRepository) IsExists(twitterID int64) bool {
  var entity *models.Post
  result := r.Db.Where("twitter_id", twitterID).Take(&entity)
  if errors.Is(result.Error, gorm.ErrRecordNotFound) {
    return false
  }
  return true
}

func (r *PostsRepository) Create(
  userID string,
  twitterID int64,
  statusID int64,
  content string,
  media datatypes.JSONMap,
  timestamp int64,
  status int,
) (id string, err error) {
  id = xid.New().String()
  entity := &models.Post{
    ID:        id,
    UserID:    userID,
    TwitterID: twitterID,
    StatusID:  statusID,
    Content:   content,
    Media:     media,
    Timestamp: timestamp,
    Status:    status,
  }
  err = r.Db.Create(&entity).Error
  if err == nil {
    data, _ := json.Marshal(map[string]interface{}{
      "id": id,
    })
    r.Nats.Publish(config.NATS_POSTS_CREATE, data)
    r.Nats.Flush()
  }
  return
}

func (r *PostsRepository) Update(post *models.Post, column string, value interface{}) (err error) {
  r.Db.Model(&post).Update(column, value)
  return nil
}

func (r *PostsRepository) Updates(post *models.Post, values map[string]interface{}) (err error) {
  err = r.Db.Model(&post).Updates(values).Error
  if err == nil {
    data, _ := json.Marshal(map[string]interface{}{
      "id": post.ID,
    })
    if status, ok := values["status"]; ok && status.(int) == 1 {
      r.Nats.Publish(config.NATS_POSTS_CREATE, data)
      r.Nats.Flush()
    }
  }
  return nil
}
