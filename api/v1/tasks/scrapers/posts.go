package scrapers

import (
  "errors"
  "fmt"
  "net/http"
  "regexp"
  "strconv"
  "strings"

  "github.com/go-chi/chi/v5"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/api"
  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/config"
  "scraper.local/twitter-scraper/repositories"
  scrapersRepository "scraper.local/twitter-scraper/repositories/scrapers"
)

type PostsHandler struct {
  ApiContext         *common.ApiContext
  Response           *api.ResponseHandler
  Repository         *repositories.TasksRepository
  SessionsRepository *repositories.SessionsRepository
  UsersRepository    *repositories.UsersRepository
  ScrapersRepository *scrapersRepository.UsersRepository
}

func NewPostsRouter(apiContext *common.ApiContext) http.Handler {
  h := PostsHandler{
    ApiContext: apiContext,
  }
  h.Repository = &repositories.TasksRepository{
    Db: h.ApiContext.Db,
  }
  h.SessionsRepository = &repositories.SessionsRepository{
    Db: h.ApiContext.Db,
  }
  h.UsersRepository = &repositories.UsersRepository{
    Db: h.ApiContext.Db,
  }
  h.ScrapersRepository = &scrapersRepository.UsersRepository{
    Db: h.ApiContext.Db,
  }
  h.ScrapersRepository.UsersRepository = &repositories.UsersRepository{
    Db: h.ApiContext.Db,
  }

  r := chi.NewRouter()
  //r.Use(api.Authenticator)
  r.Get("/", h.Listings)
  r.Post("/", h.Apply)
  r.Put("/", h.Apply)

  return r
}

func (h *PostsHandler) Listings(
  w http.ResponseWriter,
  r *http.Request,
) {
  h.ApiContext.Mux.Lock()
  defer h.ApiContext.Mux.Unlock()

  h.Response = &api.ResponseHandler{
    Writer: w,
  }

  q := r.URL.Query()

  var current int
  if !q.Has("current") {
    current = 1
  }
  current, _ = strconv.Atoi(r.URL.Query().Get("current"))
  if current < 1 {
    h.Response.Error(http.StatusForbidden, 1004, "current not valid")
    return
  }

  var pageSize int
  if !q.Has("page_size") {
    pageSize = 50
  } else {
    pageSize, _ = strconv.Atoi(r.URL.Query().Get("page_size"))
  }
  if pageSize < 1 || pageSize > 100 {
    h.Response.Error(http.StatusForbidden, 1004, "page size not valid")
    return
  }

  conditions := map[string]interface{}{
    "action": 1,
  }

  if q.Get("account") != "" {
    conditions["account"] = r.URL.Query().Get("account")
  }

  if q.Get("status") != "" {
    conditions["status"], _ = strconv.Atoi(r.URL.Query().Get("status"))
  }

  total := h.Repository.Count(conditions)
  tasks := h.Repository.Listings(conditions, current, pageSize)
  data := make([]*PostInfo, len(tasks))
  for i, task := range tasks {
    user, _ := h.UsersRepository.Find(task.Params["user_id"].(string))
    data[i] = &PostInfo{
      ID:        task.ID,
      Account:   user.Account,
      Timestamp: task.Timestamp,
      Status:    task.Status,
      CreatedAt: task.CreatedAt,
      UpdatedAt: task.UpdatedAt,
    }
  }

  h.Response.Pagenate(data, total, current, pageSize)
}

func (h *PostsHandler) Apply(
  w http.ResponseWriter,
  r *http.Request,
) {
  h.Response = &api.ResponseHandler{
    Writer: w,
  }

  r.ParseForm()

  d := r.Form

  account := strings.TrimSpace(d.Get("account"))

  if account == "" {
    h.Response.Error(http.StatusForbidden, 1004, "account is empty")
    return
  }

  re := regexp.MustCompile(`([a-zA-Z0-9-_]*)$`)
  matches := re.FindStringSubmatch(account)
  if len(matches) > 1 {
    account = matches[1]
  }

  user, err := h.UsersRepository.Get(account)
  if errors.Is(err, gorm.ErrRecordNotFound) {
    session := h.SessionsRepository.Special()
    if session == nil {
      h.Response.Error(http.StatusForbidden, 1000, "current session is empty")
      return
    }
    user, err = h.ScrapersRepository.Process(session, account)
    if err != nil {
      h.Response.Error(http.StatusForbidden, 1000, "user scraper failed")
      return
    }
  }

  name := fmt.Sprintf("%v@posts", user.ID)
  params := map[string]interface{}{
    "user_id": user.ID,
  }

  err = h.Repository.Apply(name, config.TASK_ACTION_SCRAPERS_POSTS, params)
  if err != nil {
    h.Response.Error(http.StatusForbidden, 1000, "task apply failed")
    return
  }

  h.Response.Json(nil)
}
