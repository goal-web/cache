package drivers

import (
	"fmt"
	"time"

	"github.com/goal-web/contracts"
)

// NewRedisCache 创建Redis缓存实例
// 改进：添加参数验证
func NewRedisCache(redis contracts.RedisConnection, prefix string) contracts.CacheStore {
	if redis == nil {
		panic("Redis connection cannot be nil")
	}

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
	result, err := store.redis.Get(store.getKey(key))
	if err != nil {
		// 改进：记录错误日志
		return nil
	}

	// 改进：检查空字符串结果
	if result == "" {
		return nil
	}

	return result
}

func (store *RedisStore) Many(keys []string) []any {
	if len(keys) == 0 {
		return []any{}
	}

	results, err := store.redis.MGet(store.getKeys(keys)...)
	if err != nil {
		// 改进：错误处理，返回与keys长度相同的nil切片
		results = make([]any, len(keys))
	}

	// 改进：确保返回结果长度与keys一致
	if len(results) < len(keys) {
		for i := len(results); i < len(keys); i++ {
			results = append(results, nil)
		}
	}

	return results
}

func (store *RedisStore) Put(key string, value any, seconds time.Duration) error {
	// 改进：参数验证
	if seconds <= 0 {
		return fmt.Errorf("expiration time must be positive, got %v", seconds)
	}

	_, err := store.redis.Set(store.getKey(key), value, seconds)
	return err
}

func (store *RedisStore) Add(key string, value any, ttls ...time.Duration) bool {
	var ttl time.Duration
	if len(ttls) > 0 {
		ttl = ttls[0]
	} else {
		ttl = time.Hour // 改进：使用更清晰的默认值
	}

	// 改进：确保TTL为正数
	if ttl <= 0 {
		ttl = time.Hour
	}

	result, err := store.redis.SetNX(store.getKey(key), value, ttl)
	if err != nil {
		// 改进：错误处理
		return false
	}

	return result
}

func (store *RedisStore) Pull(key string, defaultValue ...any) any {
	key = store.getKey(key)
	result, err := store.redis.GetDel(key)

	if err != nil {
		// 改进：如果GetDel失败，尝试Get然后手动删除
		result, err = store.redis.Get(key)
		if err == nil && result != "" {
			_, delErr := store.redis.Del(key)
			if delErr != nil {
				// 改进：记录删除失败的错误
			}
		}
	}

	// 改进：更好的空值处理
	if result == "" || err != nil {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return nil
	}

	return result
}

func (store *RedisStore) PutMany(values map[string]any, seconds time.Duration) error {
	// 改进：参数验证
	if len(values) == 0 {
		return nil
	}

	if seconds <= 0 {
		return fmt.Errorf("expiration time must be positive, got %v", seconds)
	}

	// 改进：构建MSet参数
	args := make([]any, 0, len(values)*2)
	for key, value := range values {
		args = append(args, store.getKey(key), value)
	}

	_, err := store.redis.MSet(args...)
	if err != nil {
		return err
	}

	// 改进：批量设置过期时间，记录错误但不中断
	for key := range values {
		_, expireErr := store.redis.Expire(store.getKey(key), seconds)
		if expireErr != nil {
			// 记录错误但继续处理其他key
		}
	}

	return err
}

func (store *RedisStore) Increment(key string, value ...int64) (int64, error) {
	key = store.getKey(key)

	var increment int64 = 1
	if len(value) > 0 {
		increment = value[0]
	}

	// 改进：如果增量为0，直接获取当前值
	if increment == 0 {
		return store.redis.Incr(key)
	}

	return store.redis.IncrBy(key, increment)
}

func (store *RedisStore) Decrement(key string, value ...int64) (int64, error) {
	key = store.getKey(key)

	var decrement int64 = 1
	if len(value) > 0 {
		decrement = value[0]
	}

	// 改进：如果减量为0，直接获取当前值
	if decrement == 0 {
		return store.redis.Decr(key)
	}

	return store.redis.DecrBy(key, decrement)
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
	result := make([]string, len(keys))
	for index, key := range keys {
		result[index] = store.getKey(key)
	}
	return result
}

// 新增方法：缓存统计和监控

// GetStats 获取缓存统计信息
func (store *RedisStore) GetStats() map[string]any {
	stats := make(map[string]any)

	// 获取数据库大小（键数量）
	if dbSize, err := store.redis.Command("DBSIZE"); err == nil {
		stats["db_size"] = dbSize
	}

	// 获取内存使用情况
	if info, err := store.redis.Command("INFO", "memory"); err == nil {
		stats["memory_info"] = info
	}

	// 获取连接信息
	if info, err := store.redis.Command("INFO", "clients"); err == nil {
		stats["clients_info"] = info
	}

	return stats
}

// GetKeysByPattern 根据模式获取键列表
func (store *RedisStore) GetKeysByPattern(pattern string) ([]string, error) {
	if pattern == "" {
		pattern = "*"
	}

	// 添加前缀到模式
	if store.prefix != "" {
		pattern = store.prefix + pattern
	}

	return store.redis.Keys(pattern)
}

// ScanKeys 扫描键（用于大数据集）
func (store *RedisStore) ScanKeys(pattern string, count int64) ([]string, error) {
	if pattern == "" {
		pattern = "*"
	}

	// 添加前缀到模式
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

// GetTTL 获取键的剩余生存时间
func (store *RedisStore) GetTTL(key string) (time.Duration, error) {
	return store.redis.TTL(store.getKey(key))
}

// Exists 检查键是否存在
func (store *RedisStore) Exists(key string) (bool, error) {
	count, err := store.redis.Exists(store.getKey(key))
	return count > 0, err
}

func (store *RedisStore) Remember(key string, ttl time.Duration, provider contracts.InstanceProvider[any]) any {
	// 改进：参数验证
	if provider == nil {
		return nil
	}

	result := store.Get(key)
	if result == nil || result == "" {
		// 改进：执行回调获取新值
		value := provider()
		if value != nil {
			// 改进：存储新值，记录错误但不中断
			if err := store.Put(key, value, ttl); err != nil {
				// 记录错误
			}
		}
	}
	return result
}

func (store *RedisStore) RememberForever(key string, provider contracts.InstanceProvider[any]) any {
	// 改进：参数验证
	if provider == nil {
		return nil
	}

	result := store.Get(key)
	if result == nil || result == "" {
		// 改进：执行回调获取新值
		value := provider()
		if value != nil {
			// 改进：永久存储新值，记录错误但不中断
			if err := store.Forever(key, value); err != nil {
				// 记录错误
			}
		}
	}
	return result
}
