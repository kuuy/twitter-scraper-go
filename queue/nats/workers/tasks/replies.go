package tasks

import (
  "encoding/json"
  "fmt"
  "log"
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
  h.NatsContext.Conn.Subscribe(config.NATS_POSTS_CREATE, h.Apply)
  return nil
}

func (h *Replies) Apply(m *nats.Msg) {
  var payload *PostsCreatePayload
  json.Unmarshal(m.Data, &payload)

  if common.GetEnvInt("SCRAPER_POSTS_ONLY") == 1 {
    log.Println("scrapper posts only")
    return
  }

  mutex := common.NewMutex(
    h.NatsContext.Rdb,
    h.NatsContext.Ctx,
    fmt.Sprintf(config.LOCKS_TASKS_SCRAPERS_REPLIES_APPLY, payload.ID),
  )
  if !mutex.Lock(3 * time.Second) {
    return
  }
  defer mutex.Unlock()

  name := fmt.Sprintf("%v@replies", payload.ID)
  action := config.TASK_ACTION_SCRAPERS_REPLIES
  params := map[string]interface{}{
    "post_id": payload.ID,
  }
  h.Repository.Apply(name, action, params)
}
