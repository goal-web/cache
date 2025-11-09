// Package cache provides caching functionality and singleton access.
// 包 cache 提供缓存功能和单例访问。
package cache

import (
	"fmt"

	"github.com/goal-web/contracts"
	"github.com/goal-web/supports/utils"
)

// factory implements a cache factory for creating and managing cache stores.
// factory 实现用于创建和管理缓存存储的缓存工厂。
type factory struct {
	config           Config                                   // config holds the cache configuration.
	                                                          // config 保存缓存配置。
	exceptionHandler contracts.ExceptionHandler              // exceptionHandler handles exceptions.
	                                                          // exceptionHandler 处理异常。
	stores           map[string]contracts.CacheStore         // stores holds instantiated cache stores.
	                                                          // stores 保存已实例化的缓存存储。
	drivers          map[string]contracts.CacheStoreProvider // drivers holds registered cache store drivers.
	                                                          // drivers 保存已注册的缓存存储驱动。
}

// getName returns the cache store name or the default name.
// getName 返回缓存存储名称或默认名称。
func (f *factory) getName(names ...string) string {
	if len(names) > 0 {
		return names[0]
	}
	return f.config.Default

}

// getConfig returns the configuration for the given cache store.
// getConfig 返回给定缓存存储的配置。
func (f *factory) getConfig(name string) contracts.Fields {
	return f.config.Stores[name]
}

// Store returns a cache store instance with the given name.
// Store 返回指定名称的缓存存储实例。
func (f *factory) Store(names ...string) contracts.CacheStore {
	name := f.getName(names...)
	if cacheStore, existsStore := f.stores[name]; existsStore {
		return cacheStore
	}

	f.stores[name] = f.make(name)

	return f.stores[name]
}

// Extend registers a custom cache store driver.
// Extend 注册自定义缓存存储驱动。
func (f *factory) Extend(driver string, cacheStoreProvider contracts.CacheStoreProvider) {
	f.drivers[driver] = cacheStoreProvider
}

// make creates a new cache store instance.
// make 创建新的缓存存储实例。
func (f *factory) make(name string) contracts.CacheStore {
	config := f.getConfig(name)
	driver := utils.GetStringField(config, "driver")
	driveProvider, existsProvider := f.drivers[driver]
	if !existsProvider {
		panic(DriverException{Err: fmt.Errorf("不支持的缓存驱动：%s", driver)})
	}
	return driveProvider(config)
}
