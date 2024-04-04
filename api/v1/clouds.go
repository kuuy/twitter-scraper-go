package v1

import (
  "scraper.local/twitter-scraper/api/v1/clouds"
  "scraper.local/twitter-scraper/common"
  "github.com/go-chi/chi/v5"
  "net/http"
)

func NewCloudsRouter(apiContext *common.ApiContext) http.Handler {
  r := chi.NewRouter()
  r.Mount("/media", clouds.NewMediaRouter(apiContext))
  return r
}
