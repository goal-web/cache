// Package cache provides caching functionality and singleton access.
// 包 cache 提供缓存功能和单例访问。
package cache

import (
	"github.com/goal-web/supports/exceptions"
)

// DriverException represents an error in the cache module drivers.
// DriverException 表示缓存模块驱动中的错误。
type DriverException = exceptions.Exception
