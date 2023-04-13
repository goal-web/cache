package drivers

import (
	"github.com/goal-web/contracts"
	"time"
)

func NewRedisCache(redis contracts.RedisConnection, prefix string) contracts.CacheStore {
	return &RedisStore{
		redis:  redis,
		prefix: prefix,
	}
}

type RedisStore struct {
	redis  contracts.RedisConnection
	prefix string
}

func (store *RedisStore) Get(key string) any {
	result, _ := store.redis.Get(store.getKey(key))
	return result
}

func (store *RedisStore) Many(keys []string) []any {
	results, _ := store.redis.MGet(store.getKeys(keys)...)
	return results
}

func (store *RedisStore) Put(key string, value any, seconds time.Duration) error {
	_, err := store.redis.Set(store.getKey(key), value, seconds)
	return err
}

func (store *RedisStore) Add(key string, value any, ttls ...time.Duration) bool {
	var ttl time.Duration
	if len(ttls) > 0 {
		ttl = ttls[0]
	} else {
		ttl = time.Second * 60 * 60 // default 1 hour
	}
	result, _ := store.redis.SetNX(store.getKey(key), value, ttl)

	return result
}

func (store *RedisStore) Pull(key string, defaultValue ...any) any {
	key = store.getKey(key)
	result, err := store.redis.GetDel(key)

	if err != nil {
		result, err = store.redis.Get(key)
		if result != "" {
			_, _ = store.redis.Del(key)
		}
	}

	if (result == "" || err != nil) && len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return result
}

func (store *RedisStore) PutMany(values map[string]any, seconds time.Duration) error {
	data := make(map[string]any)
	for key, value := range values {
		data[store.getKey(key)] = value
	}
	_, err := store.redis.MSet(data)

	for key, _ := range data {
		_, _ = store.redis.Expire(key, seconds)
	}

	return err
}

func (store *RedisStore) Increment(key string, value ...int64) (int64, error) {
	key = store.getKey(key)
	if len(value) > 0 {
		return store.redis.IncrBy(key, value[0])
	}
	return store.redis.Incr(key)
}

func (store *RedisStore) Decrement(key string, value ...int64) (int64, error) {
	key = store.getKey(key)
	if len(value) > 0 {
		return store.redis.DecrBy(key, value[0])
	}
	return store.redis.Decr(key)
}

func (store *RedisStore) Forever(key string, value any) error {
	_, err := store.redis.Set(store.getKey(key), value, -1)
	return err
}

func (store *RedisStore) Forget(key string) error {
	_, err := store.redis.Del(store.getKey(key))
	return err
}

func (store *RedisStore) Flush() error {
	_, err := store.redis.FlushDB()
	return err
}

func (store *RedisStore) GetPrefix() string {
	return store.prefix
}

func (store *RedisStore) getKey(key string) string {
	return store.prefix + key
}

func (store *RedisStore) getKeys(keys []string) []string {
	for index, key := range keys {
		keys[index] = store.getKey(key)
	}
	return keys
}

func (store *RedisStore) Remember(key string, ttl time.Duration, provider contracts.InstanceProvider[any]) any {
	result := store.Get(key)
	if result == nil || result == "" {
		_ = store.Put(key, provider(), ttl)
	}
	return result
}

func (store *RedisStore) RememberForever(key string, provider contracts.InstanceProvider[any]) any {
	return store.Remember(key, -1, provider)
}
