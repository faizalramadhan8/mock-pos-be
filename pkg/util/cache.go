package util

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func InvalidateUserSessionCache(redis *redis.Client, userID string) error {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("user:session:%s", userID)
	return redis.Del(ctx, cacheKey).Err()
}

func GetUserSessionCacheKey(userID string) string {
	return fmt.Sprintf("user:session:%s", userID)
}
