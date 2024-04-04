package scrapers

import (
  "crypto/md5"
  "crypto/sha1"
  "encoding/hex"
  "encoding/json"
  "fmt"
  "hash/crc32"
  "net/http"
  "regexp"
  "sort"
  "strconv"
  "time"

  "github.com/go-chi/chi/v5"

  "scraper.local/twitter-scraper/api"
  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/config"
  "scraper.local/twitter-scraper/repositories"
  mediaRepositories "scraper.local/twitter-scraper/repositories/media"
)

type PostsHandler struct {
  ApiContext            *common.ApiContext
  Response              *api.ResponseHandler
  Repository            *repositories.PostsRepository
  RepliesRepository     *repositories.RepliesRepository
  UsersRepository       *repositories.UsersRepository
  MediaPhotosRepository *mediaRepositories.PhotosRepository
  MediaVideosRepository *mediaRepositories.VideosRepository
}

func NewPostsRouter(apiContext *common.ApiContext) http.Handler {
  h := PostsHandler{
    ApiContext: apiContext,
  }
  h.Repository = &repositories.PostsRepository{
    Db: h.ApiContext.Db,
  }
  h.RepliesRepository = &repositories.RepliesRepository{
    Db: h.ApiContext.Db,
  }
  h.UsersRepository = &repositories.UsersRepository{
    Db: h.ApiContext.Db,
  }
  h.MediaPhotosRepository = &mediaRepositories.PhotosRepository{
    Db:  h.ApiContext.Db,
    Rdb: h.ApiContext.Rdb,
    Ctx: h.ApiContext.Ctx,
  }
  h.MediaVideosRepository = &mediaRepositories.VideosRepository{
    Db:  h.ApiContext.Db,
    Rdb: h.ApiContext.Rdb,
    Ctx: h.ApiContext.Ctx,
  }

  r := chi.NewRouter()
  r.Get("/", h.Listings)
  return r
}

