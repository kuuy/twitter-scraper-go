package models

import (
  "gorm.io/datatypes"
  "time"
)

type Post struct {
  ID        string            `gorm:"size:20;primaryKey"`
  UserID    string            `gorm:"size:20;not null;index:idx_twitter_users_posts,priority:1"`
  TwitterID int64             `gorm:"not null;uniqueIndex"`
  StatusID  int64             `gorm:"not null"`
  Content   string            `gorm:"size:5000;not null"`
  Media     datatypes.JSONMap `gorm:"not null"`
  Timestamp int64             `gorm:"not null;index:idx_twitter_posts,priority:1"`
  Status    int               `gorm:"not null;index:idx_twitter_posts,priority:2;index:idx_twitter_users_posts,priority:2"`
  CreatedAt time.Time         `gorm:"not null;index:idx_twitter_users_posts,priority:3"`
  UpdatedAt time.Time         `gorm:"not null"`
}

func (m *Post) TableName() string {
  return "twitter_posts"
}
