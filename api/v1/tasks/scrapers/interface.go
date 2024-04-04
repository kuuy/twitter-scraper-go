package scrapers

import (
  "time"
)

type PostInfo struct {
  ID        string    `json:"id"`
  Account   string    `json:"account"`
  Timestamp int64     `json:"timestamp"`
  Status    int       `json:"status"`
  CreatedAt time.Time `json:"created_at"`
  UpdatedAt time.Time `json:"updated_at"`
}
