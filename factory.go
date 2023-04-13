package cache

import (
	"fmt"
	"github.com/goal-web/contracts"
	"github.com/goal-web/supports/utils"
)

type Factory struct {
	config           Config
	exceptionHandler contracts.ExceptionHandler
	stores           map[string]contracts.CacheStore
	drivers          map[string]contracts.CacheStoreProvider
}

func (factory *Factory) getName(names ...string) string {
	if len(names) > 0 {
		return names[0]
	}
	return factory.config.Default

}

func (factory *Factory) getConfig(name string) contracts.Fields {
	return factory.config.Stores[name]
}

func (factory *Factory) Store(names ...string) contracts.CacheStore {
	name := factory.getName(names...)
	if cacheStore, existsStore := factory.stores[name]; existsStore {
		return cacheStore
	}

	factory.stores[name] = factory.make(name)

	return factory.stores[name]
}

func (factory *Factory) Extend(driver string, cacheStoreProvider contracts.CacheStoreProvider) {
	factory.drivers[driver] = cacheStoreProvider
}

func (factory *Factory) make(name string) contracts.CacheStore {
	config := factory.getConfig(name)
	driver := utils.GetStringField(config, "driver")
	driveProvider, existsProvider := factory.drivers[driver]
	if !existsProvider {
		panic(DriverException{Err: fmt.Errorf("不支持的缓存驱动：%s", driver)})
	}
	return driveProvider(config)
}
