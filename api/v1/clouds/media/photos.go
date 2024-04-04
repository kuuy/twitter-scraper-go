package media

import (
  "errors"
  "fmt"
  "hash/crc32"
  "log"
  "net/http"
  "os"
  "time"

  "github.com/go-chi/chi/v5"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/api"
  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/config"
  repositories "scraper.local/twitter-scraper/repositories/media"
)

type PhotosHandler struct {
  ApiContext *common.ApiContext
  Response   *api.ResponseHandler
  Repository *repositories.PhotosRepository
}

func NewPhotosRouter(apiContext *common.ApiContext) http.Handler {
  h := PhotosHandler{
    ApiContext: apiContext,
  }
  h.Repository = &repositories.PhotosRepository{
    Db: h.ApiContext.Db,
  }

  r := chi.NewRouter()
  r.Post("/notify", h.Notify)

  return r
}

func (h *PhotosHandler) Notify(
  w http.ResponseWriter,
  r *http.Request,
) {
  h.ApiContext.Mux.Lock()
  defer h.ApiContext.Mux.Unlock()

  h.Response = &api.ResponseHandler{
    Writer: w,
  }

  r.ParseForm()

  if r.Form.Get("sourceId") == "" {
    h.Response.Error(http.StatusForbidden, 1004, "sourceId is empty")
    return
  }

  if r.Form.Get("url") == "" {
    h.Response.Error(http.StatusForbidden, 1004, "url is empty")
    return
  }

  log.Println("data", r.Form)

  id := r.Form.Get("sourceId")
  cloudUrl := r.Form.Get("url")
  photo, err := h.Repository.Find(id)
  if errors.Is(err, gorm.ErrRecordNotFound) {
    h.Response.Error(http.StatusForbidden, 1004, "photo not exists")
    return
  }

  mutex := common.NewMutex(
    h.ApiContext.Rdb,
    h.ApiContext.Ctx,
    fmt.Sprintf(config.LOCKS_TASKS_CLOUDS_MEDIA_PHOTOS_NOTIFY, photo.ID),
  )
  if !mutex.Lock(5 * time.Second) {
    h.Response.Error(http.StatusForbidden, 1004, "waiting for the lock release")
    return
  }
  defer mutex.Unlock()

  h.Repository.Updates(photo, map[string]interface{}{
    "cloud_url": cloudUrl,
    "is_synced": true,
  })

  crc32q := crc32.MakeTable(0xD5828281)
  i := crc32.Checksum([]byte(photo.Filehash), crc32q)
  localpath := fmt.Sprintf(
    "%s/photos/%d/%d",
    common.GetEnvString("SCRAPER_STORAGE_PATH"),
    i/233%50,
    i/89%50,
  )
  localfile := fmt.Sprintf(
    "%s/%s.%s",
    localpath,
    photo.Filehash,
    photo.Extension,
  )
  os.Remove(localfile)

  day := time.Now().UTC().Format("0102")

  h.ApiContext.Rdb.ZRem(h.ApiContext.Ctx, config.REDIS_KEY_CLOUDS_SYNCING_MEDIA_PHOTOS, photo.ID)
  h.ApiContext.Rdb.Del(h.ApiContext.Ctx, fmt.Sprintf(config.REDIS_KEY_MEDIA_PHOTOS, photo.UrlSha1, day))

  log.Println("success removed local file", localfile)

  h.Response.Json(nil)
}
