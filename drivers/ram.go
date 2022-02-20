package drivers

import (
	"errors"
	"github.com/goal-web/contracts"
	"github.com/goal-web/supports/logs"
	"github.com/goal-web/supports/utils"
	"sync"
	"time"
)

var (
	DataNotExistsErr = errors.New("data does not exist")
)

func NewRam(config contracts.Fields) contracts.CacheStore {
	return &Ram{
		mutex:  sync.RWMutex{},
		data:   map[string]data{},
		ttl:    time.Duration(utils.GetIntField(config, "ttl", 24*int(time.Hour))),
		prefix: utils.GetStringField(config, "prefix"),
	}
}

type data struct {
	value     interface{}
	expiredAt time.Time
	forever   bool
}

type Ram struct {
	mutex  sync.RWMutex
	data   map[string]data
	ttl    time.Duration
	prefix string
}

func (ram *Ram) Get(key string) interface{} {
	ram.mutex.RLock()
	defer ram.mutex.RUnlock()
	if item, ok := ram.data[key]; ok {
		if item.forever || time.Now().Sub(item.expiredAt) > 0 {
			return item.value
		} else {
			delete(ram.data, key)
			return nil
		}
	}
	return nil
}

func (ram *Ram) Many(keys []string) []interface{} {
	ram.mutex.RLock()
	defer ram.mutex.RUnlock()
	var (
		results = make([]interface{}, 0)
		now     = time.Now()
	)
	for _, key := range keys {
		if item, ok := ram.data[key]; ok {
			if item.forever || now.Sub(item.expiredAt) > 0 {
				results = append(results, item.value)
			} else {
				delete(ram.data, key)
				results = append(results, nil)
			}
		}
	}
	return results
}

func (ram *Ram) Put(key string, value interface{}, seconds time.Duration) error {
	ram.mutex.Lock()
	defer ram.mutex.Unlock()
	ram.data[key] = data{
		value:     value,
		expiredAt: time.Now().Add(seconds),
	}
	return nil
}

func (ram *Ram) Add(key string, value interface{}, ttl ...time.Duration) bool {
	var item, exists = ram.data[key]
	if exists && (item.forever || time.Now().Sub(item.expiredAt) > 0) { // 存在且没过期
		return false
	}
	ram.mutex.Lock()
	defer ram.mutex.Unlock()
	var lifetime = time.Second * 5
	if len(ttl) > 0 {
		lifetime = ttl[0]
	}
	return ram.Put(key, value, lifetime) == nil
}

func (ram *Ram) Pull(key string, defaultValue ...interface{}) interface{} {
	var item, exists = ram.data[key]
	if !exists || (!item.forever && time.Now().Sub(item.expiredAt) < 0) { // 不存在或者(不是永久且已过期)
		return utils.DefaultInterface(defaultValue)
	}
	ram.mutex.Lock()
	defer ram.mutex.Unlock()
	delete(ram.data, key)
	return item.value
}

func (ram *Ram) PutMany(values map[string]interface{}, seconds time.Duration) error {
	ram.mutex.Lock()
	defer ram.mutex.Unlock()
	var now = time.Now()
	for key, value := range values {
		ram.data[key] = data{
			value:     value,
			expiredAt: now.Add(seconds),
		}
	}
	return nil
}

func (ram *Ram) Increment(key string, value ...int64) (int64, error) {
	var item, ok = ram.data[key]
	if !ok {
		item = data{
			value:     1,
			expiredAt: time.Now().Add(ram.ttl),
		}
	}
	var count = utils.ConvertToInt64(item, 0)
	count += utils.DefaultInt64(value, 1)
	ram.mutex.Lock()
	defer ram.mutex.Unlock()

	item.value = count
	item.expiredAt = time.Now().Add(ram.ttl)

	ram.data[key] = item

	return count, nil
}

func (ram *Ram) Decrement(key string, value ...int64) (int64, error) {
	var item, ok = ram.data[key]
	if !ok {
		item = data{
			value:     1,
			expiredAt: time.Now().Add(ram.ttl),
		}
	}
	var count = utils.ConvertToInt64(item, 0)
	count -= utils.DefaultInt64(value, 1)
	ram.mutex.Lock()
	defer ram.mutex.Unlock()

	item.value = count
	item.expiredAt = time.Now().Add(ram.ttl)

	ram.data[key] = item

	return count, nil
}

func (ram *Ram) Forever(key string, value interface{}) error {
	ram.mutex.Lock()
	defer ram.mutex.Unlock()
	ram.data[key] = data{value: value, forever: true}
	return nil
}

func (ram *Ram) Forget(key string) error {
	ram.mutex.Lock()
	defer ram.mutex.Unlock()
	var _, exists = ram.data[key]
	if exists {
		delete(ram.data, key)
	}
	return DataNotExistsErr
}

func (ram *Ram) Flush() error {
	ram.mutex.Lock()
	defer ram.mutex.Unlock()
	ram.data = map[string]data{}
	return nil
}

func (ram *Ram) GetPrefix() string {
	return ram.prefix
}

func (ram *Ram) Remember(key string, ttl time.Duration, provider contracts.InstanceProvider) interface{} {
	if value := ram.Get(key); value != nil {
		return value
	}
	var value = provider()
	if err := ram.Put(key, value, ttl); err != nil {
		logs.WithError(err).WithField("value", value).Debug("cache.Ram.Remember: value put failed")
	}
	return value
}

func (ram *Ram) RememberForever(key string, provider contracts.InstanceProvider) interface{} {
	if value := ram.Get(key); value != nil {
		return value
	}
	var value = provider()
	if err := ram.Forever(key, value); err != nil {
		logs.WithError(err).WithField("value", value).Debug("cache.Ram.Remember: value put failed")
	}
	return value
}
