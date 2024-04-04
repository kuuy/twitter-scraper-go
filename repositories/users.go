package repositories

import (
  "encoding/json"
  "errors"
  "fmt"
  "github.com/nats-io/nats.go"
  "github.com/rs/xid"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/config"
  "scraper.local/twitter-scraper/models"
)

type UsersRepository struct {
  Db   *gorm.DB
  Nats *nats.Conn
}

func (r *UsersRepository) Find(id string) (entity *models.User, err error) {
  err = r.Db.First(&entity, "id=?", id).Error
  return
}

func (r *UsersRepository) Get(account string) (entity *models.User, err error) {
  err = r.Db.Where("account", account).Take(&entity).Error
  return
}

func (r *UsersRepository) GetByUserID(userID int64) (entity *models.User, err error) {
  err = r.Db.Where("user_id", userID).Take(&entity).Error
  return
}

func (r *UsersRepository) Ranking(
  fields []string,
  conditions map[string]interface{},
  sortField string,
  sortType int,
  limit int,
) []*models.User {
  var users []*models.User
  query := r.Db.Select(fields)
  if _, ok := conditions["timestamp"]; ok {
    if sortType == 1 {
      query.Where("timestamp>?", conditions["timestamp"].(int64))
    } else if sortType == -1 {
      query.Where("timestamp<?", conditions["timestamp"].(int64))
    }
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
  query.Limit(limit).Find(&users)
  return users
}

func (r *UsersRepository) IsExists(twitterID int64) bool {
  var entity *models.User
  result := r.Db.Where("twitter_id", twitterID).Take(&entity)
  if errors.Is(result.Error, gorm.ErrRecordNotFound) {
    return false
  }
  return true
}

func (r *UsersRepository) Create(
  account string,
  userID int64,
  name string,
  description string,
  avatar string,
  favouritesCount int,
  followersCount int,
  friendsCount int,
  listedCount int,
  mediaCount int,
  timestamp int64,
) (id string, err error) {
  id = xid.New().String()
  entity := &models.User{
    ID:              id,
    Account:         account,
    UserID:          userID,
    Name:            name,
    Description:     description,
    Avatar:          avatar,
    FavouritesCount: favouritesCount,
    FollowersCount:  followersCount,
    FriendsCount:    friendsCount,
    ListedCount:     listedCount,
    MediaCount:      mediaCount,
    Timestamp:       timestamp,
    Status:          1,
  }
  err = r.Db.Create(&entity).Error
  if err == nil {
    data, _ := json.Marshal(map[string]interface{}{
      "id": id,
    })
    r.Nats.Publish(config.NATS_USERS_CREATE, data)
    r.Nats.Flush()
  }
  return
}

func (r *UsersRepository) Update(user *models.User, column string, value interface{}) (err error) {
  r.Db.Model(&user).Update(column, value)
  return nil
}

func (r *UsersRepository) Updates(user *models.User, values map[string]interface{}) (err error) {
  r.Db.Model(&user).Updates(values)
  return nil
}
