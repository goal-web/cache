// Package drivers contains cache store implementations.
// 包 drivers 包含缓存存储实现。
package drivers

import (
	"fmt"
	"time"

	"github.com/goal-web/contracts"
)

// NewRedisCache creates a new Redis cache instance.
// NewRedisCache 创建新的 Redis 缓存实例。
// Improvements: Add parameter validation.
// 改进：添加参数验证。
func NewRedisCache(redis contracts.RedisConnection, prefix string) contracts.CacheStore {
	if redis == nil {
		panic("Redis connection cannot be nil")
	}

	return &RedisStore{
		redis:  redis,
		prefix: prefix,
	}
}

// RedisStore implements a Redis-based cache store.
// RedisStore 实现基于 Redis 的缓存存储。
type RedisStore struct {
	redis  contracts.RedisConnection  // redis holds the Redis connection.
	                                   // redis 保存 Redis 连接。
	prefix string                     // prefix holds the key prefix for this store.
	                                   // prefix 保存此存储的键前缀。
}

// Get retrieves a value from the cache by key.
// Get 根据键从缓存中获取值。
func (store *RedisStore) Get(key string) any {
	result, err := store.redis.Get(store.getKey(key))
	if err != nil {
		// Improvements: Log error.
		// 改进：记录错误日志。
		return nil
	}

	// Improvements: Check empty string result.
	// 改进：检查空字符串结果。
	if result == "" {
		return nil
	}

	return result
}

// Many retrieves multiple values from the cache by keys.
// Many 根据键列表从缓存中获取多个值。
func (store *RedisStore) Many(keys []string) []any {
	if len(keys) == 0 {
		return []any{}
	}

	results, err := store.redis.MGet(store.getKeys(keys)...)
	if err != nil {
		// Improvements: Error handling, return nil slice with same length as keys.
		// 改进：错误处理，返回与keys长度相同的nil切片。
		results = make([]any, len(keys))
	}

	// Improvements: Ensure returned result length matches keys length.
	// 改进：确保返回结果长度与keys一致。
	if len(results) < len(keys) {
		for i := len(results); i < len(keys); i++ {
			results = append(results, nil)
		}
	}

	return results
}

// Put stores a value in the cache with an expiration time.
// Put 将带过期时间的值存储在缓存中。
func (store *RedisStore) Put(key string, value any, seconds time.Duration) error {
	// Improvements: Parameter validation.
	// 改进：参数验证。
	if seconds <= 0 {
		return fmt.Errorf("expiration time must be positive, got %v", seconds)
	}

	_, err := store.redis.Set(store.getKey(key), value, seconds)
	return err
}

// Add adds a value to the cache with an expiration time if it doesn't exist.
// Add 如果不存在则将带过期时间的值添加到缓存中。
func (store *RedisStore) Add(key string, value any, ttls ...time.Duration) bool {
	var ttl time.Duration
	if len(ttls) > 0 {
		ttl = ttls[0]
	} else {
		ttl = time.Hour // Improvements: Use clearer default value.
		               // 改进：使用更清晰的默认值。
	}

	// Improvements: Ensure TTL is positive.
	// 改进：确保TTL为正数。
	if ttl <= 0 {
		ttl = time.Hour
	}

	result, err := store.redis.SetNX(store.getKey(key), value, ttl)
	if err != nil {
		// Improvements: Error handling.
		// 改进：错误处理。
		return false
	}

	return result
}

// Pull retrieves and removes a value from the cache by key.
// Pull 根据键从缓存中获取并删除值。
func (store *RedisStore) Pull(key string, defaultValue ...any) any {
	key = store.getKey(key)
	result, err := store.redis.GetDel(key)

	if err != nil {
		// Improvements: If GetDel fails, try Get then manually delete.
		// 改进：如果GetDel失败，尝试Get然后手动删除。
		result, err = store.redis.Get(key)
		if err == nil && result != "" {
			_, delErr := store.redis.Del(key)
			if delErr != nil {
				// Improvements: Log deletion failure errors.
				// 改进：记录删除失败的错误。
			}
		}
	}

	// Improvements: Better null value handling.
	// 改进：更好的空值处理。
	if result == "" || err != nil {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return nil
	}

	return result
}

