package database

import (
	"context"

	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type Redis struct {
	Client     *redis.Client
	Log        *zerolog.Logger
	Connection *config.Config
	DB         *gorm.DB
}

func NewRedis(ctx context.Context, db *gorm.DB) *Redis {
	log := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	cfg := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	return &Redis{
		DB:         db,
		Log:        log,
		Connection: cfg,
	}
}

func (r *Redis) ConnectRedis(ctx context.Context) error {
	r.Client = redis.NewClient(&redis.Options{
		Addr:     r.Connection.RedisAddr,
		Password: r.Connection.RedisPass,
		DB:       r.Connection.RedisDB,
		Protocol: 2,
	})

	pong, err := r.Client.Ping(ctx).Result()
	if err != nil {
		r.Log.Fatal().Err(err).Msg("Failed to connect to Redis")
		return err
	}

	r.Log.Info().Msgf("Connected to Redis: %s", pong)
	return nil
}

func (r *Redis) GetRedisClient(ctx context.Context) *redis.Client {
	if r.Client == nil {
		if err := r.ConnectRedis(ctx); err != nil {
			panic(err)
		}
	}
	return r.Client
}
