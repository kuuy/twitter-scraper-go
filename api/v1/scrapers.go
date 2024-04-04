package v1

import (
  "net/http"

  "github.com/go-chi/chi/v5"

  "scraper.local/twitter-scraper/api/v1/clouds"
  "scraper.local/twitter-scraper/api/v1/scrapers"
  "scraper.local/twitter-scraper/common"
)

func NewScrapersRouter(apiContext *common.ApiContext) http.Handler {
  r := chi.NewRouter()
  r.Mount("/posts", scrapers.NewPostsRouter(apiContext))
  r.Mount("/replies", scrapers.NewRepliesRouter(apiContext))
  r.Mount("/media", clouds.NewMediaRouter(apiContext))
  return r
}
