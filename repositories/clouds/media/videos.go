package media

import (
  "bytes"
  "context"
  "crypto/md5"
  "crypto/sha256"
  "encoding/hex"
  "encoding/json"
  "errors"
  "fmt"
  "hash/crc32"
  "log"
  "net"
  "net/http"
  "net/url"
  "os"
  "sort"
  "strings"
  "time"

  "gorm.io/gorm"

  "scraper.local/twitter-scraper/common"
  models "scraper.local/twitter-scraper/models/media"
  repositories "scraper.local/twitter-scraper/repositories/media"
)

type VideosRepository struct {
  Db               *gorm.DB
  Ctx              context.Context
  VideosRepository *repositories.VideosRepository
}

func (r *VideosRepository) Sync(video *models.Video) (err error) {
  tr := &http.Transport{
    DisableKeepAlives: true,
  }
  session := &net.Dialer{}
  tr.DialContext = session.DialContext

  httpClient := &http.Client{
    Transport: tr,
    Timeout:   time.Duration(30) * time.Second,
  }

  crc32q := crc32.MakeTable(0xD5828281)
  i := crc32.Checksum([]byte(video.Filehash), crc32q)
  localpath := fmt.Sprintf(
    "%s/videos/%d/%d",
    common.GetEnvString("SCRAPER_STORAGE_PATH"),
    i/233%50,
    i/89%50,
  )
  localfile := fmt.Sprintf(
    "%s/%s.%s",
    localpath,
    video.Filehash,
    video.Extension,
  )
  sourceUrl := fmt.Sprintf(
    "%s/videos/%d/%d/%s.%s",
    common.GetEnvString(fmt.Sprintf("SCRAPER_STORAGE_URL_%v", video.Node)),
    i/233%50,
    i/89%50,
    video.Filehash,
    video.Extension,
  )

  if _, err = os.Stat(localfile); err != nil {
    return
  }

  data := url.Values{}
  data.Add("sourceId", video.ID)
  data.Add("sourceUrl", sourceUrl)
  data.Add("notifyUrl", fmt.Sprintf("%v/v1/clouds/media/videos/notify", common.GetEnvString("SCRAPER_API_URL")))

  var keys []string
  for k := range data {
    keys = append(keys, k)
  }
  sort.Strings(keys)
  var buf strings.Builder
  for _, k := range keys {
    if buf.Len() > 0 {
      buf.WriteByte('&')
    }
    buf.WriteString(k)
    buf.WriteByte('=')
    buf.WriteString(data.Get(k))
  }
  hashed := sha256.Sum256([]byte(fmt.Sprintf("%s%s", buf.String(), common.GetEnvString("CLOUDS_SYNC_SIGN_KEY"))))
  hash := md5.Sum([]byte(hex.EncodeToString(hashed[:])))
  data.Add("sign", hex.EncodeToString(hash[:]))
  body := bytes.NewBufferString(data.Encode())

  log.Println("data", data)

  url := fmt.Sprintf("%s/api/system/video", common.GetEnvString("CLOUDS_SYNC_URL"))
  req, _ := http.NewRequest("POST", url, body)
  req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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

  var result map[string]interface{}
  json.NewDecoder(resp.Body).Decode(&result)

  if _, ok := result["status"]; !ok {
    log.Println("result", result)
    err = errors.New("clouds media videos sync failed")
    return
  }

  if int(result["status"].(float64)) != 1 {
    err = errors.New(result["msg"].(string))
    return
  }

  log.Println("clouds media videos sync result", result)

  return
}
