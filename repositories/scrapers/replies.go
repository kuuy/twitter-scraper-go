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

type RepliesRepository struct {
  Db                 *gorm.DB
  SessionsRepository *repositories.SessionsRepository
  UsersRepository    *repositories.UsersRepository
  RepliesRepository  *repositories.RepliesRepository
}

func (r *RepliesRepository) Process(session *models.Session, post *models.Post, params map[string]interface{}) (cursor string, count int, err error) {
  var sessionData *repositories.SessionData
  buf, _ := session.Data.MarshalJSON()
  json.Unmarshal(buf, &sessionData)

  variables := map[string]interface{}{}
  if post.StatusID > 0 {
    variables["focalTweetId"] = fmt.Sprint(post.StatusID)
  } else {
    variables["focalTweetId"] = fmt.Sprint(post.TwitterID)
  }
  if _, ok := params["cursors"]; ok {
    cursors := params["cursors"].(map[string]interface{})
    if _, ok := cursors[session.Account]; ok {
      variables["cursor"] = cursors[session.Account].(string)
    }
  }
  variables["with_rux_injections"] = false
  variables["includePromotedContent"] = true
  variables["withCommunity"] = true
  variables["withQuickPromoteEligibilityTweetFields"] = true
  variables["withBirdwatchNotes"] = true
  variables["withVoice"] = true
  variables["withV2Timeline"] = true
  features := map[string]interface{}{
    "responsive_web_graphql_exclude_directive_enabled":                        true,
    "verified_phone_label_enabled":                                            false,
    "creator_subscriptions_tweet_preview_api_enabled":                         true,
    "responsive_web_graphql_timeline_navigation_enabled":                      true,
    "responsive_web_graphql_skip_user_profile_image_extensions_enabled":       false,
    "c9s_tweet_anatomy_moderator_badge_enabled":                               true,
    "tweetypie_unmention_optimization_enabled":                                true,
    "responsive_web_edit_tweet_api_enabled":                                   true,
    "graphql_is_translatable_rweb_tweet_is_translatable_enabled":              true,
    "view_counts_everywhere_api_enabled":                                      true,
    "longform_notetweets_consumption_enabled":                                 true,
    "responsive_web_twitter_article_tweet_consumption_enabled":                false,
    "tweet_awards_web_tipping_enabled":                                        false,
    "freedom_of_speech_not_reach_fetch_enabled":                               true,
    "standardized_nudges_misinfo":                                             true,
    "tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled": true,
    "rweb_video_timestamps_enabled":                                           true,
    "longform_notetweets_rich_text_read_enabled":                              true,
    "longform_notetweets_inline_media_enabled":                                true,
    "responsive_web_media_download_video_enabled":                             false,
    "responsive_web_enhance_cards_enabled":                                    false,
  }
  fieldToggles := map[string]interface{}{
    "withArticleRichContentState": false,
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

  url := fmt.Sprintf("https://twitter.com/i/api/graphql/%v/TweetDetail", sessionData.SectionReplies)
  log.Println("url", url)
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

  container := gjson.GetBytes(body, "data.threaded_conversation_with_injections_v2")
  container.Get("instructions").ForEach(func(_, s gjson.Result) bool {
    if s.Get("type").Str == "TimelineAddEntries" {
      s.Get("entries").ForEach(func(_, s gjson.Result) bool {
        if s.Get("content.entryType").Str == "TimelineTimelineModule" {
          s.Get("content.items").ForEach(func(_, s gjson.Result) bool {
            if s.Get("item.itemContent.itemType").Str != "TimelineTweet" {
              return true
            }
            twitterID, _ := strconv.ParseInt(s.Get("item.itemContent.tweet_results.result.rest_id").Str, 10, 64)
            if twitterID == 0 {
              log.Println("twitter_id zero TimelineTimelineModule")
              return true
            }
            content := s.Get("item.itemContent.tweet_results.result.legacy.full_text").Str
            createdAt, _ := time.Parse(time.RubyDate, s.Get("item.itemContent.tweet_results.result.legacy.created_at").Str)
            log.Println("content", twitterID, content, createdAt, variables["focalTweetId"])
            count++
            media := &MediaInfo{}
            user, err := r.ExtractUserInfo(s.Get("item.itemContent.tweet_results.result.core.user_results.result"))
            if err != nil {
              log.Println("user info extract error", err)
              return true
            }
            if user.Status != 1 {
              log.Println("user status is invalid", user.ID, user.Status)
              return true
            }
            s.Get("item.itemContent.tweet_results.result.legacy.entities.media").ForEach(func(_, s gjson.Result) bool {
              if s.Get("type").Str == "photo" {
                media.Photos = append(media.Photos, &PhotoInfo{
                  Url: s.Get("media_url_https").Str,
                })
              }
              if s.Get("type").Str == "video" {
                videoInfo := &VideoInfo{}
                videoInfo.Cover = s.Get("media_url_https").Str
                videoInfo.DurationMillis, _ = strconv.Atoi(s.Get("video_info.duration_millis").Str)
                s.Get("video_info.aspect_ratio").ForEach(func(_, s gjson.Result) bool {
                  videoInfo.AspectRatio = append(videoInfo.AspectRatio, int(s.Value().(float64)))
                  return true
                })
                s.Get("video_info.variants").ForEach(func(_, s gjson.Result) bool {
                  variant := &VideoVariant{}
                  if s.Get("bitrate").Raw != "" {
                    variant.Bitrate = int(s.Get("bitrate").Value().(float64))
                  }
                  variant.ContentType = s.Get("content_type").Str
                  variant.Url = s.Get("url").Str
                  videoInfo.Variants = append(videoInfo.Variants, variant)
                  return true
                })
                media.Videos = append(media.Videos, videoInfo)
              }
              return true
            })
            status := 1
            if media.Photos == nil && media.Videos == nil {
              status = 3
            }
            reply, err := r.RepliesRepository.Get(twitterID)
            if errors.Is(err, gorm.ErrRecordNotFound) {
              r.RepliesRepository.Create(
                user.ID,
                post.ID,
                twitterID,
                content,
                common.JSONMap(media),
                createdAt.UnixMilli(),
                status,
              )
              r.UsersRepository.Update(user, "replies_count", gorm.Expr("replies_count+1"))
            } else {
              if reply.Status != 1 && reply.Status != 2 && reply.Status != 3 {
                r.RepliesRepository.Updates(reply, map[string]interface{}{
                  "user_id": user.ID,
                  "post_id": post.ID,
                  "content": content,
                  "media":   common.JSONMap(media),
                  "status":  status,
                })
              }
            }
            return true
          })
        }
        if s.Get("content.itemContent.itemType").Str == "TimelineTimelineCursor" {
          if s.Get("content.itemContent.cursorType").Str == "Bottom" {
            cursor = s.Get("content.itemContent.value").Str
          }
        }
        return true
      })
    }
    return true
  })

  log.Println("scrapers replies result", count, variables["cursor"], cursor)

  if count == 0 {
    cursor = ""
  }

  return
}

func (r *RepliesRepository) ExtractUserInfo(s gjson.Result) (*models.User, error) {
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

  user, err := r.UsersRepository.GetByUserID(userID)
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
