package scrapers

import (
  "encoding/json"
  "errors"
  "fmt"
  "io"
  "log"
  "net"
  "net/http"
  "strconv"
  "strings"
  "time"

  "github.com/tidwall/gjson"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/models"
  "scraper.local/twitter-scraper/repositories"
)

type UsersRepository struct {
  Db                 *gorm.DB
  SessionsRepository *repositories.SessionsRepository
  UsersRepository    *repositories.UsersRepository
}

func (r *UsersRepository) Process(session *models.Session, account string) (user *models.User, err error) {
  var sessionData *repositories.SessionData
  buf, _ := session.Data.MarshalJSON()
  json.Unmarshal(buf, &sessionData)

  variables := map[string]interface{}{}
  variables["screen_name"] = fmt.Sprintf("%v", account)
  variables["withSafetyModeUserFields"] = true
  features := map[string]interface{}{
    "hidden_profile_likes_enabled":                                      true,
    "hidden_profile_subscriptions_enabled":                              true,
    "responsive_web_graphql_exclude_directive_enabled":                  true,
    "verified_phone_label_enabled":                                      false,
    "subscriptions_verification_info_is_identity_verified_enabled":      true,
    "subscriptions_verification_info_verified_since_enabled":            true,
    "highlights_tweets_tab_ui_enabled":                                  true,
    "responsive_web_twitter_article_notes_tab_enabled":                  true,
    "creator_subscriptions_tweet_preview_api_enabled":                   true,
    "responsive_web_graphql_skip_user_profile_image_extensions_enabled": false,
    "responsive_web_graphql_timeline_navigation_enabled":                true,
  }
  fieldToggles := map[string]interface{}{
    "withAuxiliaryUserLabels": false,
  }
  tr := &http.Transport{
    DisableKeepAlives: true,
  }
  if session.Slot > 0 {
    tr.DialContext = (&common.ProxySession{
      Proxy: fmt.Sprintf("socks5://127.0.0.1:%d?timeout=30s", 2080+session.Slot),
    }).DialContext
  } else {
    tr.DialContext = (&net.Dialer{}).DialContext
  }

  httpClient := &http.Client{
    Transport: tr,
    Timeout:   time.Duration(15) * time.Second,
  }

  timestamp := time.Now().UnixMicro()
  if session.UnblockedAt > timestamp {
    err = errors.New("waiting for scrapper unblock")
    return
  }

  headers := map[string]string{
    "User-Agent":    session.Agent,
    "cookie":        session.Cookie,
    "Authorization": fmt.Sprintf("Bearer %v", sessionData.AccessToken),
  }

  for _, p := range strings.Split(headers["cookie"], ";") {
    parts := strings.SplitN(p, "=", 2)
    if strings.Trim(parts[0], " ") == "ct0" {
      headers["X-Csrf-Token"] = strings.Trim(parts[1], " ")
      break
    }
  }

  url := fmt.Sprintf("https://twitter.com/i/api/graphql/%v/UserByScreenName", sessionData.SecionUsers)
  req, _ := http.NewRequest("GET", url, nil)
  for key, val := range headers {
    req.Header.Set(key, val)
  }
  q := req.URL.Query()
  b1, _ := json.Marshal(variables)
  b2, _ := json.Marshal(features)
  b3, _ := json.Marshal(fieldToggles)
  q.Add("variables", string(b1))
  q.Add("features", string(b2))
  q.Add("fieldToggles", string(b3))
  req.URL.RawQuery = q.Encode()
  resp, err := httpClient.Do(req)
  if err != nil {
    if session.Slot > 0 {
      log.Println("request can not be send", 2080+session.Slot)
    }
    return
  }
  defer resp.Body.Close()

  if resp.StatusCode != http.StatusOK {
    if resp.StatusCode == 401 {
      r.SessionsRepository.Update(session, "status", 0)
    }
    if resp.StatusCode == 429 {
      r.SessionsRepository.Update(session, "unblocked_at", timestamp+900000000)
    }
    err = errors.New(
      fmt.Sprintf(
        "request error: account[%s] status[%s] code[%d] cookie[%v]",
        session.Account,
        resp.Status,
        resp.StatusCode,
        common.GetEnvString("cookie"),
      ),
    )
    return
  }

  body, _ := io.ReadAll(resp.Body)
  container := gjson.GetBytes(body, "data.user.result")

  if len(container.Raw) == 0 {
    err = errors.New("user info can not be found")
    return
  }

  user, err = r.ExtractUserInfo(container)

  return
}

func (r *UsersRepository) ExtractUserInfo(s gjson.Result) (user *models.User, err error) {
  account := s.Get("legacy.screen_name").Str
  userID, _ := strconv.ParseInt(strings.Trim(s.Get("rest_id").Raw, "\""), 10, 64)
  name := s.Get("legacy.name").Str
  description := s.Get("legacy.description").Str
  avatar := strings.Replace(s.Get("legacy.profile_image_url_https").Str, "_normal.", ".", 1)
  favouritesCount, _ := strconv.Atoi(strings.Trim(s.Get("legacy.favourites_count").Raw, "\""))
  followersCount, _ := strconv.Atoi(strings.Trim(s.Get("legacy.followers_count").Raw, "\""))
  friendsCount, _ := strconv.Atoi(strings.Trim(s.Get("legacy.friends_count").Raw, "\""))
  listedCount, _ := strconv.Atoi(strings.Trim(s.Get("legacy.listed_count").Raw, "\""))
  mediaCount, _ := strconv.Atoi(strings.Trim(s.Get("legacy.media_count").Raw, "\""))
  createdAt, _ := time.Parse(time.RubyDate, s.Get("legacy.created_at").Str)

  if userID == 0 {
    err = errors.New("user info extract error")
    return
  }

  user, err = r.UsersRepository.GetByUserID(userID)
  if errors.Is(err, gorm.ErrRecordNotFound) {
    user.ID, _ = r.UsersRepository.Create(
      account,
      userID,
      name,
      description,
      avatar,
      favouritesCount,
      followersCount,
      friendsCount,
      listedCount,
      mediaCount,
      createdAt.UnixMilli(),
    )
  } else {
    r.UsersRepository.Updates(user, map[string]interface{}{
      "account":          account,
      "user_id":          userID,
      "name":             name,
      "description":      description,
      "avatar":           avatar,
      "favourites_count": favouritesCount,
      "followers_count":  followersCount,
      "friends_count":    friendsCount,
      "listed_count":     listedCount,
      "media_count":      mediaCount,
      "timestamp":        createdAt.UnixMilli(),
    })
  }

  return user, nil
}
