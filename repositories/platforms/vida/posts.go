package vida

import (
  "crypto/sha1"
  "encoding/hex"
  "encoding/json"
  "errors"
  "sort"
  "strings"
  "time"

  "github.com/rs/xid"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/models"
  vidaModels "scraper.local/twitter-scraper/models/platforms/vida"
  syncModels "scraper.local/twitter-scraper/models/platforms/vida/posts"
  mediaRepositories "scraper.local/twitter-scraper/repositories/media"
  scrapersRepositories "scraper.local/twitter-scraper/repositories/scrapers/media"
)

type PostsRepository struct {
  Db                       *gorm.DB
  VidaDb                   *gorm.DB
  ScrapersPhotosRepository *scrapersRepositories.PhotosRepository
  MediaPhotosRepository    *mediaRepositories.PhotosRepository
  MediaVideosRepository    *mediaRepositories.VideosRepository
}

func (r *PostsRepository) Sync(aff int, post *models.Post) error {
  var mediaInfo *MediaInfo
  buf, _ := post.Media.MarshalJSON()
  json.Unmarshal(buf, &mediaInfo)

  if mediaInfo.Photos != nil {
    var sync = &syncModels.Sync{}
    entity := &vidaModels.Post{
      Aff:       aff,
      Type:      5,
      Shape:     0,
      Content:   post.Content,
      ImgPath:   "",
      ImgWidth:  0,
      ImgHeight: 0,
      ImgMore:   "",
      VideoPath: "",
      Status:    1,
      DisplayAt: time.Now(),
    }
    var imgMore []string
    for i, item := range mediaInfo.Photos {
      url := item.Url
      hash := sha1.Sum([]byte(url))
      urlSha1 := hex.EncodeToString(hash[:])
      photo, _ := r.MediaPhotosRepository.Get(url, urlSha1)
      if i == 0 {
        r.Db.Where("post_id=? AND url_sha1=?", post.ID, urlSha1).Take(&sync)
        if sync.ID != "" {
          break
        }
        sync.UrlSha1 = urlSha1

        if photo.Width == 0 {
          config, err := r.ScrapersPhotosRepository.Config(url)
          if err != nil {
            continue
          }
          photo.Width = config.Width
          photo.Height = config.Height
        }

        if photo.CloudUrl != "" {
          entity.ImgPath = photo.CloudUrl
        } else {
          entity.ImgPath = photo.Url
        }
        entity.ImgWidth = photo.Width
        entity.ImgHeight = photo.Height
      } else {
        imgMore = append(imgMore, item.Url)
      }
    }
    entity.ImgMore = strings.Join(imgMore[:], ",")

    if sync.ID == "" && sync.UrlSha1 != "" && entity.ImgPath != "" {
      result := r.Db.Create(&syncModels.Sync{
        ID:      xid.New().String(),
        PostID:  post.ID,
        UrlSha1: sync.UrlSha1,
        SyncID:  entity.ID,
      })
      if result.Error != nil {
        return result.Error
      }
      if result.RowsAffected == 0 {
        return errors.New("vida post sync create failed")
      }
      result = r.VidaDb.Create(&entity)
      if result.Error != nil {
        return result.Error
      }
      if result.RowsAffected == 0 {
        return errors.New("vida post create failed")
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

      entity := &vidaModels.Post{
        Aff:       aff,
        Type:      1,
        Shape:     1,
        Content:   post.Content,
        ImgPath:   "",
        ImgWidth:  0,
        ImgHeight: 0,
        ImgMore:   "",
        VideoPath: "",
        Status:    1,
        DisplayAt: time.Now(),
      }

      url := item.Cover
      hash := sha1.Sum([]byte(url))
      urlSha1 := hex.EncodeToString(hash[:])
      photo, _ := r.MediaPhotosRepository.Get(url, urlSha1)
      if photo.Width == 0 {
        config, err := r.ScrapersPhotosRepository.Config(url)
        if err != nil {
          continue
        }
        photo.Width = config.Width
        photo.Height = config.Height
      }

      url = item.Variants[0].Url
      hash = sha1.Sum([]byte(url))
      urlSha1 = hex.EncodeToString(hash[:])
      video, err := r.MediaVideosRepository.Get(url, urlSha1)
      if err != nil {
        continue
      }

      if photo.CloudUrl != "" {
        entity.ImgPath = photo.CloudUrl
      } else {
        entity.ImgPath = photo.Url
      }
      entity.ImgWidth = photo.Width
      entity.ImgHeight = photo.Height
      entity.VideoPath = video.Url

      if entity.ImgPath == "" && entity.VideoPath == "" {
        continue
      }

      var sync = &syncModels.Sync{}
      result := r.Db.Where("post_id=? AND url_sha1=?", post.ID, video.UrlSha1).Take(&sync)
      if sync.ID != "" {
        continue
      }

      result = r.Db.Create(&syncModels.Sync{
        ID:      xid.New().String(),
        PostID:  post.ID,
        UrlSha1: urlSha1,
        SyncID:  entity.ID,
      })
      if result.Error != nil {
        return result.Error
      }
      if result.RowsAffected == 0 {
        return errors.New("vida post sync create failed")
      }

      result = r.VidaDb.Create(&entity)
      if result.Error != nil {
        return result.Error
      }
      if result.RowsAffected == 0 {
        return errors.New("vida post create failed")
      }
    }
  }

  return nil
}
