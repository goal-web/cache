# 文件缓存驱动 (File Cache Driver)

文件缓存驱动是 Goal Web 框架缓存组件的一个驱动实现，它将缓存数据存储在文件系统中，每个缓存项对应一个独立的文件。

## 特性

- **持久化存储**: 缓存数据存储在文件系统中，应用重启后数据仍然存在
- **自动过期**: 支持设置缓存项的过期时间，过期后自动清理
- **永久存储**: 支持永久存储的缓存项
- **线程安全**: 使用读写锁保证并发安全
- **自动目录创建**: 自动创建缓存目录结构
- **JSON序列化**: 使用JSON格式存储缓存数据，支持复杂数据类型

## 配置

### 基本配置

```toml
[stores.file]
driver = "file"
path = "./storage/cache"  # 缓存文件存储路径
prefix = "app_"          # 缓存键前缀
```

### 配置参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `driver` | string | - | 驱动名称，必须为 "file" |
| `path` | string | "./storage/cache" | 缓存文件存储的根目录路径 |
| `prefix` | string | "" | 缓存键的前缀，用于区分不同应用的缓存 |

## 使用方法

### 1. 基本操作

```go
package main

import (
    "github.com/goal-web/cache"
    "time"
)

func main() {
    // 获取文件缓存实例
    fileCache := cache.Store("file")
    
    // 存储缓存项，过期时间为1小时
    err := fileCache.Put("user:123", userData, time.Hour)
    if err != nil {
        // 处理错误
    }
    
    // 获取缓存项
    value := fileCache.Get("user:123")
    if value != nil {
        // 使用缓存数据
    }
    
    // 删除缓存项
    err = fileCache.Forget("user:123")
    
    // 清空所有缓存
    err = fileCache.Flush()
}
```

### 2. 高级操作

```go
// 永久存储
err := fileCache.Forever("config:app", appConfig)

// 递增/递减计数器
count, err := fileCache.Increment("visits:page", 1)
count, err := fileCache.Decrement("stock:product", 1)

// 批量操作
values := map[string]any{
    "user:1": user1,
    "user:2": user2,
}
err := fileCache.PutMany(values, time.Hour)

// 获取多个缓存项
results := fileCache.Many([]string{"user:1", "user:2"})

// 条件存储（仅当键不存在时）
success := fileCache.Add("unique:key", value, time.Hour)

// 获取并删除
value := fileCache.Pull("temp:data", "default_value")

// 记住模式（缓存回调结果）
result := fileCache.Remember("expensive:operation", time.Hour, func() any {
    // 执行昂贵的操作
    return expensiveOperation()
})
```

## 文件存储结构

文件缓存驱动使用以下文件结构：

```
storage/cache/
├── a1b2c3d4e5f6.cache  # 缓存文件（MD5哈希名）
├── 7g8h9i0j1k2.cache
└── ...
```

每个缓存文件包含JSON格式的数据：

```json
{
    "value": "缓存的值",
    "expired_at": "2024-01-01T12:00:00Z",
    "forever": false
}
```

## 性能考虑

### 优点

- **持久化**: 数据在应用重启后仍然存在
- **简单**: 无需额外的服务依赖
- **可移植**: 可以轻松迁移缓存数据

### 缺点

- **性能**: 相比内存缓存，文件I/O操作较慢
- **并发**: 大量并发写入可能影响性能
- **磁盘空间**: 需要管理磁盘空间使用

### 适用场景

- 开发环境和小型应用
- 需要持久化缓存的场景
- 对性能要求不高的场景
- 作为内存缓存的备选方案

## 最佳实践

1. **合理设置过期时间**: 避免缓存数据过多占用磁盘空间
2. **定期清理**: 使用 `Flush()` 方法定期清理过期缓存
3. **路径配置**: 确保缓存路径有足够的磁盘空间和权限
4. **前缀使用**: 使用有意义的前缀区分不同类型的缓存
5. **监控**: 监控缓存目录的大小和文件数量

## 错误处理

文件缓存驱动可能遇到的常见错误：

- **权限错误**: 缓存目录没有写入权限
- **磁盘空间不足**: 磁盘空间不足导致写入失败
- **文件损坏**: JSON格式错误导致读取失败

建议在生产环境中添加适当的错误处理和监控机制。

## 与其他驱动的对比

| 特性 | 文件缓存 | 内存缓存 | Redis缓存 |
|------|----------|----------|-----------|
| 持久化 | ✅ | ❌ | ✅ |
| 性能 | 中等 | 高 | 高 |
| 部署复杂度 | 低 | 低 | 中等 |
| 扩展性 | 有限 | 有限 | 高 |
| 内存占用 | 低 | 高 | 低 |

## 示例配置

完整的配置文件示例：

```toml
[stores]
default = "file"

[stores.file]
driver = "file"
path = "./storage/cache"
prefix = "app_"

[stores.temp]
driver = "file"
path = "./storage/temp_cache"
prefix = "temp_"

[stores.memory]
driver = "ram"
ttl = 3600
prefix = "mem_"
```

## 测试

运行文件缓存驱动的测试：

```bash
cd cache/drivers
go test -v -run TestFile
```

## 贡献

如果您发现bug或有改进建议，请提交issue或pull request。