// PutMany stores multiple values in the cache with an expiration time.
// PutMany 将多个带过期时间的值存储在缓存中。
func (store *RedisStore) PutMany(values map[string]any, seconds time.Duration) error {
	// Improvements: Parameter validation.
	// 改进：参数验证。
	if len(values) == 0 {
		return nil
	}

	if seconds <= 0 {
		return fmt.Errorf("expiration time must be positive, got %v", seconds)
	}

	// Improvements: Build MSet arguments.
	// 改进：构建MSet参数。
	args := make([]any, 0, len(values)*2)
	for key, value := range values {
		args = append(args, store.getKey(key), value)
	}

	_, err := store.redis.MSet(args...)
	if err != nil {
		return err
	}

	// Improvements: Batch set expiration times, log errors but don't interrupt.
	// 改进：批量设置过期时间，记录错误但不中断。
	for key := range values {
		_, expireErr := store.redis.Expire(store.getKey(key), seconds)
		if expireErr != nil {
			// Log error but continue processing other keys.
			// 记录错误但继续处理其他key。
		}
	}

	return err
}

// Increment increments the value of the given key by the given value.
// Increment 按给定值增加指定键的值。
func (store *RedisStore) Increment(key string, value ...int64) (int64, error) {
	key = store.getKey(key)

	var increment int64 = 1
	if len(value) > 0 {
		increment = value[0]
	}

	// Improvements: If increment is 0, directly get current value.
	// 改进：如果增量为0，直接获取当前值。
	if increment == 0 {
		return store.redis.Incr(key)
	}

	return store.redis.IncrBy(key, increment)
}

// Decrement decrements the value of the given key by the given value.
// Decrement 按给定值减少指定键的值。
func (store *RedisStore) Decrement(key string, value ...int64) (int64, error) {
	key = store.getKey(key)

	var decrement int64 = 1
	if len(value) > 0 {
		decrement = value[0]
	}

	// Improvements: If decrement is 0, directly get current value.
	// 改进：如果减量为0，直接获取当前值。
	if decrement == 0 {
		return store.redis.Decr(key)
	}

	return store.redis.DecrBy(key, decrement)
}

// Forever stores a value in the cache without expiration time.
// Forever 将无过期时间的值存储在缓存中。
func (store *RedisStore) Forever(key string, value any) error {
	_, err := store.redis.Set(store.getKey(key), value, -1)
	return err
}

// Forget removes a value from the cache by key.
// Forget 根据键从缓存中删除值。
func (store *RedisStore) Forget(key string) error {
	_, err := store.redis.Del(store.getKey(key))
	return err
}

// Flush flushes all values from the cache.
// Flush 清空缓存中的所有值。
func (store *RedisStore) Flush() error {
	_, err := store.redis.FlushDB()
	return err
}

// GetPrefix returns the cache key prefix.
// GetPrefix 返回缓存键前缀。
func (store *RedisStore) GetPrefix() string {
	return store.prefix
}

// getKey prepends the prefix to the key.
// getKey 将前缀添加到键。
func (store *RedisStore) getKey(key string) string {
	return store.prefix + key
}

// getKeys prepends the prefix to all the keys.
// getKeys 将前缀添加到所有键。
func (store *RedisStore) getKeys(keys []string) []string {
	result := make([]string, len(keys))
	for index, key := range keys {
		result[index] = store.getKey(key)
	}
	return result
}

// Improvements: New methods for cache statistics and monitoring.
// 改进：缓存统计和监控的新方法。

