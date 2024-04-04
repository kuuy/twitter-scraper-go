package tasks

import (
  "net/http"

  "github.com/go-chi/chi/v5"

  "scraper.local/twitter-scraper/api/v1/tasks/scrapers"
  "scraper.local/twitter-scraper/common"
)

func NewScrapersRouter(apiContext *common.ApiContext) http.Handler {
  r := chi.NewRouter()
  r.Mount("/posts", scrapers.NewPostsRouter(apiContext))
  return r
}
