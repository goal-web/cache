// Package cache provides caching functionality and singleton access.
// 包 cache 提供缓存功能和单例访问。
package cache

import (
	"github.com/goal-web/contracts"
)

// Config holds cache configuration.
// Config 保存缓存配置。
type Config struct {
	Default string                         // Default specifies the default cache store.
	                                        // Default 指定默认缓存存储。
	Stores  map[string]contracts.Fields   // Stores contains cache store-specific configurations.
	                                        // Stores 包含缓存存储特定配置。
}
