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

type Replies struct {
  NatsContext *common.NatsContext
  Repository  *repositories.TasksRepository
}

func NewReplies(natsContext *common.NatsContext) *Replies {
  h := &Replies{
    NatsContext: natsContext,
  }
  h.Repository = &repositories.TasksRepository{
    Db: h.NatsContext.Db,
  }
  return h
}

func (h *Replies) Subscribe() error {
  h.NatsContext.Conn.Subscribe(config.NATS_REPLIES_CREATE, h.Apply)
  return nil
}

func (h *Replies) Apply(m *nats.Msg) {
  var payload *RepliesCreatePayload
  json.Unmarshal(m.Data, &payload)

  mutex := common.NewMutex(
    h.NatsContext.Rdb,
    h.NatsContext.Ctx,
    fmt.Sprintf(config.LOCKS_TASKS_SCRAPERS_MEDIA_REPLIES_APPLY, payload.ID),
  )
  if !mutex.Lock(3 * time.Second) {
    return
  }
  defer mutex.Unlock()

  name := fmt.Sprintf("%v@media.repies", payload.ID)
  action := config.TASK_ACTION_SCRAPERS_MEDIA_REPLIES
  params := map[string]interface{}{
    "id": payload.ID,
  }
  h.Repository.Apply(name, action, params)
}