func (h *PostsHandler) Listings(
  w http.ResponseWriter,
  r *http.Request,
) {
  h.ApiContext.Mux.Lock()
  defer h.ApiContext.Mux.Unlock()

  h.Response = &api.ResponseHandler{
    Writer: w,
  }

  hideLinks, _ := strconv.Atoi(r.URL.Query().Get("hide_links"))
  status, _ := strconv.Atoi(r.URL.Query().Get("status"))

  var current int
  if !r.URL.Query().Has("current") {
    current = 1
  }
  current, _ = strconv.Atoi(r.URL.Query().Get("current"))
  if current < 1 {
    h.Response.Error(http.StatusForbidden, 1004, "current not valid")
    return
  }

  var pageSize int
  if !r.URL.Query().Has("page_size") {
    pageSize = 50
  } else {
    pageSize, _ = strconv.Atoi(r.URL.Query().Get("page_size"))
  }
  if pageSize < 1 || pageSize > 100 {
    h.Response.Error(http.StatusForbidden, 1004, "page size not valid")
    return
  }

  conditions := make(map[string]interface{})

  if r.URL.Query().Get("is_synced") != "" {
    conditions["status"] = 3
  }

  if r.URL.Query().Get("account") != "" {
    conditions["account"] = r.URL.Query().Get("account")
  }

  if r.URL.Query().Get("status") != "" {
    conditions["status"] = status
  }

  m := regexp.MustCompile("https?://[A-Za-z0-9\\.\\-]+(/[A-Za-z0-9\\?\\&\\=;\\+!'\\(\\)\\*\\-\\._~%]*)*")

  hash := md5.Sum([]byte(fmt.Sprintf("%v", conditions)))
  redisKey := fmt.Sprintf(
    config.REDIS_KEY_POSTS_COUNT,
    hex.EncodeToString(hash[:]),
  )
  var total int64
  val, _ := h.ApiContext.Rdb.Get(h.ApiContext.Ctx, redisKey).Result()
  if val == "" {
    total = h.Repository.Count(conditions)
    if total > 1000000 {
      h.ApiContext.Rdb.SetEX(h.ApiContext.Ctx, redisKey, total, time.Hour*72)
    } else {
      h.ApiContext.Rdb.SetEX(h.ApiContext.Ctx, redisKey, total, time.Minute*15)
    }
  } else {
    total, _ = strconv.ParseInt(val, 10, 64)
  }

  posts := h.Repository.Listings(conditions, current, pageSize)
  data := make([]*PostInfo, len(posts))
  for i, post := range posts {
    var mediaInfo *MediaInfo
    buf, _ := post.Media.MarshalJSON()
    json.Unmarshal(buf, &mediaInfo)

    if mediaInfo.Photos != nil {
      for _, item := range mediaInfo.Photos {
        url := item.Url
        hash := sha1.Sum([]byte(url))
        urlSha1 := hex.EncodeToString(hash[:])
        if photo, err := h.MediaPhotosRepository.Get(url, urlSha1); err == nil && photo.Status == 1 {
          crc32q := crc32.MakeTable(0xD5828281)
          i := crc32.Checksum([]byte(photo.Filehash), crc32q)
          item.Url = fmt.Sprintf(
            "%s/photos/%d/%d/%s.%s",
            common.GetEnvString(fmt.Sprintf("SCRAPER_STORAGE_URL_%v", photo.Node)),
            i/233%50,
            i/89%50,
            photo.Filehash,
            photo.Extension,
          )
          if photo.IsSynced {
            item.Url = photo.CloudUrl
          }
        }
      }
    }

    if mediaInfo.Videos != nil {
      for _, item := range mediaInfo.Videos {
        sort.Slice(item.Variants, func(i, j int) bool {
          return item.Variants[i].Bitrate > item.Variants[j].Bitrate
        })
        if item.Variants[0].Bitrate == 0 {
          continue
        }
        url := item.Cover
        hash := sha1.Sum([]byte(url))
        urlSha1 := hex.EncodeToString(hash[:])
        if photo, err := h.MediaPhotosRepository.Get(url, urlSha1); err == nil && photo.Status == 1 {
          crc32q := crc32.MakeTable(0xD5828281)
          i := crc32.Checksum([]byte(photo.Filehash), crc32q)
          item.Cover = fmt.Sprintf(
            "%s/photos/%d/%d/%s.%s",
            common.GetEnvString(fmt.Sprintf("SCRAPER_STORAGE_URL_%v", photo.Node)),
            i/233%50,
            i/89%50,
            photo.Filehash,
            photo.Extension,
          )
          if photo.IsSynced {
            item.Cover = photo.CloudUrl
          }
        }
        url = item.Variants[0].Url
        hash = sha1.Sum([]byte(url))
        urlSha1 = hex.EncodeToString(hash[:])
        if video, err := h.MediaVideosRepository.Get(url, urlSha1); err == nil && video.Status == 1 {
          crc32q := crc32.MakeTable(0xD5828281)
          i := crc32.Checksum([]byte(video.Filehash), crc32q)
          item.Variants[0].Url = fmt.Sprintf(
            "%s/videos/%d/%d/%s.%s",
            common.GetEnvString(fmt.Sprintf("SCRAPER_STORAGE_URL_%v", video.Node)),
            i/233%50,
            i/89%50,
            video.Filehash,
            video.Extension,
          )
          if video.IsSynced {
            item.Variants[0].Url = video.CloudUrl
          }
        }
        item.Variants = []*VideoVariant{item.Variants[0]}
      }
    }

    data[i] = &PostInfo{
      ID:        post.ID,
      TwitterID: fmt.Sprint(post.TwitterID),
      StatusID:  fmt.Sprint(post.StatusID),
      Content:   post.Content,
      Media:     mediaInfo,
      Timestamp: post.Timestamp,
    }
    if user, err := h.UsersRepository.Find(post.UserID); err == nil {
      url := user.Avatar
      hash := sha1.Sum([]byte(url))
      urlSha1 := hex.EncodeToString(hash[:])
      if photo, err := h.MediaPhotosRepository.Get(url, urlSha1); err == nil && photo.Status == 1 {
        crc32q := crc32.MakeTable(0xD5828281)
        i := crc32.Checksum([]byte(photo.Filehash), crc32q)
        url = fmt.Sprintf(
          "%s/photos/%d/%d/%s.%s",
          common.GetEnvString(fmt.Sprintf("SCRAPER_STORAGE_URL_%v", photo.Node)),
          i/233%50,
          i/89%50,
          photo.Filehash,
          photo.Extension,
        )
        if photo.IsSynced {
          url = photo.CloudUrl
        }
      }
      data[i].UserInfo = &UserInfo{
        ID:              user.ID,
        Account:         user.Account,
        UserID:          fmt.Sprint(user.UserID),
        Name:            user.Name,
        Description:     user.Description,
        Avatar:          url,
        FavouritesCount: user.FavouritesCount,
        FollowersCount:  user.FollowersCount,
        FriendsCount:    user.FriendsCount,
        ListedCount:     user.ListedCount,
        MediaCount:      user.MediaCount,
        RepliesCount:    user.RepliesCount,
        Timestamp:       user.Timestamp,
      }
    }

    if hideLinks == 1 {
      data[i].Content = m.ReplaceAllString(data[i].Content, "")
      data[i].UserInfo.Description = m.ReplaceAllString(data[i].UserInfo.Description, "")
    }
  }

  h.Response.Pagenate(data, total, current, pageSize)
}
