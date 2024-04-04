package scrapers

type PostInfo struct {
  ID        string     `json:"id"`
  UserInfo  *UserInfo  `json:"user"`
  TwitterID string     `json:"twitter_id"`
  StatusID  string     `json:"status_id"`
  Content   string     `json:"content"`
  Media     *MediaInfo `json:"media"`
  Timestamp int64      `json:"timestamp"`
}

type ReplyInfo struct {
  ID        string         `json:"id"`
  UserInfo  *UserInfo      `json:"user"`
  Post      *PostShortInfo `json:"post"`
  TwitterID string         `json:"twitter_id"`
  Content   string         `json:"content"`
  Media     *MediaInfo     `json:"media"`
  Timestamp int64          `json:"timestamp"`
}

type PostShortInfo struct {
  ID        string `json:"id"`
  TwitterID string `json:"twitter_id"`
  StatusID  string `json:"status_id"`
}

type UserInfo struct {
  ID              string `json:"id"`
  Account         string `json:"account"`
  UserID          string `json:"user_id"`
  Name            string `json:"name"`
  Description     string `json:"description"`
  Avatar          string `json:"avatar"`
  FavouritesCount int    `json:"favourites_count"`
  FollowersCount  int    `json:"followers_count"`
  FriendsCount    int    `json:"friends_count"`
  ListedCount     int    `json:"listed_count"`
  MediaCount      int    `json:"media_count"`
  RepliesCount    int    `json:"replies_count"`
  Timestamp       int64  `json:"timestamp"`
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
