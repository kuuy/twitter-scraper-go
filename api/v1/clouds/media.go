package clouds

import (
  "scraper.local/twitter-scraper/api/v1/clouds/media"
  "scraper.local/twitter-scraper/common"
  "github.com/go-chi/chi/v5"
  "net/http"
)

func NewMediaRouter(apiContext *common.ApiContext) http.Handler {
  r := chi.NewRouter()
  r.Mount("/photos", media.NewPhotosRouter(apiContext))
  r.Mount("/videos", media.NewVideosRouter(apiContext))
  return r
}
