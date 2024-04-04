package models

import (
  "scraper.local/twitter-scraper/models/media"
  "gorm.io/gorm"
)

type Media struct{}

func NewMedia() *Media {
  return &Media{}
}

func (m *Media) AutoMigrate(db *gorm.DB) error {
  db.AutoMigrate(
    &media.Photo{},
    &media.Video{},
  )
  return nil
}
