// Package drivers contains cache store implementations.
// 包 drivers 包含缓存存储实现。
package drivers

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goal-web/contracts"
	"github.com/goal-web/supports/logs"
	"github.com/goal-web/supports/utils"
)

var (
	// DataNotExistsErr represents an error when data does not exist.
	// DataNotExistsErr 表示数据不存在时的错误。
	DataNotExistsErr = errors.New("data does not exist")
)

// NewMemory creates a new in-memory cache instance.
// NewMemory 创建新的内存缓存实例。
func NewMemory(config contracts.Fields) contracts.CacheStore {
	ttl := utils.GetIntField(config, "ttl", 24*int(time.Hour))
	if ttl <= 0 {
		ttl = 24 * int(time.Hour) // 默认24小时
	}

	prefix := utils.GetStringField(config, "prefix")

	return &Memory{
		mutex:     sync.RWMutex{},
		data:      make(map[string]data),
		ttl:       time.Duration(ttl),
		prefix:    prefix,
		createdAt: time.Now(),
		stats:     &memoryStats{},
	}
}

// data represents cached data with expiration information.
// data 表示带过期信息的缓存数据。
type data struct {
	value     any       // value holds the cached value.
	                     // value 保存缓存值。
	expiredAt time.Time // expiredAt holds the expiration time.
	                     // expiredAt 保存过期时间。
	forever   bool      // forever indicates if the value never expires.
	                     // forever 指示值是否永不过期。
	createdAt time.Time // createdAt holds the creation time.
	                     // createdAt 保存创建时间。
	size      int64     // size holds the data size in bytes.
	                     // size 保存数据大小（字节）。
}

// memoryStats holds memory cache statistics.
// memoryStats 保存内存缓存统计信息。
type memoryStats struct {
	hits      int64 // hits holds the number of cache hits.
	                     // hits 保存缓存命中次数。
	misses    int64 // misses holds the number of cache misses.
	                     // misses 保存缓存未命中次数。
	evictions int64 // evictions holds the number of expired items cleaned up.
	                     // evictions 保存过期清理次数。
	totalSize int64 // totalSize holds the total memory usage in bytes.
	                     // totalSize 保存总内存使用量。
	keyCount  int64 // keyCount holds the current number of keys.
	                     // keyCount 保存当前键数量。
}

// Memory implements an in-memory cache store.
// Memory 实现内存缓存存储。
type Memory struct {
	mutex     sync.RWMutex    // mutex provides thread-safe access to the memory cache.
	                         // mutex 提供对内存缓存的线程安全访问。
	data      map[string]data // data holds the cached data.
	                         // data 保存缓存数据。
	ttl       time.Duration   // ttl holds the default time-to-live for cached items.
	                         // ttl 保存缓存项的默认生存时间。
	prefix    string          // prefix holds the key prefix for this store.
	                         // prefix 保存此存储的键前缀。
	createdAt time.Time       // createdAt holds the time when the cache was created.
	                         // createdAt 保存缓存创建时间。
	stats     *memoryStats    // stats holds cache statistics.
	                         // stats 保存缓存统计信息。
}

// Get retrieves a value from the cache by key.
// Get 根据键从缓存中获取值。
func (ram *Memory) Get(key string) any {
	ram.mutex.RLock()
	item, ok := ram.data[key]
	ram.mutex.RUnlock()

	if !ok {
		atomic.AddInt64(&ram.stats.misses, 1)
		return nil
	}

	// Check if expired.
	// 检查是否过期。
	if !item.forever && time.Now().After(item.expiredAt) {
		// Cleanup expired item asynchronously.
		// 异步清理过期项。
		go ram.cleanupExpired(key)
		atomic.AddInt64(&ram.stats.misses, 1)
		return nil
	}

	atomic.AddInt64(&ram.stats.hits, 1)
	return item.value
}

// Many retrieves multiple values from the cache by keys.
// Many 根据键列表从缓存中获取多个值。
func (ram *Memory) Many(keys []string) []any {
	if len(keys) == 0 {
		return []any{}
	}

	results := make([]any, len(keys))
	now := time.Now()

	ram.mutex.RLock()
	for i, key := range keys {
		if item, ok := ram.data[key]; ok {
			if item.forever || now.Before(item.expiredAt) {
				results[i] = item.value
				atomic.AddInt64(&ram.stats.hits, 1)
			} else {
				// Cleanup expired item asynchronously.
				// 异步清理过期项。
				go ram.cleanupExpired(key)
				atomic.AddInt64(&ram.stats.misses, 1)
			}
		} else {
			atomic.AddInt64(&ram.stats.misses, 1)
		}
	}
	ram.mutex.RUnlock()

	return results
}

