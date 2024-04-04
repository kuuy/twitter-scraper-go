package v1

import (
  "net/http"

  "github.com/go-chi/chi/v5"

  "scraper.local/twitter-scraper/api/v1/tasks"
  "scraper.local/twitter-scraper/common"
)

func NewTasksRouter(apiContext *common.ApiContext) http.Handler {
  r := chi.NewRouter()
  r.Mount("/scrapers", tasks.NewScrapersRouter(apiContext))
  return r
}
