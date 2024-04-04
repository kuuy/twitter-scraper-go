package models

import (
  "gorm.io/datatypes"
  "time"
)

type Task struct {
  ID        string            `gorm:"size:20;primaryKey"`
  Name      string            `gorm:"size:50;not null;uniqueIndex"`
  Action    int               `gorm:"not null;index:idx_twitter_tasks,priority:1"`
  Params    datatypes.JSONMap `gorm:"not null"`
  Timestamp int64             `gorm:"not null;index:idx_twitter_tasks,priority:3"`
  Status    int               `gorm:"not null;index:idx_twitter_tasks,priority:2"`
  CreatedAt time.Time         `gorm:"not null"`
  UpdatedAt time.Time         `gorm:"not null"`
}

func (m *Task) TableName() string {
  return "twitter_tasks"
}
