package tasks

import (
  "scraper.local/twitter-scraper/common"
  tasks "scraper.local/twitter-scraper/tasks/tor"
)

type TorTask struct {
  AnsqContext *common.AnsqClientContext
  BridgesTask *tasks.BridgesTask
}

func NewTorTask(ansqContext *common.AnsqClientContext) *TorTask {
  return &TorTask{
    AnsqContext: ansqContext,
  }
}

func (t *TorTask) Bridges() *tasks.BridgesTask {
  if t.BridgesTask == nil {
    t.BridgesTask = tasks.NewBridgesTask(t.AnsqContext)
  }
  return t.BridgesTask
}
