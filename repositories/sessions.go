package repositories

import (
  "context"
  "errors"
  "fmt"
  "io"
  "net"
  "net/http"
  "regexp"
  "strings"
  "time"

  "github.com/PuerkitoBio/goquery"
  "github.com/nats-io/nats.go"
  "github.com/rs/xid"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/models"
)

type SessionsRepository struct {
  Db   *gorm.DB
  Ctx  context.Context
  Nats *nats.Conn
}

func (r *SessionsRepository) Find(id string) (entity *models.Session, err error) {
  err = r.Db.First(&entity, "id=?", id).Error
  return
}

func (r *SessionsRepository) Get(account string) (entity *models.Session, err error) {
  err = r.Db.Where("account", account).Take(&entity).Error
  return
}

func (r *SessionsRepository) Apply(
  account string,
  cookie string,
  slot int,
) (session *models.Session, err error) {
  result := r.Db.Where("account", account).Take(&session)
  if errors.Is(result.Error, gorm.ErrRecordNotFound) {
    session = &models.Session{
      ID:      xid.New().String(),
      Account: account,
      Node:    common.GetEnvInt("SCRAPER_STORAGE_NODE"),
      Agent:   common.GetEnvString("SCRAPER_AGENT"),
      Cookie:  cookie,
      Slot:    slot,
      Data:    common.JSONMap(&SessionData{}),
      Status:  1,
    }
    err = r.Db.Create(&session).Error
  } else {
    if session.Status != 1 && session.Status != 2 {
      r.Db.Model(&session).Update("status", 1)
    }
  }
  return
}

func (r *SessionsRepository) Current() (session *models.Session) {
  r.Db.Where("node = ? AND status = 1", common.GetEnvInt("SCRAPER_STORAGE_NODE")).Order("timestamp ASC").Take(&session)
  return
}

func (r *SessionsRepository) Special() (session *models.Session) {
  r.Db.Where("node = ? AND status = 8", common.GetEnvInt("SCRAPER_STORAGE_NODE")).Order("timestamp ASC").Take(&session)
  return
}

func (r *SessionsRepository) Flush(session *models.Session) (err error) {
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

  headers := map[string]string{
    "User-Agent": session.Agent,
    "cookie":     session.Cookie,
  }

  url := "https://twitter.com/i/bookmarks"
  req, _ := http.NewRequest("GET", url, nil)
  for key, val := range headers {
    req.Header.Set(key, val)
  }
  resp, err := httpClient.Do(req)
  if err != nil {
    return
  }
  defer resp.Body.Close()

  if resp.StatusCode != http.StatusOK {
    return errors.New(
      fmt.Sprintf(
        "request error: status[%s] code[%d] cookie[%v]",
        resp.Status,
        resp.StatusCode,
        common.GetEnvString("cookie"),
      ),
    )
  }

  doc, err := goquery.NewDocumentFromReader(resp.Body)
  if err != nil {
    return
  }

  doc.Find("script").Each(func(i int, s *goquery.Selection) {
    if src, ok := s.Attr("src"); ok {
      if strings.Contains(src, "client-web/main.") || strings.Contains(src, "client-web-legacy/main.") {
        err = r.ExtractMainJS(session, src)
      }
    }
  })

  return
}

func (r *SessionsRepository) ExtractMainJS(session *models.Session, url string) (err error) {
  tr := &http.Transport{
    DisableKeepAlives: true,
  }
  tr.DialContext = (&net.Dialer{}).DialContext

  httpClient := &http.Client{
    Transport: tr,
    Timeout:   time.Duration(15) * time.Second,
  }

  headers := map[string]string{
    "User-Agent": session.Agent,
    "cookie":     session.Cookie,
  }

  req, _ := http.NewRequest("GET", url, nil)
  for key, val := range headers {
    req.Header.Set(key, val)
  }
  resp, err := httpClient.Do(req)
  if err != nil {
    return
  }
  defer resp.Body.Close()

  if resp.StatusCode != http.StatusOK {
    return errors.New(
      fmt.Sprintf(
        "request error: status[%s] code[%d] cookie[%v]",
        resp.Status,
        resp.StatusCode,
        common.GetEnvString("cookie"),
      ),
    )
  }

  body, _ := io.ReadAll(resp.Body)
  content := string(body)

  re := regexp.MustCompile(`AAAAAAAAAAAAAAA([a-zA-Z0-9-_%]*)`)
  matches := re.FindStringSubmatch(content)
  if len(matches) == 0 {
    return errors.New("access token not found")
  }

  data := &SessionData{}
  data.AccessToken = matches[0]

  re = regexp.MustCompile(`"([a-zA-Z0-9-_]*)",operationName:"UserByScreenName"`)
  matches = re.FindStringSubmatch(content)
  if len(matches) > 1 {
    data.SecionUsers = matches[1]
  }

  re = regexp.MustCompile(`"([a-zA-Z0-9-_]*)",operationName:"UserTweets"`)
  matches = re.FindStringSubmatch(content)
  if len(matches) > 1 {
    data.SectionPosts = matches[1]
  }

  re = regexp.MustCompile(`"([a-zA-Z0-9-_]*)",operationName:"TweetDetail"`)
  matches = re.FindStringSubmatch(content)
  if len(matches) > 1 {
    data.SectionReplies = matches[1]
  }

  r.Db.Model(&session).Updates(map[string]interface{}{
    "data":       common.JSONMap(data),
    "flushed_at": time.Now().UnixMicro(),
  })

  return
}

func (r *SessionsRepository) Update(session *models.Session, column string, value interface{}) (err error) {
  r.Db.Model(&session).Update(column, value)
  return nil
}

func (r *SessionsRepository) Updates(session *models.Session, values map[string]interface{}) (err error) {
  r.Db.Model(&session).Updates(values)
  return nil
}
