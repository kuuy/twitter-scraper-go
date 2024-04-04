package repositories

import (
  "errors"
  "fmt"

  "github.com/rs/xid"
  "gorm.io/gorm"

  "scraper.local/twitter-scraper/models"
)

type TasksRepository struct {
  Db *gorm.DB
}

func (r *TasksRepository) Find(id string) (task *models.Task, err error) {
  err = r.Db.First(&task, "id=?", id).Error
  return
}

func (r *TasksRepository) Count(conditions map[string]interface{}) int64 {
  var total int64
  query := r.Db.Model(&models.Task{})
  if _, ok := conditions["name"]; ok {
    query.Where("name=?", conditions["name"].(string))
  }
  if _, ok := conditions["action"]; ok {
    query.Where("action=?", conditions["action"].(int))
  }
  if _, ok := conditions["status"]; ok {
    query.Where("status", conditions["status"].(int))
  } else {
    query.Where("status IN (1,2)")
  }
  query.Count(&total)
  return total
}

func (r *TasksRepository) Listings(conditions map[string]interface{}, current int, pageSize int) []*models.Task {
  var tasks []*models.Task
  query := r.Db.Select([]string{
    "id",
    "name",
    "action",
    "params",
    "timestamp",
    "status",
    "created_at",
    "updated_at",
  })
  if _, ok := conditions["name"]; ok {
    query.Where("name=?", conditions["name"].(string))
  }
  if _, ok := conditions["action"]; ok {
    query.Where("action=?", conditions["action"].(int))
  }
  if _, ok := conditions["status"]; ok {
    query.Where("status", conditions["status"].(int))
  } else {
    query.Where("status IN (1,2)")
  }
  query.Order("created_at desc")
  query.Offset((current - 1) * pageSize).Limit(pageSize).Find(&tasks)
  return tasks
}

func (r *TasksRepository) Ranking(
  fields []string,
  conditions map[string]interface{},
  sortField string,
  sortType int,
  limit int,
) []*models.Task {
  var tasks []*models.Task
  query := r.Db.Select(fields)
  if _, ok := conditions["action"]; ok {
    query.Where("action", conditions["action"].(int))
  }
  if _, ok := conditions["ids"]; ok {
    query.Where("id IN ?", conditions["ids"].([]string))
  }
  if _, ok := conditions["timestamp"]; ok {
    if sortType == 1 {
      query.Where("timestamp>?", conditions["timestamp"].(int64))
    } else if sortType == -1 {
      query.Where("timestamp<?", conditions["timestamp"].(int64))
    }
  }
  if _, ok := conditions["status"]; ok {
    query.Where("status", conditions["status"].(int))
  } else {
    query.Where("status=1")
  }
  if sortType == 1 {
    query.Order(fmt.Sprintf("%v ASC", sortField))
  } else if sortType == -1 {
    query.Order(fmt.Sprintf("%v DESC", sortField))
  }
  query.Limit(limit).Find(&tasks)
  return tasks
}

func (r *TasksRepository) Apply(name string, action int, params map[string]interface{}) (err error) {
  var task models.Task
  result := r.Db.Where("name", name).Take(&task)
  if errors.Is(result.Error, gorm.ErrRecordNotFound) {
    task = models.Task{
      ID:     xid.New().String(),
      Name:   name,
      Action: action,
      Params: params,
      Status: 1,
    }
    r.Db.Create(&task)
  } else {
    if task.Status != 1 && task.Status != 2 {
      r.Db.Model(&task).Update("status", 1)
    }
  }
  return
}

func (r *TasksRepository) Update(task *models.Task, column string, value interface{}) (err error) {
  r.Db.Model(&task).Update(column, value)
  return nil
}

func (r *TasksRepository) Updates(task *models.Task, values map[string]interface{}) (err error) {
  r.Db.Model(&task).Updates(values)
  return nil
}

func (r *TasksRepository) Delete(id string) (err error) {
  r.Db.Delete(&models.Task{ID: id})
  return nil
}
