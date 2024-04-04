package repositories

import (
  "errors"

  "github.com/rs/xid"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/models"
)

type AdminsRepository struct {
  Db *gorm.DB
}

func (r *AdminsRepository) Find(id string) (*models.Admin, error) {
  var entity *models.Admin
  result := r.Db.First(&entity, "id=?", id)
  if errors.Is(result.Error, gorm.ErrRecordNotFound) {
    return nil, result.Error
  }
  return entity, nil
}

func (r *AdminsRepository) Get(account string) *models.Admin {
  var entity models.Admin
  result := r.Db.Where(
    "account=?",
    account,
  ).Take(&entity)
  if errors.Is(result.Error, gorm.ErrRecordNotFound) {
    return nil
  }

  return &entity
}

func (r *AdminsRepository) Create(account string, password string) error {
  var entity models.Admin
  result := r.Db.Where(
    "account=?",
    account,
  ).Take(&entity)
  if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
    return errors.New("admin already exists")
  }
  salt := common.GenerateSalt(16)
  hashedPassword := common.GeneratePassword(password, salt)

  entity = models.Admin{
    ID:       xid.New().String(),
    Account:  account,
    Password: hashedPassword,
    Salt:     salt,
    Status:   1,
  }
  r.Db.Create(&entity)

  return nil
}
