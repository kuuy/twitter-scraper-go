package common

import (
  "encoding/json"
  "gorm.io/datatypes"
)

func JSONMap(in interface{}) datatypes.JSONMap {
  buf, _ := json.Marshal(in)
  var out datatypes.JSONMap
  json.Unmarshal(buf, &out)
  return out
}
