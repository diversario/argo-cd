package cache

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestTwoLevelClientSetRestoresExpiredEntry(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	external := NewRedisCache(redisClient, time.Hour, RedisCompressionGZip)
	client := NewTwoLevelClient(external, 10*time.Minute)
	item := &Item{Key: "managed-resources", Object: "unchanged", CacheActionOpts: CacheActionOpts{Expiration: time.Hour}}

	require.NoError(t, external.Set(item))
	server.FastForward(time.Hour - time.Minute)
	var value string
	require.NoError(t, client.Get(item.Key, &value))
	server.FastForward(time.Minute)
	require.ErrorIs(t, external.Get(item.Key, &value), ErrCacheMiss)

	require.NoError(t, client.Set(item))
	require.NoError(t, external.Get(item.Key, &value))
	require.Equal(t, "unchanged", value)
}

func TestTwoLevelClientSetRetriesFailedWrite(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = redisClient.Close() })
	external := NewRedisCache(redisClient, time.Hour, RedisCompressionGZip)
	client := NewTwoLevelClient(external, 10*time.Minute)
	item := &Item{Key: "managed-resources", Object: "unchanged", CacheActionOpts: CacheActionOpts{Expiration: time.Hour}}

	var value string
	server.SetError("ERR write failed")
	require.Error(t, client.Set(item))
	require.Error(t, client.Get(item.Key, &value))
	server.SetError("")

	require.NoError(t, client.Set(item))
	require.NoError(t, external.Get(item.Key, &value))
	require.Equal(t, "unchanged", value)
}