// Put stores a value in the cache with an expiration time.
// Put 将带过期时间的值存储在缓存中。
func (ram *Memory) Put(key string, value any, seconds time.Duration) error {
	if seconds <= 0 {
		return fmt.Errorf("expiration time must be positive, got %v", seconds)
	}

	// Calculate data size (simplified estimation).
	// 计算数据大小（简化估算）。
	size := ram.estimateSize(value)

	ram.mutex.Lock()
	defer ram.mutex.Unlock()

	// If key already exists, subtract old value size first.
	// 如果键已存在，先减去旧值的大小。
	if oldItem, exists := ram.data[key]; exists {
		atomic.AddInt64(&ram.stats.totalSize, -oldItem.size)
	} else {
		atomic.AddInt64(&ram.stats.keyCount, 1)
	}

	ram.data[key] = data{
		value:     value,
		expiredAt: time.Now().Add(seconds),
		forever:   false,
		createdAt: time.Now(),
		size:      size,
	}

	atomic.AddInt64(&ram.stats.totalSize, size)
	return nil
}

// Add adds a value to the cache with an expiration time if it doesn't exist.
// Add 如果不存在则将带过期时间的值添加到缓存中。
func (ram *Memory) Add(key string, value any, ttl ...time.Duration) bool {
	lifetime := time.Hour
	if len(ttl) > 0 && ttl[0] > 0 {
		lifetime = ttl[0]
	}

	ram.mutex.Lock()
	defer ram.mutex.Unlock()

	// Check if key exists and is not expired.
	// 检查键是否存在且未过期。
	if item, exists := ram.data[key]; exists {
		if item.forever || time.Now().Before(item.expiredAt) {
			return false
		}
	}

	// Set new value.
	// 设置新值。
	size := ram.estimateSize(value)
	ram.data[key] = data{
		value:     value,
		expiredAt: time.Now().Add(lifetime),
		forever:   false,
		createdAt: time.Now(),
		size:      size,
	}

	atomic.AddInt64(&ram.stats.keyCount, 1)
	atomic.AddInt64(&ram.stats.totalSize, size)
	return true
}

// Pull retrieves and removes a value from the cache by key.
// Pull 根据键从缓存中获取并删除值。
func (ram *Memory) Pull(key string, defaultValue ...any) any {
	ram.mutex.Lock()
	defer ram.mutex.Unlock()

	item, exists := ram.data[key]
	if !exists {
		return utils.DefaultInterface(defaultValue)
	}

	// Check if expired.
	// 检查是否过期。
	if !item.forever && time.Now().After(item.expiredAt) {
		delete(ram.data, key)
		atomic.AddInt64(&ram.stats.keyCount, -1)
		atomic.AddInt64(&ram.stats.totalSize, -item.size)
		return utils.DefaultInterface(defaultValue)
	}

	// Delete and return.
	// 删除并返回。
	delete(ram.data, key)
	atomic.AddInt64(&ram.stats.keyCount, -1)
	atomic.AddInt64(&ram.stats.totalSize, -item.size)

	atomic.AddInt64(&ram.stats.hits, 1)
	return item.value
}

// PutMany stores multiple values in the cache with an expiration time.
// PutMany 将多个带过期时间的值存储在缓存中。
func (ram *Memory) PutMany(values map[string]any, seconds time.Duration) error {
	if len(values) == 0 {
		return nil
	}

	if seconds <= 0 {
		return fmt.Errorf("expiration time must be positive, got %v", seconds)
	}

	now := time.Now()
	totalSize := int64(0)

	ram.mutex.Lock()
	defer ram.mutex.Unlock()

	for key, value := range values {
		size := ram.estimateSize(value)

		// If key already exists, subtract old value size first.
		// 如果键已存在，先减去旧值的大小。
		if oldItem, exists := ram.data[key]; exists {
			atomic.AddInt64(&ram.stats.totalSize, -oldItem.size)
		} else {
			atomic.AddInt64(&ram.stats.keyCount, 1)
		}

		ram.data[key] = data{
			value:     value,
			expiredAt: now.Add(seconds),
			forever:   false,
			createdAt: now,
			size:      size,
		}

		totalSize += size
	}

	atomic.AddInt64(&ram.stats.totalSize, totalSize)
	return nil
}

