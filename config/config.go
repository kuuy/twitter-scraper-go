package config

const (
  REDIS_KEY_SCRAPER                          = "twitter:scraper"
  REDIS_KEY_TASKS_POSTS_TARGET               = "twitter:scraper:tasks:posts:target"
  REDIS_KEY_TASKS_REPLIES_TARGET             = "twitter:scraper:tasks:replies:target"
  REDIS_KEY_TASKS_USERS_POSTS_TARGET         = "twitter:scraper:tasks:users:posts:target"
  REDIS_KEY_CLOUDS_SYNCING_MEDIA_PHOTOS      = "twitter:scraper:clouds:syncing:media:photos"
  REDIS_KEY_CLOUDS_SYNCING_MEDIA_VIDEOS      = "twitter:scraper:clouds:syncing:media:videos"
  REDIS_KEY_POSTS_COUNT                      = "twitter:scraper:posts:count:%s"
  REDIS_KEY_REPLIES_COUNT                    = "twitter:scraper:replies:count:%s"
  REDIS_KEY_MEDIA_VIDEOS                     = "twitter:scraper:media:videos:%s:%s"
  REDIS_KEY_MEDIA_PHOTOS                     = "twitter:scraper:media:photos:%s:%s"
  SCRAPERS_POSTS_TARGET_LIMIT                = 20
  SCRAPERS_REPLIES_TARGET_LIMIT              = 50
  SCRAPERS_USERS_POSTS_TARGET_LIMIT          = 50
  SCRAPERS_CURSOR_WAITING_TIMEOUT            = 300000
  CLOUDS_SYNCING_MEDIA_PHOTOS_LIMIT          = 200
  CLOUDS_SYNCING_MEDIA_VIDEOS_LIMIT          = 50
  TASK_ACTION_SCRAPERS_POSTS                 = 1
  TASK_ACTION_SCRAPERS_REPLIES               = 2
  TASK_ACTION_SCRAPERS_MEDIA_USERS           = 3
  TASK_ACTION_SCRAPERS_MEDIA_POSTS           = 4
  TASK_ACTION_SCRAPERS_MEDIA_REPLIES         = 5
  TASK_ACTION_SCRAPERS_USERS_POSTS           = 6
  NATS_POSTS_CREATE                          = "twitter:posts:create"
  NATS_REPLIES_CREATE                        = "twitter:replies:create"
  NATS_USERS_CREATE                          = "twitter:users:create"
  ASYNQ_QUEUE_SESSIONS                       = "twitter:sessions"
  ASYNQ_QUEUE_SCRAPERS_POSTS                 = "twitter:scrapers:posts"
  ASYNQ_QUEUE_SCRAPERS_REPLIES               = "twitter:scrapers:replies"
  ASYNQ_QUEUE_SCRAPERS_USERS_POSTS           = "twitter:scrapers:users:posts"
  ASYNQ_JOBS_SESSIONS_FLUSH                  = "twitter:sessions:flush"
  ASYNQ_JOBS_SCRAPERS_POSTS_FLUSH            = "twitter:scrapers:posts:flush"
  ASYNQ_JOBS_SCRAPERS_POSTS_PROCESS          = "twitter:scrapers:posts:process"
  ASYNQ_JOBS_SCRAPERS_REPLIES_INIT           = "twitter:scrapers:replies:init"
  ASYNQ_JOBS_SCRAPERS_REPLIES_FLUSH          = "twitter:scrapers:replies:flush"
  ASYNQ_JOBS_SCRAPERS_REPLIES_PROCESS        = "twitter:scrapers:replies:process"
  ASYNQ_JOBS_SCRAPERS_USERS_POSTS_FLUSH      = "twitter:scrapers:users:posts:flush"
  ASYNQ_JOBS_SCRAPERS_USERS_POSTS_PROCESS    = "twitter:scrapers:users:posts:process"
  LOCKS_TASKS_POSTS_FLUSH                    = "locks:twitter:tasks:posts:flush:%v"
  LOCKS_TASKS_REPLIES_FLUSH                  = "locks:twitter:tasks:replies:flush:%v"
  LOCKS_TASKS_CLOUDS_MEDIA_PHOTOS_SYNC       = "locks:twitter:tasks:clouds:media:photos:sync:%v"
  LOCKS_TASKS_CLOUDS_MEDIA_VIDEOS_SYNC       = "locks:twitter:tasks:clouds:media:videos:sync:%v"
  LOCKS_TASKS_CLOUDS_MEDIA_PHOTOS_NOTIFY     = "locks:twitter:tasks:clouds:media:photos:notify:%v"
  LOCKS_TASKS_CLOUDS_MEDIA_VIDEOS_NOTIFY     = "locks:twitter:tasks:clouds:media:videos:notify:%v"
  LOCKS_TASKS_SCRAPERS_REPLIES_APPLY         = "locks:twitter:tasks:scrapers:replies:apply:%v"
  LOCKS_TASKS_SCRAPERS_MEDIA_USERS_APPLY     = "locks:twitter:tasks:scrapers:media:users:apply:%v"
  LOCKS_TASKS_SCRAPERS_MEDIA_POSTS_APPLY     = "locks:twitter:tasks:scrapers:media:posts:apply:%v"
  LOCKS_TASKS_SCRAPERS_MEDIA_REPLIES_APPLY   = "locks:twitter:tasks:scrapers:media:replies:apply:%v"
  LOCKS_TASKS_SCRAPERS_POSTS_FLUSH           = "locks:twitter:tasks:scrapers:posts:flush:%v"
  LOCKS_TASKS_SCRAPERS_POSTS_PROCESS         = "locks:twitter:tasks:scrapers:posts:process:%v"
  LOCKS_TASKS_SCRAPERS_USERS_POSTS_FLUSH     = "locks:twitter:tasks:scrapers:users:posts:flush:%v"
  LOCKS_TASKS_SCRAPERS_USERS_POSTS_PROCESS   = "locks:twitter:tasks:scrapers:users:posts:process:%v"
  LOCKS_TASKS_SCRAPERS_REPLIES_INIT          = "locks:twitter:tasks:scrapers:replies:init:%v"
  LOCKS_TASKS_SCRAPERS_REPLIES_FLUSH         = "locks:twitter:tasks:scrapers:replies:flush:%v"
  LOCKS_TASKS_SCRAPERS_REPLIES_PROCESS       = "locks:twitter:tasks:scrapers:replies:process:%v"
  LOCKS_TASKS_SCRAPERS_MEDIA_USERS_PROCESS   = "locks:twitter:tasks:scrapers:media:users:process:%v"
  LOCKS_TASKS_SCRAPERS_MEDIA_POSTS_PROCESS   = "locks:twitter:tasks:scrapers:media:posts:process:%v"
  LOCKS_TASKS_SCRAPERS_MEDIA_REPLIES_PROCESS = "locks:twitter:tasks:scrapers:media:replies:process:%v"
)
