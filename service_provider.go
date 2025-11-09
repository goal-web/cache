// Package cache provides caching functionality and singleton access.
// 包 cache 提供缓存功能和单例访问。
package cache

import (
	"github.com/goal-web/cache/drivers"
	"github.com/goal-web/contracts"
	"github.com/goal-web/supports/utils"
)

// serviceProvider implements the contracts.ServiceProvider interface for cache service.
// serviceProvider 实现缓存服务的 contracts.ServiceProvider 接口。
type serviceProvider struct {
}

// NewService creates a new instance of the cache service provider.
// NewService 创建缓存服务提供者的新实例。
func NewService() contracts.ServiceProvider {
	return serviceProvider{}
}

// Stop stops the cache service provider.
// Stop 停止缓存服务提供者。
func (provider serviceProvider) Stop() {

}

// Start starts the cache service provider.
// Start 启动缓存服务提供者。
func (provider serviceProvider) Start() error {
	return nil
}

// Register registers the cache service and related components in the container.
// Register 在容器中注册缓存服务和相关组件。
func (provider serviceProvider) Register(container contracts.Application) {
	container.Singleton("cache", func(
		config contracts.Config,
		redis contracts.RedisFactory,
		handler contracts.ExceptionHandler) contracts.CacheFactory {
		factory := &factory{
			config:           config.Get("cache").(Config),
			exceptionHandler: handler,
			stores:           make(map[string]contracts.CacheStore),
			drivers: map[string]contracts.CacheStoreProvider{
				"memory": drivers.NewMemory,
				"file":   drivers.NewFile,
			},
		}

		factory.Extend("redis", func(cacheConfig contracts.Fields) contracts.CacheStore {
			return drivers.NewRedisCache(
				redis.Connection(utils.GetStringField(cacheConfig, "connection")),
				utils.GetStringField(cacheConfig, "prefix"),
			)
		})

		return factory
	})
	container.Singleton("cache.store", func(factory contracts.CacheFactory) contracts.CacheStore {
		return factory.Store()
	})
}
