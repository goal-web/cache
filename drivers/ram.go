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
	DataNotExistsErr = errors.New("data does not exist")
)

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

type data struct {
	value     any
	expiredAt time.Time
	forever   bool
	createdAt time.Time
	size      int64 // 数据大小（字节）
}

type memoryStats struct {
	hits      int64 // 命中次数
	misses    int64 // 未命中次数
	evictions int64 // 过期清理次数
	totalSize int64 // 总内存使用量
	keyCount  int64 // 当前键数量
}

type Memory struct {
	mutex     sync.RWMutex
	data      map[string]data
	ttl       time.Duration
	prefix    string
	createdAt time.Time
	stats     *memoryStats
}

func (ram *Memory) Get(key string) any {
	ram.mutex.RLock()
	item, ok := ram.data[key]
	ram.mutex.RUnlock()

	if !ok {
		atomic.AddInt64(&ram.stats.misses, 1)
		return nil
	}

	// 检查是否过期
	if !item.forever && time.Now().After(item.expiredAt) {
		// 异步清理过期项
		go ram.cleanupExpired(key)
		atomic.AddInt64(&ram.stats.misses, 1)
		return nil
	}

	atomic.AddInt64(&ram.stats.hits, 1)
	return item.value
}

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
				// 异步清理过期项
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

func (ram *Memory) Put(key string, value any, seconds time.Duration) error {
	if seconds <= 0 {
		return fmt.Errorf("expiration time must be positive, got %v", seconds)
	}

	// 计算数据大小（简化估算）
	size := ram.estimateSize(value)

	ram.mutex.Lock()
	defer ram.mutex.Unlock()

	// 如果键已存在，先减去旧值的大小
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

func (ram *Memory) Add(key string, value any, ttl ...time.Duration) bool {
	lifetime := time.Hour
	if len(ttl) > 0 && ttl[0] > 0 {
		lifetime = ttl[0]
	}

	ram.mutex.Lock()
	defer ram.mutex.Unlock()

	// 检查键是否存在且未过期
	if item, exists := ram.data[key]; exists {
		if item.forever || time.Now().Before(item.expiredAt) {
			return false
		}
	}

	// 设置新值
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

func (ram *Memory) Pull(key string, defaultValue ...any) any {
	ram.mutex.Lock()
	defer ram.mutex.Unlock()

	item, exists := ram.data[key]
	if !exists {
		return utils.DefaultInterface(defaultValue)
	}

	// 检查是否过期
	if !item.forever && time.Now().After(item.expiredAt) {
		delete(ram.data, key)
		atomic.AddInt64(&ram.stats.keyCount, -1)
		atomic.AddInt64(&ram.stats.totalSize, -item.size)
		return utils.DefaultInterface(defaultValue)
	}

	// 删除并返回
	delete(ram.data, key)
	atomic.AddInt64(&ram.stats.keyCount, -1)
	atomic.AddInt64(&ram.stats.totalSize, -item.size)

	atomic.AddInt64(&ram.stats.hits, 1)
	return item.value
}

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

		// 如果键已存在，先减去旧值的大小
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

func (ram *Memory) Increment(key string, value ...int64) (int64, error) {
	increment := int64(1)
	if len(value) > 0 {
		increment = value[0]
	}

	ram.mutex.Lock()
	defer ram.mutex.Unlock()

	item, exists := ram.data[key]
	if !exists {
		// 创建新计数器
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

	// 检查是否过期
	if !item.forever && time.Now().After(item.expiredAt) {
		item.value = increment
		item.expiredAt = time.Now().Add(ram.ttl)
		item.createdAt = time.Now()
	} else {
		// 递增现有值
		currentValue := utils.ToInt64(item.value, 0)
		item.value = currentValue + increment
		item.expiredAt = time.Now().Add(ram.ttl)
	}

	ram.data[key] = item
	return utils.ToInt64(item.value, 0), nil
}

func (ram *Memory) Decrement(key string, value ...int64) (int64, error) {
	decrement := int64(1)
	if len(value) > 0 {
		decrement = value[0]
	}

	ram.mutex.Lock()
	defer ram.mutex.Unlock()

	item, exists := ram.data[key]
	if !exists {
		// 创建新计数器
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

	// 检查是否过期
	if !item.forever && time.Now().After(item.expiredAt) {
		item.value = -decrement
		item.expiredAt = time.Now().Add(ram.ttl)
		item.createdAt = time.Now()
	} else {
		// 递减现有值
		currentValue := utils.ToInt64(item.value, 0)
		item.value = currentValue - decrement
		item.expiredAt = time.Now().Add(ram.ttl)
	}

	ram.data[key] = item
	return utils.ToInt64(item.value, 0), nil
}

func (ram *Memory) Forever(key string, value any) error {
	size := ram.estimateSize(value)

	ram.mutex.Lock()
	defer ram.mutex.Unlock()

	// 如果键已存在，先减去旧值的大小
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

func (ram *Memory) Flush() error {
	ram.mutex.Lock()
	defer ram.mutex.Unlock()

	ram.data = make(map[string]data)
	atomic.StoreInt64(&ram.stats.keyCount, 0)
	atomic.StoreInt64(&ram.stats.totalSize, 0)

	return nil
}

func (ram *Memory) GetPrefix() string {
	return ram.prefix
}

func (ram *Memory) Remember(key string, ttl time.Duration, provider contracts.InstanceProvider[any]) any {
	// 改进：参数验证
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

func (ram *Memory) RememberForever(key string, provider contracts.InstanceProvider[any]) any {
	// 改进：参数验证
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

// 新增方法：内存管理和统计

// GetStats 获取缓存统计信息
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

// GetHitRate 获取缓存命中率
func (ram *Memory) getHitRate() float64 {
	hits := atomic.LoadInt64(&ram.stats.hits)
	misses := atomic.LoadInt64(&ram.stats.misses)
	total := hits + misses

	if total == 0 {
		return 0.0
	}

	return float64(hits) / float64(total) * 100
}

// GetMemoryUsage 获取内存使用情况
func (ram *Memory) getMemoryUsage() map[string]any {
	return map[string]any{
		"total_bytes": atomic.LoadInt64(&ram.stats.totalSize),
		"total_mb":    float64(atomic.LoadInt64(&ram.stats.totalSize)) / 1024 / 1024,
		"key_count":   atomic.LoadInt64(&ram.stats.keyCount),
	}
}

// CleanupExpired 清理过期项
// 改进：异步清理，减少锁竞争
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

// CleanupAllExpired 清理所有过期项
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

// GetKeys 获取所有键（用于调试和监控）
func (ram *Memory) GetKeys() []string {
	ram.mutex.RLock()
	defer ram.mutex.RUnlock()

	keys := make([]string, 0, len(ram.data))
	for key := range ram.data {
		keys = append(keys, key)
	}

	return keys
}

// GetKeysByPattern 根据模式获取键列表（简单实现）
func (ram *Memory) GetKeysByPattern(pattern string) []string {
	// 简化实现，实际项目中可以使用正则表达式
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

// estimateSize 估算数据大小（简化实现）
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
		// 对于复杂类型，返回一个估算值
		return 64
	}
}
