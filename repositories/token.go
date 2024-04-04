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
  "github.com/go-redis/redis/v8"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/config"
)

type TokenRepository struct {
  Rdb *redis.Client
  Ctx context.Context
}

func (r *TokenRepository) Refresh() (err error) {
  tr := &http.Transport{
    DisableKeepAlives: true,
  }
  session := &net.Dialer{}
  tr.DialContext = session.DialContext

  httpClient := &http.Client{
    Transport: tr,
    Timeout:   time.Duration(15) * time.Second,
  }

  headers := map[string]string{
    "User-Agent": common.GetEnvString("SCRAPER_AGENT"),
    "cookie":     common.GetEnvString("SCRAPER_COOKIE"),
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
      if strings.Contains(src, "client-web/main.") {
        err = r.ExtractMainJS(src)
      }
    }
  })

  if err != nil {
    return
  }

  return
}

func (r *TokenRepository) ExtractMainJS(url string) (err error) {
  tr := &http.Transport{
    DisableKeepAlives: true,
  }
  session := &net.Dialer{}
  tr.DialContext = session.DialContext

  httpClient := &http.Client{
    Transport: tr,
    Timeout:   time.Duration(15) * time.Second,
  }

  headers := map[string]string{
    "User-Agent": common.GetEnvString("SCRAPER_AGENT"),
    "cookie":     common.GetEnvString("SCRAPER_COOKIE"),
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
    err = errors.New("access token not found")
    return
  }

  data := map[string]interface{}{
    "access_token": matches[0],
    "flushed_at":   time.Now().Unix(),
  }

  re = regexp.MustCompile(`"([a-zA-Z0-9-_]*)",operationName:"UserTweets"`)
  matches = re.FindStringSubmatch(content)
  if len(matches) > 1 {
    data["section_posts"] = matches[1]
  }

  re = regexp.MustCompile(`"([a-zA-Z0-9-_]*)",operationName:"TweetDetail"`)
  matches = re.FindStringSubmatch(content)
  if len(matches) > 1 {
    data["section_replies"] = matches[1]
  }

  r.Rdb.HMSet(
    r.Ctx,
    fmt.Sprintf(config.REDIS_KEY_SCRAPER),
    data,
  )

  return nil
}
