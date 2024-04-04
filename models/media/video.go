package media

import (
  "time"
)

type Video struct {
  ID        string    `gorm:"size:20;primaryKey"`
  Url       string    `gorm:"size:155;not null;"`
  UrlSha1   string    `gorm:"size:40;not null;index"`
  Mime      string    `gorm:"size:30;not null"`
  Size      int64     `gorm:"not null;index:idx_twitter_media_videos_sync,priority:1"`
  Node      int       `gorm:"not null;index:idx_twitter_media_videos_sync,priority:2"`
  CloudUrl  string    `gorm:"size:155;not null"`
  Filehash  string    `gorm:"size:64;not null;index"`
  Extension string    `gorm:"size:10;not null"`
  IsSynced  bool      `gorm:"not null;index:idx_twitter_media_videos_sync,priority:3"`
  Timestamp int64     `gorm:"not null;index:idx_twitter_media_videos,priority:1"`
  Status    int       `gorm:"not null;index:idx_twitter_media_videos,priority:2"`
  CreatedAt time.Time `gorm:"not null"`
  UpdatedAt time.Time `gorm:"not null"`
}

func (m *Video) TableName() string {
  return "twitter_media_videos"
}
