package media

import (
  "crypto/sha1"
  "encoding/hex"
  "errors"
  "fmt"
  "hash/crc32"
  "image"
  "io"
  "math/rand"
  "net"
  "net/http"
  "os"
  "time"

  _ "image/gif"
  _ "image/jpeg"
  _ "image/png"

  "github.com/h2non/filetype"
  "github.com/rs/xid"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/common"
  models "scraper.local/twitter-scraper/models/media"
)

type PhotosRepository struct {
  Db *gorm.DB
}

func (r *PhotosRepository) Download(url string, urlSha1 string) (err error) {
  tr := &http.Transport{
    DisableKeepAlives: true,
  }
  session := &net.Dialer{}
  tr.DialContext = session.DialContext

  httpClient := &http.Client{
    Transport: tr,
    Timeout:   time.Duration(30) * time.Second,
  }

  req, _ := http.NewRequest("GET", url, nil)
  resp, err := httpClient.Do(req)
  if err != nil {
    return
  }
  defer resp.Body.Close()

  if resp.StatusCode != http.StatusOK {
    err = errors.New(
      fmt.Sprintf(
        "request error: status[%s] code[%d] cookie[%v]",
        resp.Status,
        resp.StatusCode,
        common.GetEnvString("cookie"),
      ),
    )
    return
  }

  tmppath := fmt.Sprintf(
    "%s/.cache/%d/%d",
    common.GetEnvString("SCRAPER_STORAGE_PATH"),
    rand.Intn(50),
    rand.Intn(50),
  )
  err = os.MkdirAll(
    tmppath,
    os.ModePerm,
  )
  if err != nil {
    return
  }

  tmpfile := fmt.Sprintf(
    "%s/%s.download",
    tmppath,
    xid.New().String(),
  )
  dst, err := os.Create(tmpfile)
  if err != nil {
    return
  }
  defer os.Remove(tmpfile)
  defer dst.Close()

  hash := sha1.New()
  t := io.TeeReader(resp.Body, hash)
  _, err = io.Copy(dst, t)
  if err != nil {
    return
  }

  head := make([]byte, 261)
  if _, err = dst.ReadAt(head, 0); err != nil {
    return
  }

  kind, _ := filetype.Image(head)
  if kind == filetype.Unknown {
    err = errors.New("unknow filetype")
    return
  }

  info, err := dst.Stat()
  if err != nil {
    return
  }

  if _, err = dst.Seek(0, 0); err != nil {
    return
  }

  config, _, err := image.DecodeConfig(dst)
  if err != nil {
    return
  }

  filehash := hex.EncodeToString(hash.Sum(nil))
  crc32q := crc32.MakeTable(0xD5828281)
  i := crc32.Checksum([]byte(filehash), crc32q)
  localpath := fmt.Sprintf(
    "%s/photos/%d/%d",
    common.GetEnvString("SCRAPER_STORAGE_PATH"),
    i/233%50,
    i/89%50,
  )
  err = os.MkdirAll(localpath, os.ModePerm)
  if err != nil {
    return
  }
  localfile := fmt.Sprintf(
    "%s/%s.%s",
    localpath,
    filehash,
    kind.Extension,
  )

  var photo *models.Photo
  if err := r.Db.Where("url_sha1=? AND url=?", urlSha1, url).Take(&photo).Error; errors.Is(err, gorm.ErrRecordNotFound) {
    photo = &models.Photo{
      ID:        xid.New().String(),
      Url:       url,
      UrlSha1:   urlSha1,
      Mime:      kind.MIME.Value,
      Width:     config.Width,
      Height:    config.Height,
      Size:      info.Size(),
      Node:      common.GetEnvInt("SCRAPER_STORAGE_NODE"),
      Filehash:  filehash,
      Extension: kind.Extension,
      Timestamp: time.Now().UnixMilli(),
      Status:    1,
    }
    var syncedPhoto *models.Photo
    if err := r.Db.Where("filehash=? AND is_synced=?", filehash, true).Take(&syncedPhoto).Error; err == nil {
      photo.CloudUrl = syncedPhoto.CloudUrl
      photo.IsSynced = true
    }
    r.Db.Create(&photo)
    os.Rename(tmpfile, localfile)
  }

  return
}

func (r *PhotosRepository) Config(url string) (config image.Config, err error) {
  tr := &http.Transport{
    DisableKeepAlives: true,
  }
  session := &net.Dialer{}
  tr.DialContext = session.DialContext

  httpClient := &http.Client{
    Transport: tr,
    Timeout:   time.Duration(30) * time.Second,
  }

  req, _ := http.NewRequest("GET", url, nil)
  resp, err := httpClient.Do(req)
  if err != nil {
    return
  }
  defer resp.Body.Close()

  if resp.StatusCode != http.StatusOK {
    err = errors.New(
      fmt.Sprintf(
        "request error: status[%s] code[%d] cookie[%v]",
        resp.Status,
        resp.StatusCode,
        common.GetEnvString("cookie"),
      ),
    )
    return
  }

  tmppath := fmt.Sprintf(
    "%s/.cache/%d/%d",
    common.GetEnvString("SCRAPER_STORAGE_PATH"),
    rand.Intn(50),
    rand.Intn(50),
  )
  err = os.MkdirAll(
    tmppath,
    os.ModePerm,
  )
  if err != nil {
    return
  }

  tmpfile := fmt.Sprintf(
    "%s/%s.download",
    tmppath,
    xid.New().String(),
  )
  dst, err := os.Create(tmpfile)
  if err != nil {
    return
  }
  defer os.Remove(tmpfile)
  defer dst.Close()

  config, _, err = image.DecodeConfig(resp.Body)
  if err != nil {
    return
  }

  return
}
