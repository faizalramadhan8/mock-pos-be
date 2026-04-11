package enum

type ContextKey string

const (
	ConfigCtxKey ContextKey = "config.ctx.key"
	GormCtxKey   ContextKey = "gorm.ctx.key"
	LoggerCtxKey ContextKey = "logger.ctx.key"
	RedisCtxKey  ContextKey = "redis.ctx.key"
)
