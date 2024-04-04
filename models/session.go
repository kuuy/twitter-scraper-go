package models

import (
  "gorm.io/datatypes"
  "time"
)

type Session struct {
  ID          string            `gorm:"size:20;primaryKey"`
  Account     string            `gorm:"size:50;not null;uniqueIndex"`
  TwitterID   int64             `gorm:"not null"`
  Node        int               `gorm:"not null"`
  Agent       string            `gorm:"size:155;not null"`
  Cookie      string            `gorm:"size:2000;not null"`
  Slot        int               `gorm:"not null"`
  Data        datatypes.JSONMap `gorm:"not null"`
  FlushedAt   int64             `gorm:"not null"`
  UnblockedAt int64             `gorm:"not null"`
  Timestamp   int64             `gorm:"not null"`
  Status      int               `gorm:"not null"`
  CreatedAt   time.Time         `gorm:"not null"`
  UpdatedAt   time.Time         `gorm:"not null"`
}

func (m *Session) TableName() string {
  return "twitter_sessions"
}