// Increment increments the value of the given key by the given value.
// Increment 按给定值增加指定键的值。
func (ram *Memory) Increment(key string, value ...int64) (int64, error) {
	increment := int64(1)
	if len(value) > 0 {
		increment = value[0]
	}

	ram.mutex.Lock()
	defer ram.mutex.Unlock()

	item, exists := ram.data[key]
	if !exists {
		// Create new counter.
		// 创建新计数器。
		newItem := data{
			value:     increment,
			expiredAt: time.Now().Add(ram.ttl),
			forever:   false,
			createdAt: time.Now(),
			size:      8, // int64的大小
		}
		ram.data[key] = newItem
		atomic.AddInt64(&ram.stats.keyCount, 1)
		atomic.AddInt64(&ram.stats.totalSize, 8)
		return increment, nil
	}

	// Check if expired.
	// 检查是否过期。
	if !item.forever && time.Now().After(item.expiredAt) {
		item.value = increment
		item.expiredAt = time.Now().Add(ram.ttl)
		item.createdAt = time.Now()
	} else {
		// Increment existing value.
		// 递增现有值。
		currentValue := utils.ToInt64(item.value, 0)
		item.value = currentValue + increment
		item.expiredAt = time.Now().Add(ram.ttl)
	}

	ram.data[key] = item
	return utils.ToInt64(item.value, 0), nil
}

// Decrement decrements the value of the given key by the given value.
// Decrement 按给定值减少指定键的值。
func (ram *Memory) Decrement(key string, value ...int64) (int64, error) {
	decrement := int64(1)
	if len(value) > 0 {
		decrement = value[0]
	}

	ram.mutex.Lock()
	defer ram.mutex.Unlock()

	item, exists := ram.data[key]
	if !exists {
		// Create new counter.
		// 创建新计数器。
		newItem := data{
			value:     -decrement,
			expiredAt: time.Now().Add(ram.ttl),
			forever:   false,
			createdAt: time.Now(),
			size:      8, // int64的大小
		}
		ram.data[key] = newItem
		atomic.AddInt64(&ram.stats.keyCount, 1)
		atomic.AddInt64(&ram.stats.totalSize, 8)
		return -decrement, nil
	}

	// Check if expired.
	// 检查是否过期。
	if !item.forever && time.Now().After(item.expiredAt) {
		item.value = -decrement
		item.expiredAt = time.Now().Add(ram.ttl)
		item.createdAt = time.Now()
	} else {
		// Decrement existing value.
		// 递减现有值。
		currentValue := utils.ToInt64(item.value, 0)
		item.value = currentValue - decrement
		item.expiredAt = time.Now().Add(ram.ttl)
	}

	ram.data[key] = item
	return utils.ToInt64(item.value, 0), nil
}

// Forever stores a value in the cache without expiration time.
// Forever 将无过期时间的值存储在缓存中。
func (ram *Memory) Forever(key string, value any) error {
	size := ram.estimateSize(value)

	ram.mutex.Lock()
	defer ram.mutex.Unlock()

	// If key already exists, subtract old value size first.
	// 如果键已存在，先减去旧值的大小。
	if oldItem, exists := ram.data[key]; exists {
		atomic.AddInt64(&ram.stats.totalSize, -oldItem.size)
	} else {
		atomic.AddInt64(&ram.stats.keyCount, 1)
	}

	ram.data[key] = data{
		value:     value,
		forever:   true,
		createdAt: time.Now(),
		size:      size,
	}

	atomic.AddInt64(&ram.stats.totalSize, size)
	return nil
}

// Forget removes a value from the cache by key.
// Forget 根据键从缓存中删除值。
func (ram *Memory) Forget(key string) error {
	ram.mutex.Lock()
	defer ram.mutex.Unlock()

	item, exists := ram.data[key]
	if !exists {
		return DataNotExistsErr
	}

	delete(ram.data, key)
	atomic.AddInt64(&ram.stats.keyCount, -1)
	atomic.AddInt64(&ram.stats.totalSize, -item.size)

	return nil
}

// Flush flushes all values from the cache.
// Flush 清空缓存中的所有值。
func (ram *Memory) Flush() error {
	ram.mutex.Lock()
	defer ram.mutex.Unlock()

	ram.data = make(map[string]data)
	atomic.StoreInt64(&ram.stats.keyCount, 0)
	atomic.StoreInt64(&ram.stats.totalSize, 0)

	return nil
}

// GetPrefix returns the cache key prefix.
// GetPrefix 返回缓存键前缀。
func (ram *Memory) GetPrefix() string {
	return ram.prefix
}

// Remember retrieves a value from the cache or executes the provider to set it.
// Remember 从缓存中获取值，如果不存在则执行提供者设置值。
func (ram *Memory) Remember(key string, ttl time.Duration, provider contracts.InstanceProvider[any]) any {
	// Improvement: Parameter validation.
	// 改进：参数验证。
	if provider == nil {
		return nil
	}

	if value := ram.Get(key); value != nil {
		return value
	}

	value := provider()
	if value != nil {
		if err := ram.Put(key, value, ttl); err != nil {
			logs.WithError(err).WithField("value", value).Debug("cache.Memory.Remember: value put failed")
		}
	}

	return value
}

