package v1

import (
  "net/http"

  "github.com/go-chi/chi/v5"

  "scraper.local/twitter-scraper/api"
  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/repositories"
  jwtRepositories "scraper.local/twitter-scraper/repositories/jwt"
)

type LoginHandler struct {
  ApiContext       *common.ApiContext
  Response         *api.ResponseHandler
  AdminsRepository *repositories.AdminsRepository
  TokenRepository  *jwtRepositories.TokenRepository
}

type Token struct {
  AccessToken  string `json:"access_token"`
  RefreshToken string `json:"refresh_token"`
}

func NewLoginRouter(apiContext *common.ApiContext) http.Handler {
  h := LoginHandler{
    ApiContext: apiContext,
  }
  h.AdminsRepository = &repositories.AdminsRepository{
    Db: h.ApiContext.Db,
  }

  r := chi.NewRouter()
  r.Post("/", h.Do)

  return r
}

func (h *LoginHandler) Token() *jwtRepositories.TokenRepository {
  if h.TokenRepository == nil {
    h.TokenRepository = &jwtRepositories.TokenRepository{}
  }
  return h.TokenRepository
}

func (h *LoginHandler) Do(
  w http.ResponseWriter,
  r *http.Request,
) {
  h.Response = &api.ResponseHandler{
    Writer: w,
  }

  r.ParseMultipartForm(1024)

  if r.Form.Get("account") == "" {
    h.Response.Error(http.StatusForbidden, 1004, "account is empty")
    return
  }

  if r.Form.Get("password") == "" {
    h.Response.Error(http.StatusForbidden, 1004, "password is empty")
    return
  }

  account := r.Form.Get("account")
  password := r.Form.Get("password")

  user := h.AdminsRepository.Get(account)
  if user == nil {
    h.Response.Error(http.StatusForbidden, 1000, "account not exists")
    return
  }
  if !common.VerifyPassword(password, user.Salt, user.Password) {
    h.Response.Error(http.StatusForbidden, 1000, "password is wrong")
    return
  }

  accessToken, err := h.Token().AccessToken(user.ID)
  if err != nil {
    h.Response.Error(http.StatusInternalServerError, 500, "server error")
    return
  }
  refreshToken, err := h.Token().RefreshToken(user.ID)
  if err != nil {
    h.Response.Error(http.StatusInternalServerError, 500, "server error")
    return
  }

  token := &Token{
    AccessToken:  accessToken,
    RefreshToken: refreshToken,
  }

  h.Response.Json(token)
}
