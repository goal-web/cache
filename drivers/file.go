package drivers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/goal-web/contracts"
	"github.com/goal-web/supports/logs"
	"github.com/goal-web/supports/utils"
)



func NewFile(config contracts.Fields) contracts.CacheStore {
	path := utils.GetStringField(config, "path", "./storage/cache")
	prefix := utils.GetStringField(config, "prefix", "")
	
	// 确保缓存目录存在
	if err := os.MkdirAll(path, 0755); err != nil {
		panic(fmt.Errorf("failed to create cache directory: %w", err))
	}
	
	return &File{
		mutex:  sync.RWMutex{},
		path:   path,
		prefix: prefix,
	}
}

type cacheData struct {
	Value     any       `json:"value"`
	ExpiredAt time.Time `json:"expired_at"`
	Forever   bool      `json:"forever"`
}

type File struct {
	mutex  sync.RWMutex
	path   string
	prefix string
}

func (f *File) getFilePath(key string) string {
	// 使用MD5哈希作为文件名，避免特殊字符问题
	hashedKey := utils.Md5(f.prefix + key)
	return filepath.Join(f.path, hashedKey+".cache")
}

func (f *File) readCacheData(filePath string) (*cacheData, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	
	var cacheData cacheData
	if err := json.Unmarshal(data, &cacheData); err != nil {
		return nil, err
	}
	
	return &cacheData, nil
}

func (f *File) writeCacheData(filePath string, data *cacheData) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	
	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	
	return os.WriteFile(filePath, jsonData, 0644)
}

func (f *File) isExpired(data *cacheData) bool {
	return !data.Forever && time.Now().After(data.ExpiredAt)
}

func (f *File) Get(key string) any {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	
	filePath := f.getFilePath(key)
	if !f.fileExists(filePath) {
		return nil
	}
	
	data, err := f.readCacheData(filePath)
	if err != nil {
		logs.WithError(err).WithField("key", key).Debug("cache.File.Get: failed to read cache data")
		return nil
	}
	
	if f.isExpired(data) {
		// 删除过期文件
		os.Remove(filePath)
		return nil
	}
	
	return data.Value
}

func (f *File) Many(keys []string) []any {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	
	results := make([]any, len(keys))
	for i, key := range keys {
		results[i] = f.Get(key)
	}
	return results
}

func (f *File) Put(key string, value any, seconds time.Duration) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	
	data := &cacheData{
		Value:     value,
		ExpiredAt: time.Now().Add(seconds),
		Forever:   false,
	}
	
	filePath := f.getFilePath(key)
	return f.writeCacheData(filePath, data)
}

func (f *File) Add(key string, value any, ttl ...time.Duration) bool {
	// 检查是否已存在
	if f.Get(key) != nil {
		return false
	}
	
	lifetime := time.Second * 5
	if len(ttl) > 0 {
		lifetime = ttl[0]
	}
	
	return f.Put(key, value, lifetime) == nil
}

func (f *File) Pull(key string, defaultValue ...any) any {
	value := f.Get(key)
	if value == nil {
		return utils.DefaultInterface(defaultValue)
	}
	
	// 删除缓存项
	f.Forget(key)
	return value
}

func (f *File) PutMany(values map[string]any, seconds time.Duration) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	
	now := time.Now()
	for key, value := range values {
		data := &cacheData{
			Value:     value,
			ExpiredAt: now.Add(seconds),
			Forever:   false,
		}
		
		filePath := f.getFilePath(key)
		if err := f.writeCacheData(filePath, data); err != nil {
			return err
		}
	}
	
	return nil
}

func (f *File) Increment(key string, value ...int64) (int64, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	
	increment := utils.DefaultInt64(value, 1)
	currentValue := f.Get(key)
	
	var count int64
	if currentValue == nil {
		count = increment
	} else {
		count = utils.ToInt64(currentValue, 0) + increment
	}
	
	// 使用默认TTL
	if err := f.Put(key, count, 24*time.Hour); err != nil {
		return 0, err
	}
	
	return count, nil
}

func (f *File) Decrement(key string, value ...int64) (int64, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	
	decrement := utils.DefaultInt64(value, 1)
	currentValue := f.Get(key)
	
	var count int64
	if currentValue == nil {
		count = -decrement
	} else {
		count = utils.ToInt64(currentValue, 0) - decrement
	}
	
	// 使用默认TTL
	if err := f.Put(key, count, 24*time.Hour); err != nil {
		return 0, err
	}
	
	return count, nil
}

func (f *File) Forever(key string, value any) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	
	data := &cacheData{
		Value:   value,
		Forever: true,
	}
	
	filePath := f.getFilePath(key)
	return f.writeCacheData(filePath, data)
}

func (f *File) Forget(key string) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	
	filePath := f.getFilePath(key)
	if f.fileExists(filePath) {
		return os.Remove(filePath)
	}
	return DataNotExistsErr
}

func (f *File) Flush() error {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	
	// 删除所有缓存文件
	return os.RemoveAll(f.path)
}

func (f *File) GetPrefix() string {
	return f.prefix
}

func (f *File) Remember(key string, ttl time.Duration, provider contracts.InstanceProvider[any]) any {
	if value := f.Get(key); value != nil {
		return value
	}
	
	value := provider()
	if err := f.Put(key, value, ttl); err != nil {
		logs.WithError(err).WithField("value", value).Debug("cache.File.Remember: value put failed")
	}
	
	return value
}

func (f *File) RememberForever(key string, provider contracts.InstanceProvider[any]) any {
	if value := f.Get(key); value != nil {
		return value
	}
	
	value := provider()
	if err := f.Forever(key, value); err != nil {
		logs.WithError(err).WithField("value", value).Debug("cache.File.RememberForever: value put failed")
	}
	
	return value
}

func (f *File) fileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}
