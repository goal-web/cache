# RAM缓存驱动优化说明

## 概述

本文档描述了RAM缓存驱动的优化内容，包括性能改进、内存管理、统计功能等方面。

## 主要优化

### 1. 过期时间逻辑修复

#### 原有问题
- `time.Since(item.expiredAt) > 0` 逻辑错误，应该是 `time.Now().After(item.expiredAt)`
- 在读取锁期间进行写操作（删除过期项），违反锁的使用原则

#### 优化内容
- 修复过期时间比较逻辑
- 使用异步清理过期项，减少锁竞争
- 优化锁的使用策略

```go
// 优化前
if item.forever || time.Since(item.expiredAt) > 0 {
    return item.value
} else {
    delete(ram.data, key)  // 在读取锁期间写操作
    return nil
}

// 优化后
if !item.forever && time.Now().After(item.expiredAt) {
    // 异步清理过期项
    go ram.cleanupExpired(key)
    atomic.AddInt64(&ram.stats.misses, 1)
    return nil
}
```

### 2. 内存管理和统计

#### 新增功能
- **内存使用跟踪**: 跟踪每个缓存项的大小和总内存使用量
- **命中率统计**: 统计缓存命中次数和未命中次数
- **过期清理统计**: 统计自动清理的过期项数量
- **运行时间统计**: 跟踪缓存服务的运行时间

#### 数据结构改进
```go
type data struct {
    value     any
    expiredAt time.Time
    forever   bool
    createdAt time.Time    // 新增：创建时间
    size      int64        // 新增：数据大小（字节）
}

type memoryStats struct {
    hits        int64 // 命中次数
    misses      int64 // 未命中次数
    evictions   int64 // 过期清理次数
    totalSize   int64 // 总内存使用量
    keyCount    int64 // 当前键数量
}
```

### 3. 锁优化

#### 原有问题
- 某些方法在读取锁期间进行写操作
- 锁的粒度不够精细

#### 优化内容
- 分离读写操作，减少锁竞争
- 使用原子操作进行统计计数
- 异步清理过期项

```go
// 优化前：在读取锁期间写操作
func (ram *Memory) Get(key string) any {
    ram.mutex.RLock()
    defer ram.mutex.RUnlock()
    if item, ok := ram.data[key]; ok {
        if item.forever || time.Since(item.expiredAt) > 0 {
            return item.value
        } else {
            delete(ram.data, key)  // 在读取锁期间写操作
            return nil
        }
    }
    return nil
}

// 优化后：分离读写操作
func (ram *Memory) Get(key string) any {
    ram.mutex.RLock()
    item, ok := ram.data[key]
    ram.mutex.RUnlock()
    
    if !ok {
        atomic.AddInt64(&ram.stats.misses, 1)
        return nil
    }
    
    if !item.forever && time.Now().After(item.expiredAt) {
        go ram.cleanupExpired(key)  // 异步清理
        atomic.AddInt64(&ram.stats.misses, 1)
        return nil
    }
    
    atomic.AddInt64(&ram.stats.hits, 1)
    return item.value
}
```

### 4. 参数验证

#### 新增验证
- TTL参数验证：确保过期时间为正数
- 空值检查：避免存储nil值
- 边界条件处理

```go
func (ram *Memory) Put(key string, value any, seconds time.Duration) error {
    if seconds <= 0 {
        return fmt.Errorf("expiration time must be positive, got %v", seconds)
    }
    
    // 计算数据大小
    size := ram.estimateSize(value)
    
    // ... 其他逻辑
}
```

### 5. 批量操作优化

#### 原有问题
- `Many`方法可能返回长度不一致的结果
- 缺乏对空切片的处理

#### 优化内容
- 确保返回结果长度与请求的keys一致
- 优化空值处理
- 改进边界情况处理

```go
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
```

### 6. 新增实用方法

