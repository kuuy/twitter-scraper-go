package scrapers

import (
  "scraper.local/twitter-scraper/common"
  tasks "scraper.local/twitter-scraper/tasks/scrapers/users"
)

type UsersTask struct {
  AnsqContext *common.AnsqClientContext
  PostsTask   *tasks.PostsTask
}

func NewUsersTask(ansqContext *common.AnsqClientContext) *UsersTask {
  return &UsersTask{
    AnsqContext: ansqContext,
  }
}

func (t *UsersTask) Posts() *tasks.PostsTask {
  if t.PostsTask == nil {
    t.PostsTask = tasks.NewPostsTask(t.AnsqContext)
  }
  return t.PostsTask
}
