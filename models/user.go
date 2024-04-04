package models

import (
  "time"
)

type User struct {
  ID              string    `gorm:"size:20;primaryKey"`
  Account         string    `gorm:"size:50;not null;uniqueIndex"`
  UserID          int64     `gorm:"not null;uniqueIndex"`
  Name            string    `gorm:"size:50;not null"`
  Description     string    `gorm:"size:500;not null"`
  Avatar          string    `gorm:"size:200;not null"`
  FavouritesCount int       `gorm:"not null"`
  FollowersCount  int       `gorm:"not null"`
  FriendsCount    int       `gorm:"not null"`
  ListedCount     int       `gorm:"not null"`
  MediaCount      int       `gorm:"not null"`
  RepliesCount    int       `gorm:"not null;index"`
  Timestamp       int64     `gorm:"not null;index:idx_twitter_users,priority:1"`
  Status          int       `gorm:"not null;index:idx_twitter_users,priority:2"`
  CreatedAt       time.Time `gorm:"not null;index"`
  UpdatedAt       time.Time `gorm:"not null"`
}

func (m *User) TableName() string {
  return "twitter_users"
}