// RememberForever retrieves a value from the cache or executes the provider to set it without expiration.
// RememberForever 从缓存中获取值，如果不存在则执行提供者设置值，无过期时间。
func (ram *Memory) RememberForever(key string, provider contracts.InstanceProvider[any]) any {
	// Improvement: Parameter validation.
	// 改进：参数验证。
	if provider == nil {
		return nil
	}

	if value := ram.Get(key); value != nil {
		return value
	}

	value := provider()
	if value != nil {
		if err := ram.Forever(key, value); err != nil {
			logs.WithError(err).WithField("value", value).Debug("cache.Memory.RememberForever: value put failed")
		}
	}

	return value
}

// New methods: Memory management and statistics.
// 新增方法：内存管理和统计。

// GetStats retrieves cache statistics information.
// GetStats 获取缓存统计信息。
func (ram *Memory) GetStats() map[string]any {
	return map[string]any{
		"hits":         atomic.LoadInt64(&ram.stats.hits),
		"misses":       atomic.LoadInt64(&ram.stats.misses),
		"evictions":    atomic.LoadInt64(&ram.stats.evictions),
		"total_size":   atomic.LoadInt64(&ram.stats.totalSize),
		"key_count":    atomic.LoadInt64(&ram.stats.keyCount),
		"hit_rate":     ram.getHitRate(),
		"memory_usage": ram.getMemoryUsage(),
		"uptime":       time.Since(ram.createdAt).String(),
	}
}

// getHitRate retrieves the cache hit rate.
// getHitRate 获取缓存命中率。
func (ram *Memory) getHitRate() float64 {
	hits := atomic.LoadInt64(&ram.stats.hits)
	misses := atomic.LoadInt64(&ram.stats.misses)
	total := hits + misses

	if total == 0 {
		return 0.0
	}

	return float64(hits) / float64(total) * 100
}

// getMemoryUsage retrieves memory usage information.
// getMemoryUsage 获取内存使用情况。
func (ram *Memory) getMemoryUsage() map[string]any {
	return map[string]any{
		"total_bytes": atomic.LoadInt64(&ram.stats.totalSize),
		"total_mb":    float64(atomic.LoadInt64(&ram.stats.totalSize)) / 1024 / 1024,
		"key_count":   atomic.LoadInt64(&ram.stats.keyCount),
	}
}

// cleanupExpired cleans up an expired item.
// cleanupExpired 清理过期项。
// Improvement: Asynchronous cleanup to reduce lock contention.
// 改进：异步清理，减少锁竞争。
func (ram *Memory) cleanupExpired(key string) {
	ram.mutex.Lock()
	defer ram.mutex.Unlock()

	if item, exists := ram.data[key]; exists {
		if !item.forever && time.Now().After(item.expiredAt) {
			delete(ram.data, key)
			atomic.AddInt64(&ram.stats.evictions, 1)
			atomic.AddInt64(&ram.stats.keyCount, -1)
			atomic.AddInt64(&ram.stats.totalSize, -item.size)
		}
	}
}

// CleanupAllExpired cleans up all expired items.
// CleanupAllExpired 清理所有过期项。
func (ram *Memory) CleanupAllExpired() int {
	ram.mutex.Lock()
	defer ram.mutex.Unlock()

	now := time.Now()
	cleaned := 0

	for key, item := range ram.data {
		if !item.forever && now.After(item.expiredAt) {
			delete(ram.data, key)
			atomic.AddInt64(&ram.stats.evictions, 1)
			atomic.AddInt64(&ram.stats.totalSize, -item.size)
			cleaned++
		}
	}

	atomic.AddInt64(&ram.stats.keyCount, -int64(cleaned))
	return cleaned
}

// GetKeys retrieves all keys (for debugging and monitoring).
// GetKeys 获取所有键（用于调试和监控）。
func (ram *Memory) GetKeys() []string {
	ram.mutex.RLock()
	defer ram.mutex.RUnlock()

	keys := make([]string, 0, len(ram.data))
	for key := range ram.data {
		keys = append(keys, key)
	}

	return keys
}

// GetKeysByPattern retrieves a list of keys matching the pattern (simple implementation).
// GetKeysByPattern 根据模式获取键列表（简单实现）。
func (ram *Memory) GetKeysByPattern(pattern string) []string {
	// Simplified implementation, can use regex in actual projects.
	// 简化实现，实际项目中可以使用正则表达式。
	keys := ram.GetKeys()
	if pattern == "*" || pattern == "" {
		return keys
	}

	var matched []string
	for _, key := range keys {
		if len(key) >= len(pattern) && key[:len(pattern)] == pattern {
			matched = append(matched, key)
		}
	}

	return matched
}

// estimateSize estimates data size (simplified implementation).
// estimateSize 估算数据大小（简化实现）。
func (ram *Memory) estimateSize(value any) int64 {
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return 8
	case float32, float64:
		return 8
	case bool:
		return 1
	default:
		// For complex types, return an estimated value.
		// 对于复杂类型，返回一个估算值。
		return 64
	}
}
