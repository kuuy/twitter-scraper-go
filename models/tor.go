package models

import (
  "scraper.local/twitter-scraper/models/tor"
  "gorm.io/gorm"
)

type Tor struct{}

func NewTor() *Tor {
  return &Tor{}
}

func (m *Tor) AutoMigrate(db *gorm.DB) error {
  db.AutoMigrate(
    &tor.Bridge{},
  )
  return nil
}
