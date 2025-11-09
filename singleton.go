// Package cache provides caching functionality and singleton access.
// 包 cache 提供缓存功能和单例访问。
package cache

import (
	"sync"
	"time"

	"github.com/goal-web/application"
	"github.com/goal-web/contracts"
)

// singleton holds the singleton instance of the cache factory.
// singleton 保存缓存工厂的单例实例。
var singleton contracts.CacheFactory

// once ensures the singleton is initialized only once.
// once 确保单例只被初始化一次。
var once sync.Once

// Default returns the singleton instance of the cache factory.
// Default 返回缓存工厂的单例实例。
func Default() contracts.CacheFactory {
	once.Do(func() {
		singleton = application.Get("cache").(contracts.CacheFactory)
	})

	return singleton
}

// Store returns a cache store instance with the given name.
// Store 返回指定名称的缓存存储实例。
func Store(name ...string) contracts.CacheStore {
	return Default().Store(name...)
}

// Extend registers a custom cache store provider.
// Extend 注册自定义缓存存储提供者。
func Extend(drive string, cacheStoreProvider contracts.CacheStoreProvider) {
	Default().Extend(drive, cacheStoreProvider)
}

// Increment increments the value of the given key by the given value.
// Increment 按给定值增加指定键的值。
func Increment(key string, value int64) (int64, error) {
	return Store().Increment(key, value)
}

// Decrement decrements the value of the given key by the given value.
// Decrement 按给定值减少指定键的值。
func Decrement(key string, value int64) (int64, error) {
	return Store().Decrement(key, value)
}

// CacheGet retrieves a value from the cache by key.
// CacheGet 根据键从缓存中获取值。
func CacheGet(key string) any {
	return Store().Get(key)
}

// Many retrieves multiple values from the cache by keys.
// Many 根据键列表从缓存中获取多个值。
func Many(keys []string) []any {
	return Store().Many(keys)
}

// Add adds a value to the cache with an expiration time if it doesn't exist.
// Add 如果不存在则将带过期时间的值添加到缓存中。
func Add(key string, value any, ttl time.Duration) bool {
	return Store().Add(key, value, ttl)
}

// Put puts a value to the cache with an expiration time.
// Put 将带过期时间的值放入缓存中。
func Put(key string, value any, ttl time.Duration) error {
	return Store().Put(key, value, ttl)
}

// Pull retrieves and removes a value from the cache by key.
// Pull 根据键从缓存中获取并删除值。
func Pull(key string, defaultValue ...any) any {
	return Store().Pull(key, defaultValue...)
}

// Remember retrieves a value from the cache or executes the callback to set it.
// Remember 从缓存中获取值，如果不存在则执行回调函数设置值。
func Remember(key string, ttl time.Duration, callback func() any) any {
	return Store().Remember(key, ttl, callback)
}

// RememberForever retrieves a value from the cache or executes the callback to set it without expiration.
// RememberForever 从缓存中获取值，如果不存在则执行回调函数设置值，无过期时间。
func RememberForever(key string, callback func() any) any {
	return Store().RememberForever(key, callback)
}

// Forget removes a value from the cache by key.
// Forget 根据键从缓存中删除值。
func Forget(key string) error {
	return Store().Forget(key)
}

// Forever puts a value to the cache without expiration time.
// Forever 将无过期时间的值放入缓存中。
func Forever(key string, value any) error {
	return Store().Forever(key, value)
}

// PutMany puts multiple values to the cache with an expiration time.
// PutMany 将多个带过期时间的值放入缓存中。
func PutMany(values map[string]any, seconds time.Duration) error {
	return Store().PutMany(values, seconds)
}

// Flush flushes all values from the cache.
// Flush 清空缓存中的所有值。
func Flush() error {
	return Store().Flush()
}

// GetPrefix returns the cache key prefix.
// GetPrefix 返回缓存键前缀。
func GetPrefix() string {
	return Store().GetPrefix()
}
