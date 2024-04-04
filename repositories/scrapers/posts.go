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

type PostsRepository struct {
  Db                 *gorm.DB
  SessionsRepository *repositories.SessionsRepository
  UsersRepository    *repositories.UsersRepository
  PostsRepository    *repositories.PostsRepository
}

func (r *PostsRepository) Process(session *models.Session, user *models.User, params map[string]interface{}) (cursor string, count int, err error) {
  var sessionData *repositories.SessionData
  buf, _ := session.Data.MarshalJSON()
  json.Unmarshal(buf, &sessionData)

  variables := map[string]interface{}{}
  variables["userId"] = fmt.Sprintf("%v", user.UserID)
  variables["count"] = 20
  if _, ok := params["cursors"]; ok {
    cursors := params["cursors"].(map[string]interface{})
    if _, ok := cursors[session.Account]; ok {
      variables["cursor"] = cursors[session.Account].(string)
    }
  }
  variables["includePromotedContent"] = true
  variables["withQuickPromoteEligibilityTweetFields"] = true
  variables["withVoice"] = true
  variables["withV2Timeline"] = true
  features := map[string]interface{}{
    "responsive_web_graphql_exclude_directive_enabled":                        true,
    "verified_phone_label_enabled":                                            false,
    "creator_subscriptions_tweet_preview_api_enabled":                         true,
    "responsive_web_graphql_timeline_navigation_enabled":                      true,
    "responsive_web_graphql_skip_user_profile_image_extensions_enabled":       false,
    "communities_web_enable_tweet_community_results_fetch":                    true,
    "c9s_tweet_anatomy_moderator_badge_enabled":                               true,
    "tweetypie_unmention_optimization_enabled":                                true,
    "responsive_web_edit_tweet_api_enabled":                                   true,
    "graphql_is_translatable_rweb_tweet_is_translatable_enabled":              true,
    "view_counts_everywhere_api_enabled":                                      true,
    "longform_notetweets_consumption_enabled":                                 true,
    "responsive_web_twitter_article_tweet_consumption_enabled":                true,
    "tweet_awards_web_tipping_enabled":                                        false,
    "freedom_of_speech_not_reach_fetch_enabled":                               true,
    "standardized_nudges_misinfo":                                             true,
    "tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled": true,
    "rweb_video_timestamps_enabled":                                           true,
    "longform_notetweets_rich_text_read_enabled":                              true,
    "longform_notetweets_inline_media_enabled":                                true,
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

  url := fmt.Sprintf("https://twitter.com/i/api/graphql/%v/UserTweets", sessionData.SectionPosts)
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
  container := gjson.GetBytes(body, "data.user.result.timeline_v2.timeline")
  container.Get("instructions").ForEach(func(_, s gjson.Result) bool {
    if s.Get("type").Str == "TimelinePinEntry" {
      if s.Get("entry.content.itemContent.itemType").Str != "TimelineTweet" {
        return true
      }
      twitterID, _ := strconv.ParseInt(strings.Trim(s.Get("entry.content.itemContent.tweet_results.result.rest_id").Raw, "\""), 10, 64)
      if twitterID == 0 {
        log.Println("twitter_id zero TimelinePinEntry")
        return true
      }
      statusID, _ := strconv.ParseInt(strings.Trim(s.Get("entry.content.itemContent.tweet_results.result.legacy.quoted_status_id_str").Raw, "\""), 10, 64)
      content := s.Get("entry.content.itemContent.tweet_results.result.legacy.full_text").Str
      createdAt, _ := time.Parse(time.RubyDate, s.Get("entry.content.itemContent.tweet_results.result.legacy.created_at").Str)
      media := &MediaInfo{}
      err := r.ExtractUserInfo(user, s.Get("entry.content.itemContent.tweet_results.result.core.user_results.result"))
      if err != nil {
        log.Println("user info extract error", err)
        return true
      }
      s.Get("entry.content.itemContent.tweet_results.result.legacy.entities.media").ForEach(func(_, s gjson.Result) bool {
        if s.Get("type").Str == "photo" {
          media.Photos = append(media.Photos, &PhotoInfo{
            Url: s.Get("media_url_https").Str,
          })
        }
        if s.Get("type").Str == "video" {
          videoInfo := &VideoInfo{}
          videoInfo.Cover = s.Get("media_url_https").Str
          videoInfo.DurationMillis, _ = strconv.Atoi(s.Get("video_info.duration_millis").Raw)
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
      post, err := r.PostsRepository.Get(twitterID)
      if errors.Is(err, gorm.ErrRecordNotFound) {
        r.PostsRepository.Create(
          user.ID,
          twitterID,
          statusID,
          content,
          common.JSONMap(&media),
          createdAt.UnixMilli(),
          status,
        )
      } else {
        if post.Status != 1 && post.Status != 2 && post.Status != 3 {
          r.PostsRepository.Updates(post, map[string]interface{}{
            "user_id":   user.ID,
            "status_id": statusID,
            "content":   content,
            "media":     common.JSONMap(&media),
            "status":    status,
          })
        }
      }
      return true
    }
    if s.Get("type").Str == "TimelineAddEntries" {
      s.Get("entries").ForEach(func(_, s gjson.Result) bool {
        if s.Get("content.entryType").Str == "TimelineTimelineItem" {
          if s.Get("content.itemContent.tweet_results.result.__typename").Str == "Tweet" {
            twitterID, _ := strconv.ParseInt(strings.Trim(s.Get("content.itemContent.tweet_results.result.legacy.id_str").Raw, "\""), 10, 64)
            if twitterID == 0 {
              log.Println("twitter_id zero Tweet", s.Get("content.itemContent").Raw)
              return true
            }
            statusID, _ := strconv.ParseInt(strings.Trim(s.Get("content.itemContent.tweet_results.result.legacy.quoted_status_id_str").Raw, "\""), 10, 64)
            content := s.Get("content.itemContent.tweet_results.result.legacy.full_text").Str
            createdAt, _ := time.Parse(time.RubyDate, s.Get("content.itemContent.tweet_results.result.legacy.created_at").Str)
            log.Println("content", twitterID, statusID, content, createdAt)
            count++
            media := &MediaInfo{}
            err := r.ExtractUserInfo(user, s.Get("content.itemContent.tweet_results.result.core.user_results.result"))
            if err != nil {
              log.Println("user info extract error", err)
              return true
            }
            s.Get("content.itemContent.tweet_results.result.legacy.entities.media").ForEach(func(_, s gjson.Result) bool {
              if s.Get("type").Str == "photo" {
                media.Photos = append(media.Photos, &PhotoInfo{
                  Url: s.Get("media_url_https").Str,
                })
              }
              if s.Get("type").Str == "video" {
                videoInfo := &VideoInfo{}
                videoInfo.DurationMillis, _ = strconv.Atoi(strings.Trim(s.Get("video_info.duration_millis").Str, "\""))
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
            post, err := r.PostsRepository.Get(twitterID)
            if errors.Is(err, gorm.ErrRecordNotFound) {
              r.PostsRepository.Create(
                user.ID,
                twitterID,
                statusID,
                content,
                common.JSONMap(&media),
                createdAt.UnixMilli(),
                status,
              )
            } else {
              if post.Status != 1 && post.Status != 2 && post.Status != 3 {
                r.PostsRepository.Updates(post, map[string]interface{}{
                  "user_id":   user.ID,
                  "status_id": statusID,
                  "content":   content,
                  "media":     common.JSONMap(&media),
                  "status":    status,
                })
              }
            }
            return true
          }
          if s.Get("content.itemContent.tweet_results.result.__typename").Str == "TweetWithVisibilityResults" {
            twitterID, _ := strconv.ParseInt(strings.Trim(s.Get("content.itemContent.tweet_results.result.tweet.rest_id").Raw, "\""), 10, 64)
            if twitterID == 0 {
              log.Println("twitter_id zero TweetWithVisibilityResults", s.Get("content.itemContent").Raw)
              return true
            }
            statusID, _ := strconv.ParseInt(strings.Trim(s.Get("content.itemContent.tweet_results.result.tweet.legacy.quoted_status_id_str").Raw, "\""), 10, 64)
            content := s.Get("content.itemContent.tweet_results.result.tweet.legacy.full_text").Str
            createdAt, _ := time.Parse(time.RubyDate, s.Get("content.itemContent.tweet_results.result.tweet.legacy.created_at").Str)
            count++
            media := &MediaInfo{}
            err := r.ExtractUserInfo(user, s.Get("content.itemContent.tweet_results.result.core.user_results.result"))
            if err != nil {
              //log.Println("user info extract error", err)
              return true
            }
            s.Get("content.itemContent.tweet_results.result.tweet.legacy.entities.media").ForEach(func(_, s gjson.Result) bool {
              if s.Get("type").Str == "photo" {
                media.Photos = append(media.Photos, &PhotoInfo{
                  Url: s.Get("media_url_https").Str,
                })
              }
              if s.Get("type").Str == "video" {
                videoInfo := &VideoInfo{}
                videoInfo.DurationMillis, _ = strconv.Atoi(strings.Trim(s.Get("video_info.duration_millis").Str, "\""))
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
            post, err := r.PostsRepository.Get(twitterID)
            if errors.Is(err, gorm.ErrRecordNotFound) {
              r.PostsRepository.Create(
                user.ID,
                twitterID,
                statusID,
                content,
                common.JSONMap(&media),
                createdAt.UnixMilli(),
                status,
              )
            } else {
              if post.Status != 1 && post.Status != 2 && post.Status != 3 {
                r.PostsRepository.Updates(post, map[string]interface{}{
                  "user_id":   user.ID,
                  "status_id": statusID,
                  "content":   content,
                  "media":     common.JSONMap(&media),
                  "status":    status,
                })
              }
            }
            return true
          }
        }
        if s.Get("content.entryType").Str == "TimelineTimelineCursor" {
          if s.Get("content.cursorType").Str == "Bottom" {
            cursor = s.Get("content.value").Str
          }
        }
        return true
      })
    }
    return true
  })

  log.Println("scrapers posts result", count, variables["cursor"], cursor)

  if count == 0 {
    cursor = ""
  }

  return
}

func (r *PostsRepository) ExtractUserInfo(user *models.User, s gjson.Result) (err error) {
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

  if userID != user.UserID {
    //log.Println("extract info", userID, twitterID, user.UserID, user.TwitterID, name, description)
    err = errors.New("user user_id not match")
    return
  }

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

  return
}
