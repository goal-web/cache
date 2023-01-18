package cache

import (
	"github.com/goal-web/cache/drivers"
	"github.com/goal-web/contracts"
	"github.com/goal-web/supports/utils"
)

type serviceProvider struct {
}

func NewService() contracts.ServiceProvider {
	return serviceProvider{}
}

func (provider serviceProvider) Stop() {

}

func (provider serviceProvider) Start() error {
	return nil
}

func (provider serviceProvider) Register(container contracts.Application) {
	container.Singleton("cache", func(
		config contracts.Config,
		redis contracts.RedisFactory,
		handler contracts.ExceptionHandler) contracts.CacheFactory {
		factory := &Factory{
			config:           config.Get("cache").(Config),
			exceptionHandler: handler,
			stores:           make(map[string]contracts.CacheStore),
			drivers: map[string]contracts.CacheStoreProvider{
				"memory": drivers.NewMemory,
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
