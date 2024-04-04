package repositories

type SessionData struct {
  AccessToken    string `json:"access_token"`
  SecionUsers    string `json:"section_users"`
  SectionPosts   string `json:"section_posts"`
  SectionReplies string `json:"section_replies"`
}

type TokenInfo struct {
  AccessToken string `json:"access_token"`
  CsrfToken   int    `json:"csrf_token"`
  RefreshedAt int    `json:"refreshed_at"`
}
