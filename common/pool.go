package common

import (
  "context"
  "database/sql"
  "errors"
  "strconv"
  "strings"
  "sync"
  "time"

  "github.com/go-redis/redis/v8"
  "github.com/hibiken/asynq"
  "github.com/nats-io/nats.go"
  "github.com/rs/xid"
  "gorm.io/driver/mysql"
  "gorm.io/driver/postgres"
  "gorm.io/gorm"
)

var (
  dbPool     *sql.DB
  dbTorPool  *sql.DB
  dbVidaPool *sql.DB
)

type ApiContext struct {
  Db  *gorm.DB
  Rdb *redis.Client
  Ctx context.Context
  Mux sync.Mutex
}

type NatsContext struct {
  Db   *gorm.DB
  Rdb  *redis.Client
  Ctx  context.Context
  Conn *nats.Conn
}

type AnsqServerContext struct {
  Db   *gorm.DB
  Rdb  *redis.Client
  Ctx  context.Context
  Mux  *asynq.ServeMux
  Nats *nats.Conn
}

type AnsqClientContext struct {
  Db   *gorm.DB
  Rdb  *redis.Client
  Ctx  context.Context
  Conn *asynq.Client
  Nats *nats.Conn
}

type Mutex struct {
  rdb   *redis.Client
  ctx   context.Context
  key   string
  value string
}

func NewRedis() *redis.Client {
  return redis.NewClient(&redis.Options{
    Addr:     GetEnvString("REDIS_HOST"),
    Password: GetEnvString("REDIS_PASSWORD"),
    DB:       GetEnvInt("REDIS_DB"),
  })
}

func NewTorRedis() *redis.Client {
  return redis.NewClient(&redis.Options{
    Addr:     GetEnvString("REDIS_TOR_HOST"),
    Password: GetEnvString("REDIS_TOR_PASSWORD"),
    DB:       GetEnvInt("REDIS_TOR_DB"),
  })
}

func NewDBPool() *sql.DB {
  if dbPool == nil {
    dsn := GetEnvString("DB_DSN")
    pool, err := sql.Open("pgx", dsn)
    if err != nil {
      panic(err)
    }
    pool.SetMaxIdleConns(50)
    pool.SetMaxOpenConns(100)
    pool.SetConnMaxLifetime(5 * time.Minute)
    dbPool = pool
  }
  return dbPool
}

func NewDB() *gorm.DB {
  db, err := gorm.Open(postgres.New(postgres.Config{
    Conn: NewDBPool(),
  }), &gorm.Config{})
  if errors.Is(err, context.DeadlineExceeded) {
    return NewDB()
  }
  if err != nil {
    panic(err)
  }
  return db
}

func NewTorDBPool() *sql.DB {
  if dbTorPool == nil {
    dsn := GetEnvString("DB_TOR_DSN")
    pool, err := sql.Open("pgx", dsn)
    if err != nil {
      panic(err)
    }
    pool.SetMaxIdleConns(50)
    pool.SetMaxOpenConns(100)
    pool.SetConnMaxLifetime(5 * time.Minute)
    dbTorPool = pool
  }
  return dbTorPool
}

func NewTorDB() *gorm.DB {
  db, err := gorm.Open(postgres.New(postgres.Config{
    Conn: NewTorDBPool(),
  }), &gorm.Config{})
  if errors.Is(err, context.DeadlineExceeded) {
    return NewTorDB()
  }
  if err != nil {
    panic(err)
  }
  return db
}

func NewVidaDBPool() *sql.DB {
  if dbTorPool == nil {
    dsn := GetEnvString("DB_VIDA_DSN")
    pool, err := sql.Open("mysql", dsn)
    if err != nil {
      panic(err)
    }
    pool.SetMaxIdleConns(50)
    pool.SetMaxOpenConns(100)
    pool.SetConnMaxLifetime(5 * time.Minute)
    dbVidaPool = pool
  }
  return dbVidaPool
}

func NewVidaDB() *gorm.DB {
  db, err := gorm.Open(mysql.New(mysql.Config{
    Conn: NewVidaDBPool(),
  }), &gorm.Config{})
  if errors.Is(err, context.DeadlineExceeded) {
    return NewVidaDB()
  }
  if err != nil {
    panic(err)
  }
  return db
}

func NewAsynqServer() *asynq.Server {
  rdb := asynq.RedisClientOpt{
    Addr: GetEnvString("ASYNQ_REDIS_ADDR"),
    DB:   GetEnvInt("ASYNQ_REDIS_DB"),
  }
  queues := make(map[string]int)
  for _, item := range GetEnvArray("ASYNQ_QUEUE") {
    data := strings.Split(item, ",")
    weight, _ := strconv.Atoi(data[1])
    queues[data[0]] = weight
  }
  return asynq.NewServer(rdb, asynq.Config{
    Concurrency: GetEnvInt("ASYNQ_CONCURRENCY"),
    Queues:      queues,
  })
}

func NewAsynqClient() *asynq.Client {
  return asynq.NewClient(asynq.RedisClientOpt{
    Addr: GetEnvString("ASYNQ_REDIS_ADDR"),
    DB:   GetEnvInt("ASYNQ_REDIS_DB"),
  })
}

func NewNats() *nats.Conn {
  nc, err := nats.Connect("127.0.0.1", nats.Token(GetEnvString("NATS_TOKEN")))
  if err != nil {
    panic(err)
  }
  return nc
}

func NewMutex(
  rdb *redis.Client,
  ctx context.Context,
  key string,
) *Mutex {
  return &Mutex{
    rdb:   rdb,
    ctx:   ctx,
    key:   key,
    value: xid.New().String(),
  }
}

func (m *Mutex) Lock(ttl time.Duration) bool {
  result, err := m.rdb.SetNX(
    m.ctx,
    m.key,
    m.value,
    ttl,
  ).Result()
  if err != nil {
    return false
  }
  return result
}

func (m *Mutex) Unlock() {
  script := redis.NewScript(`
  if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
  else
    return 0
  end
  `)
  script.Run(m.ctx, m.rdb, []string{m.key}, m.value).Result()
}