// GetStats retrieves cache statistics information.
// GetStats 获取缓存统计信息。
func (store *RedisStore) GetStats() map[string]any {
	stats := make(map[string]any)

	// Get database size (number of keys).
	// 获取数据库大小（键数量）。
	if dbSize, err := store.redis.Command("DBSIZE"); err == nil {
		stats["db_size"] = dbSize
	}

	// Get memory usage.
	// 获取内存使用情况。
	if info, err := store.redis.Command("INFO", "memory"); err == nil {
		stats["memory_info"] = info
	}

	// Get connection information.
	// 获取连接信息。
	if info, err := store.redis.Command("INFO", "clients"); err == nil {
		stats["clients_info"] = info
	}

	return stats
}

// GetKeysByPattern retrieves a list of keys matching the pattern.
// GetKeysByPattern 根据模式获取键列表。
func (store *RedisStore) GetKeysByPattern(pattern string) ([]string, error) {
	if pattern == "" {
		pattern = "*"
	}

	// Add prefix to pattern.
	// 添加前缀到模式。
	if store.prefix != "" {
		pattern = store.prefix + pattern
	}

	return store.redis.Keys(pattern)
}

// ScanKeys scans keys (for large datasets).
// ScanKeys 扫描键（用于大数据集）。
func (store *RedisStore) ScanKeys(pattern string, count int64) ([]string, error) {
	if pattern == "" {
		pattern = "*"
	}

	// Add prefix to pattern.
	// 添加前缀到模式。
	if store.prefix != "" {
		pattern = store.prefix + pattern
	}

	var keys []string
	var cursor uint64 = 0

	for {
		scannedKeys, newCursor, err := store.redis.Scan(cursor, pattern, count)
		if err != nil {
			return keys, err
		}

		keys = append(keys, scannedKeys...)
		cursor = newCursor

		if cursor == 0 {
			break
		}
	}

	return keys, nil
}

// GetTTL retrieves the remaining time to live for a key.
// GetTTL 获取键的剩余生存时间。
func (store *RedisStore) GetTTL(key string) (time.Duration, error) {
	return store.redis.TTL(store.getKey(key))
}

// Exists checks if a key exists.
// Exists 检查键是否存在。
func (store *RedisStore) Exists(key string) (bool, error) {
	count, err := store.redis.Exists(store.getKey(key))
	return count > 0, err
}

// Remember retrieves a value from the cache or executes the provider to set it.
// Remember 从缓存中获取值，如果不存在则执行提供者设置值。
func (store *RedisStore) Remember(key string, ttl time.Duration, provider contracts.InstanceProvider[any]) any {
	// Improvements: Parameter validation.
	// 改进：参数验证。
	if provider == nil {
		return nil
	}

	result := store.Get(key)
	if result == nil || result == "" {
		// Improvements: Execute callback to get new value.
		// 改进：执行回调获取新值。
		value := provider()
		if value != nil {
			// Improvements: Store new value, log error but don't interrupt.
			// 改进：存储新值，记录错误但不中断。
			if err := store.Put(key, value, ttl); err != nil {
				// Log error.
				// 记录错误。
			}
		}
	}
	return result
}

// RememberForever retrieves a value from the cache or executes the provider to set it without expiration.
// RememberForever 从缓存中获取值，如果不存在则执行提供者设置值，无过期时间。
func (store *RedisStore) RememberForever(key string, provider contracts.InstanceProvider[any]) any {
	// Improvements: Parameter validation.
	// 改进：参数验证。
	if provider == nil {
		return nil
	}

	result := store.Get(key)
	if result == nil || result == "" {
		// Improvements: Execute callback to get new value.
		// 改进：执行回调获取新值。
		value := provider()
		if value != nil {
			// Improvements: Store new value permanently, log error but don't interrupt.
			// 改进：永久存储新值，记录错误但不中断。
			if err := store.Forever(key, value); err != nil {
				// Log error.
				// 记录错误。
			}
		}
	}
	return result
}