#### 内存管理
- `GetStats()`: 获取完整的缓存统计信息
- `GetHitRate()`: 计算缓存命中率
- `GetMemoryUsage()`: 获取内存使用情况
- `CleanupAllExpired()`: 手动清理所有过期项

#### 监控和调试
- `GetKeys()`: 获取所有缓存键
- `GetKeysByPattern()`: 根据模式获取键列表
- `estimateSize()`: 估算数据大小

## 性能改进

### 1. 锁竞争减少
- 读取操作使用更短的锁持有时间
- 异步清理过期项，减少主线程阻塞
- 使用原子操作进行统计计数

### 2. 内存使用优化
- 精确跟踪内存使用量
- 自动清理过期项，防止内存泄漏
- 支持手动清理过期项

### 3. 批量操作优化
- 减少锁的获取和释放次数
- 预分配结果切片，避免动态扩容
- 并行处理过期项清理

## 使用示例

### 1. 基本使用
```go
// 创建缓存实例
cache := NewMemory(contracts.Fields{
    "ttl":     int(time.Hour),
    "prefix":  "app_",
})

// 存储和获取
cache.Put("key", "value", time.Hour)
value := cache.Get("key")
```

### 2. 统计信息
```go
// 获取统计信息
stats := cache.GetStats()
fmt.Printf("命中率: %.2f%%\n", stats["hit_rate"])
fmt.Printf("内存使用: %.2f MB\n", stats["memory_usage"].(map[string]any)["total_mb"])
fmt.Printf("键数量: %d\n", stats["key_count"])
```

### 3. 内存管理
```go
// 清理过期项
cleaned := cache.CleanupAllExpired()
fmt.Printf("清理了 %d 个过期项\n", cleaned)

// 获取所有键
keys := cache.GetKeys()
fmt.Printf("当前有 %d 个键\n", len(keys))
```

### 4. 模式匹配
```go
// 获取特定模式的键
userKeys := cache.GetKeysByPattern("user:*")
fmt.Printf("找到 %d 个用户相关的键\n", len(userKeys))
```

## 配置选项

### 基本配置
```toml
[stores.memory]
driver = "ram"
ttl = 86400        # 默认过期时间（秒）
prefix = "app_"    # 键前缀
```

### 配置参数说明
| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `driver` | string | - | 驱动名称，必须为 "ram" |
| `ttl` | int | 86400 | 默认过期时间（秒） |
| `prefix` | string | "" | 缓存键前缀 |

## 性能对比

### 优化前后对比
| 指标 | 优化前 | 优化后 | 改进 |
|------|--------|--------|------|
| 锁竞争 | 高 | 低 | 减少锁持有时间 |
| 内存管理 | 无 | 完整 | 添加内存跟踪 |
| 过期清理 | 同步 | 异步 | 减少阻塞 |
| 统计功能 | 无 | 丰富 | 添加监控能力 |
| 错误处理 | 基础 | 完善 | 更好的参数验证 |

## 最佳实践

### 1. 内存管理
- 定期调用 `CleanupAllExpired()` 清理过期项
- 监控内存使用量，避免内存泄漏
- 合理设置TTL，避免长期占用内存

### 2. 性能优化
- 使用批量操作减少锁竞争
- 避免存储过大的对象
- 合理使用键前缀进行分类

### 3. 监控和维护
- 定期检查缓存命中率
- 监控内存使用趋势
- 根据统计信息调整缓存策略

## 向后兼容性

所有优化都保持了向后兼容性：
- 公共接口保持不变
- 现有代码无需修改
- 新增方法为可选功能
- 配置格式保持兼容

## 总结

RAM缓存驱动的优化显著提升了：

1. **性能**: 减少锁竞争，优化批量操作
2. **可靠性**: 修复过期时间逻辑，改进错误处理
3. **可观测性**: 添加完整的统计和监控功能
4. **可维护性**: 更好的代码结构和注释
5. **内存管理**: 精确的内存使用跟踪和自动清理

这些优化使RAM缓存驱动更加稳定、高效，适合在生产环境中使用。


