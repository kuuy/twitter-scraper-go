package media

import (
  "encoding/json"
  "fmt"
  "time"

  "github.com/nats-io/nats.go"

  "scraper.local/twitter-scraper/common"
  "scraper.local/twitter-scraper/config"
  "scraper.local/twitter-scraper/repositories"
)

type Users struct {
  NatsContext *common.NatsContext
  Repository  *repositories.TasksRepository
}

func NewUsers(natsContext *common.NatsContext) *Users {
  h := &Users{
    NatsContext: natsContext,
  }
  h.Repository = &repositories.TasksRepository{
    Db: h.NatsContext.Db,
  }
  return h
}

func (h *Users) Subscribe() error {
  h.NatsContext.Conn.Subscribe(config.NATS_USERS_CREATE, h.Apply)
  return nil
}

func (h *Users) Apply(m *nats.Msg) {
  var payload *UsersCreatePayload
  json.Unmarshal(m.Data, &payload)

  mutex := common.NewMutex(
    h.NatsContext.Rdb,
    h.NatsContext.Ctx,
    fmt.Sprintf(config.LOCKS_TASKS_SCRAPERS_MEDIA_USERS_APPLY, payload.ID),
  )
  if !mutex.Lock(3 * time.Second) {
    return
  }
  defer mutex.Unlock()

  name := fmt.Sprintf("%v@media.users", payload.ID)
  action := config.TASK_ACTION_SCRAPERS_MEDIA_USERS
  params := map[string]interface{}{
    "id": payload.ID,
  }
  h.Repository.Apply(name, action, params)
}
