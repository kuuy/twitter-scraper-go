package scrapers

type UserInfo struct {
  Account string `json:"account"`
  UserID  string `json:"user_id"`
}

type MediaInfo struct {
  Photos []*PhotoInfo `json:"photos"`
  Videos []*VideoInfo `json:"videos"`
}

type PhotoInfo struct {
  Url string `json:"url"`
}

type VideoInfo struct {
  Cover          string          `json:"cover"`
  AspectRatio    []int           `json:"aspect_ratio"`
  DurationMillis int             `json:"duration_millis"`
  Variants       []*VideoVariant `json:"variants"`
}

type VideoVariant struct {
  Bitrate     int    `json:"bitrate"`
  ContentType string `json:"content_type"`
  Url         string `json:"url"`
}
