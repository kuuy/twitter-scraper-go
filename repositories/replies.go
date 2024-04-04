package repositories

import (
  "database/sql"
  "encoding/json"
  "errors"
  "fmt"
  "log"

  "github.com/nats-io/nats.go"
  "github.com/rs/xid"
  "gorm.io/datatypes"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/config"
  "scraper.local/twitter-scraper/models"
)

type RepliesRepository struct {
  Db   *gorm.DB
  Nats *nats.Conn
}

func (r *RepliesRepository) Count(conditions map[string]interface{}) int64 {
  var total int64
  query := r.Db.Model(&models.Reply{})
  if _, ok := conditions["user_id"]; ok {
    query.Where("user_id", conditions["user_id"].(string))
  }
  if _, ok := conditions["post_id"]; ok {
    query.Where("post_id", conditions["post_id"].(string))
  }
  if _, ok := conditions["link_id"]; ok {
    subQuery := r.Db.Model(&models.Post{}).Select([]string{"id"})
    subQuery.Where("twitter_id=@linkID OR status_id=@linkID", sql.Named("linkID", conditions["link_id"].(int64)))
    query.Where("post_id IN(?)", subQuery)
  }
  if _, ok := conditions["status"]; ok {
    query.Where("status", conditions["status"].(int))
  } else {
    query.Where("status IN (1,2,3)")
  }
  query.Count(&total)
  return total
}

func (r *RepliesRepository) Listings(conditions map[string]interface{}, current int, pageSize int) []*models.Reply {
  var replies []*models.Reply
  query := r.Db.Select([]string{
    "id",
    "user_id",
    "post_id",
    "twitter_id",
    "content",
    "media",
    "timestamp",
  })
  if _, ok := conditions["user_id"]; ok {
    query.Where("user_id", conditions["user_id"].(string))
  }
  if _, ok := conditions["post_id"]; ok {
    query.Where("post_id", conditions["post_id"].(string))
  }
  if _, ok := conditions["link_id"]; ok {
    subQuery := r.Db.Model(&models.Post{}).Select([]string{"id"})
    subQuery.Where("twitter_id=@linkID OR status_id=@linkID", sql.Named("linkID", conditions["link_id"].(int64)))
    query.Where("post_id IN(?)", subQuery)
  }
  if _, ok := conditions["status"]; ok {
    query.Where("status", conditions["status"].(int))
  } else {
    query.Where("status IN (1,2,3)")
  }
  query.Order("timestamp desc")
  query.Offset((current - 1) * pageSize).Limit(pageSize).Find(&replies)
  return replies
}

func (r *RepliesRepository) Find(id string) (entity *models.Reply, err error) {
  err = r.Db.First(&entity, "id=?", id).Error
  return
}

func (r *RepliesRepository) Get(twitterID int64) (entity *models.Reply, err error) {
  err = r.Db.Where("twitter_id", twitterID).Take(&entity).Error
  return
}

func (r *RepliesRepository) Ranking(
  fields []string,
  conditions map[string]interface{},
  sortField string,
  sortType int,
  limit int,
) []*models.Reply {
  var replies []*models.Reply
  query := r.Db.Select(fields)
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
      query.Where("status IN ?", conditions["status"])
    default:
      query.Where("status", conditions["status"])
    }
  } else {
    query.Where("status=1")
  }
  if sortType == 1 {
    query.Order(fmt.Sprintf("%v ASC", sortField))
  } else if sortType == -1 {
    query.Order(fmt.Sprintf("%v DESC", sortField))
  }
  log.Println("limit", limit)
  query.Limit(limit).Find(&replies)
  return replies
}

func (r *RepliesRepository) IsExists(twitterID int64) bool {
  var entity *models.Reply
  result := r.Db.Where("twitter_id", twitterID).Take(&entity)
  if errors.Is(result.Error, gorm.ErrRecordNotFound) {
    return false
  }
  return true
}

func (r *RepliesRepository) Create(
  userID string,
  postID string,
  twitterID int64,
  content string,
  media datatypes.JSONMap,
  timestamp int64,
  status int,
) (id string, err error) {
  id = xid.New().String()
  entity := &models.Reply{
    ID:        id,
    UserID:    userID,
    PostID:    postID,
    TwitterID: twitterID,
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
    r.Nats.Publish(config.NATS_REPLIES_CREATE, data)
    r.Nats.Flush()
  }
  return
}

func (r *RepliesRepository) Update(reply *models.Reply, column string, value interface{}) (err error) {
  r.Db.Model(&reply).Update(column, value)
  return nil
}

func (r *RepliesRepository) Updates(reply *models.Reply, values map[string]interface{}) (err error) {
  err = r.Db.Model(&reply).Updates(values).Error
  if err == nil {
    data, _ := json.Marshal(map[string]interface{}{
      "id": reply.ID,
    })
    if status, ok := values["status"]; ok && status.(int) == 1 {
      r.Nats.Publish(config.NATS_REPLIES_CREATE, data)
      r.Nats.Flush()
    }
  }
  return nil
}
