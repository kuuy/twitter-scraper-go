package media

import (
  "bufio"
  "crypto/sha1"
  "encoding/hex"
  "errors"
  "fmt"
  "hash/crc32"
  "io"
  "log"
  "math/rand"
  "net"
  "net/http"
  "os"
  "os/exec"
  "strings"
  "syscall"
  "time"

  "github.com/h2non/filetype"
  "github.com/rs/xid"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/common"
  models "scraper.local/twitter-scraper/models/media"
)

type VideosRepository struct {
  Db *gorm.DB
}

func (r *VideosRepository) Download(url string, urlSha1 string) (err error) {
  tr := &http.Transport{
    DisableKeepAlives: true,
  }
  session := &net.Dialer{}
  tr.DialContext = session.DialContext

  httpClient := &http.Client{
    Transport: tr,
    Timeout:   time.Duration(15) * time.Minute,
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

  kind, _ := filetype.Video(head)
  if kind == filetype.Unknown {
    err = errors.New("unknow filetype")
    return
  }

  info, err := dst.Stat()
  if err != nil {
    return
  }

  filehash := hex.EncodeToString(hash.Sum(nil))
  crc32q := crc32.MakeTable(0xD5828281)
  i := crc32.Checksum([]byte(filehash), crc32q)
  localpath := fmt.Sprintf(
    "%s/videos/%d/%d",
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

  status := 1
  if kind.MIME.Value != "video/mp4" {
    status = 4
  }

  if r.IsCorrupted(tmpfile) {
    status = 4
  }

  var video *models.Video
  if err := r.Db.Where("url_sha1=? AND url=?", urlSha1, url).Take(&video).Error; errors.Is(err, gorm.ErrRecordNotFound) {
    video = &models.Video{
      ID:        xid.New().String(),
      Url:       url,
      UrlSha1:   urlSha1,
      Mime:      kind.MIME.Value,
      Size:      info.Size(),
      Node:      common.GetEnvInt("SCRAPER_STORAGE_NODE"),
      Filehash:  filehash,
      Extension: kind.Extension,
      Timestamp: time.Now().UnixMilli(),
      Status:    status,
    }
    var syncedVideo *models.Video
    if err := r.Db.Where("filehash=? AND is_synced=?", filehash, true).Take(&syncedVideo).Error; err == nil {
      video.CloudUrl = syncedVideo.CloudUrl
      video.IsSynced = true
    }
    r.Db.Create(&video)
    if status == 1 {
      os.Rename(tmpfile, localfile)
    }
  }

  return
}

func (r *VideosRepository) IsCorrupted(path string) bool {
  var args []string
  args = append(args, "-v")
  args = append(args, "error")
  args = append(args, "-i")
  args = append(args, path)
  cmd := exec.Command("/usr/bin/ffmpeg", args...)
  stdout, err := cmd.StdoutPipe()
  cmd.Stderr = cmd.Stdout
  if err != nil {
    log.Println("error occurred", err.Error())
    return false
  }
  if err = cmd.Start(); err != nil {
    log.Println("error occurred", err.Error())
    return false
  }
  pid := cmd.Process.Pid
  defer func() {
    syscall.Kill(pid, syscall.SIGKILL)
  }()
  scanner := bufio.NewScanner(stdout)
  for scanner.Scan() {
    content := scanner.Text()
    if strings.Contains(content, "Invalid NAL unit size") {
      return true
    }
  }
  return false
}
