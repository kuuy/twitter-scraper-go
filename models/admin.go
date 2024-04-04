package models

import (
  "time"
)

type Admin struct {
  ID        string    `gorm:"size:20;primaryKey"`
  Account   string    `gorm:"size:64;not null;uniqueIndex"`
  Password  string    `gorm:"size:128;not null"`
  Salt      string    `gorm:"size:16;not null"`
  Status    int64     `gorm:"not null"`
  CreatedAt time.Time `gorm:"not null"`
  UpdatedAt time.Time `gorm:"not null"`
}

func (m *Admin) TableName() string {
  return "admins"
}
