package cache

import (
	"sync"
	"time"

	"github.com/goal-web/application"
	"github.com/goal-web/contracts"
)

var singleton contracts.CacheFactory
var once sync.Once

func Default() contracts.CacheFactory {
	once.Do(func() {
		singleton = application.Get("cache").(contracts.CacheFactory)
	})

	return singleton
}

func Store(name ...string) contracts.CacheStore {
	return Default().Store(name...)
}

func Extend(drive string, cacheStoreProvider contracts.CacheStoreProvider) {
	Default().Extend(drive, cacheStoreProvider)
}

func Increment(key string, value int64) (int64, error) {
	return Store().Increment(key, value)
}

func Decrement(key string, value int64) (int64, error) {
	return Store().Decrement(key, value)
}

func CacheGet(key string) any {
	return Store().Get(key)
}

func Many(keys []string) []any {
	return Store().Many(keys)
}

func Add(key string, value any, ttl time.Duration) bool {
	return Store().Add(key, value, ttl)
}

func Put(key string, value any, ttl time.Duration) error {
	return Store().Put(key, value, ttl)
}

func Pull(key string, defaultValue ...any) any {
	return Store().Pull(key, defaultValue...)
}

func Remember(key string, ttl time.Duration, callback func() any) any {
	return Store().Remember(key, ttl, callback)
}

func RememberForever(key string, callback func() any) any {
	return Store().RememberForever(key, callback)
}

func Forget(key string) error {
	return Store().Forget(key)
}

func Forever(key string, value any) error {
	return Store().Forever(key, value)
}

func PutMany(values map[string]any, seconds time.Duration) error {
	return Store().PutMany(values, seconds)
}

func Flush() error {
	return Store().Flush()
}

func GetPrefix() string {
	return Store().GetPrefix()
}
