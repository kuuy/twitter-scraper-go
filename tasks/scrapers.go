package tasks

import (
  "scraper.local/twitter-scraper/common"
  tasks "scraper.local/twitter-scraper/tasks/scrapers"
)

type ScrapersTask struct {
  AnsqContext *common.AnsqClientContext
  UsersTask   *tasks.UsersTask
  PostsTask   *tasks.PostsTask
  RepliesTask *tasks.RepliesTask
}

func NewScrapersTask(ansqContext *common.AnsqClientContext) *ScrapersTask {
  return &ScrapersTask{
    AnsqContext: ansqContext,
  }
}

func (t *ScrapersTask) Users() *tasks.UsersTask {
  if t.UsersTask == nil {
    t.UsersTask = tasks.NewUsersTask(t.AnsqContext)
  }
  return t.UsersTask
}

func (t *ScrapersTask) Posts() *tasks.PostsTask {
  if t.PostsTask == nil {
    t.PostsTask = tasks.NewPostsTask(t.AnsqContext)
  }
  return t.PostsTask
}

func (t *ScrapersTask) Replies() *tasks.RepliesTask {
  if t.RepliesTask == nil {
    t.RepliesTask = tasks.NewRepliesTask(t.AnsqContext)
  }
  return t.RepliesTask
}
