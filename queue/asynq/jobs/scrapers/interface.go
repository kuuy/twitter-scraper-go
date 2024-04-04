package scrapers

type InitPayload struct {
  UserID string `json:"user_id"`
}

type ProcessPayload struct {
  TaskID string `json:"task_id"`
}
