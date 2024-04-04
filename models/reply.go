package models

import (
  "gorm.io/datatypes"
  "time"
)

type Reply struct {
  ID        string            `gorm:"size:20;primaryKey"`
  UserID    string            `gorm:"size:20;not null"`
  PostID    string            `gorm:"size:20;not null;index:idx_twitter_replies,priority:2"`
  TwitterID int64             `gorm:"not null;uniqueIndex"`
  Content   string            `gorm:"size:5000;not null"`
  Media     datatypes.JSONMap `gorm:"not null"`
  Timestamp int64             `gorm:"not null;index:idx_twitter_replies_scan,priority:1;index:idx_twitter_replies,priority:1"`
  Status    int               `gorm:"not null;index:idx_twitter_replies_scan,priority:2;index:idx_twitter_replies,priority:3"`
  CreatedAt time.Time         `gorm:"not null;index"`
  UpdatedAt time.Time         `gorm:"not null"`
}

func (m *Reply) TableName() string {
  return "twitter_replies"
}
