package tor

import (
  "scraper.local/twitter-scraper/common"
  repositories "scraper.local/twitter-scraper/repositories/tor"
)

type BridgesTask struct {
  AnsqContext *common.AnsqClientContext
  Repository  *repositories.BridgesRepository
}

func NewBridgesTask(ansqContext *common.AnsqClientContext) *BridgesTask {
  return &BridgesTask{
    AnsqContext: ansqContext,
    Repository: &repositories.BridgesRepository{
      Db: ansqContext.Db,
    },
  }
}

func (t *BridgesTask) Flush() error {
  return t.Repository.Flush()
}

func (t *BridgesTask) Rescue() error {
  return t.Repository.Rescue()
}
