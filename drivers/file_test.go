package drivers

import (
	"github.com/goal-web/contracts"
	"os"
	"testing"
	"time"
)

func TestNewFile(t *testing.T) {
	// 测试目录
	testPath := "./test_cache"
	defer os.RemoveAll(testPath)

	config := contracts.Fields{
		"path":   testPath,
		"prefix": "test_",
	}

	cache := NewFile(config)

	if cache == nil {
		t.Fatal("NewFile should not return nil")
	}

	// 检查目录是否被创建
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Fatal("Cache directory should be created")
	}
}

func TestFile_Get_Put(t *testing.T) {
	testPath := "./test_cache_get_put"
	defer os.RemoveAll(testPath)

	config := contracts.Fields{
		"path":   testPath,
		"prefix": "test_",
	}

	cache := NewFile(config)

	// 测试Put和Get
	key := "test_key"
	value := "test_value"

	err := cache.Put(key, value, time.Hour)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// 获取值
	result := cache.Get(key)
	if result != value {
		t.Fatalf("Expected %v, got %v", value, result)
	}
}

func TestFile_Expiration(t *testing.T) {
	testPath := "./test_cache_expiration"
	defer os.RemoveAll(testPath)

	config := contracts.Fields{
		"path":   testPath,
		"prefix": "test_",
	}

	cache := NewFile(config)

	key := "expire_key"
	value := "expire_value"

	// 设置1毫秒的过期时间
	err := cache.Put(key, value, time.Millisecond)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// 等待过期
	time.Sleep(2 * time.Millisecond)

	// 应该返回nil
	result := cache.Get(key)
	if result != nil {
		t.Fatalf("Expected nil after expiration, got %v", result)
	}
}

func TestFile_Forever(t *testing.T) {
	testPath := "./test_cache_forever"
	defer os.RemoveAll(testPath)

	config := contracts.Fields{
		"path":   testPath,
		"prefix": "test_",
	}

	cache := NewFile(config)

	key := "forever_key"
	value := "forever_value"

	err := cache.Forever(key, value)
	if err != nil {
		t.Fatalf("Forever failed: %v", err)
	}

	// 应该能获取到值
	result := cache.Get(key)
	if result != value {
		t.Fatalf("Expected %v, got %v", value, result)
	}
}

func TestFile_Forget(t *testing.T) {
	testPath := "./test_cache_forget"
	defer os.RemoveAll(testPath)

	config := contracts.Fields{
		"path":   testPath,
		"prefix": "test_",
	}

	cache := NewFile(config)

	key := "forget_key"
	value := "forget_value"

	// 先存储值
	err := cache.Put(key, value, time.Hour)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// 删除值
	err = cache.Forget(key)
	if err != nil {
		t.Fatalf("Forget failed: %v", err)
	}

	// 应该返回nil
	result := cache.Get(key)
	if result != nil {
		t.Fatalf("Expected nil after forget, got %v", result)
	}
}

func TestFile_Flush(t *testing.T) {
	testPath := "./test_cache_flush"
	defer os.RemoveAll(testPath)

	config := contracts.Fields{
		"path":   testPath,
		"prefix": "test_",
	}

	cache := NewFile(config)

	// 存储几个值
	cache.Put("key1", "value1", time.Hour)
	cache.Put("key2", "value2", time.Hour)

	// 清空缓存
	err := cache.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// 所有值都应该被删除
	if cache.Get("key1") != nil {
		t.Fatal("key1 should be deleted after flush")
	}

	if cache.Get("key2") != nil {
		t.Fatal("key2 should be deleted after flush")
	}
}

func TestFile_Increment_Decrement(t *testing.T) {
	testPath := "./test_cache_increment"
	defer os.RemoveAll(testPath)

	config := contracts.Fields{
		"path":   testPath,
		"prefix": "test_",
	}

	cache := NewFile(config)

	key := "counter"

	// 测试递增
	count, err := cache.Increment(key, 5)
	if err != nil {
		t.Fatalf("Increment failed: %v", err)
	}
	if count != 5 {
		t.Fatalf("Expected 5, got %d", count)
	}

	// 再次递增
	count, err = cache.Increment(key, 3)
	if err != nil {
		t.Fatalf("Increment failed: %v", err)
	}
	if count != 8 {
		t.Fatalf("Expected 8, got %d", count)
	}

	// 测试递减
	count, err = cache.Decrement(key, 2)
	if err != nil {
		t.Fatalf("Decrement failed: %v", err)
	}
	if count != 6 {
		t.Fatalf("Expected 6, got %d", count)
	}
}

func TestFile_Remember(t *testing.T) {
	testPath := "./test_cache_remember"
	defer os.RemoveAll(testPath)

	config := contracts.Fields{
		"path":   testPath,
		"prefix": "test_",
	}

	cache := NewFile(config)

	key := "remember_key"

	// 第一次调用，应该执行provider
	called := false
	provider := func() any {
		called = true
		return "remembered_value"
	}

	result := cache.Remember(key, time.Hour, provider)
	if result != "remembered_value" {
		t.Fatalf("Expected remembered_value, got %v", result)
	}
	if !called {
		t.Fatal("Provider should be called")
	}

	// 第二次调用，不应该执行provider
	called = false
	result = cache.Remember(key, time.Hour, provider)
	if result != "remembered_value" {
		t.Fatalf("Expected remembered_value, got %v", result)
	}
	if called {
		t.Fatal("Provider should not be called again")
	}
}

func TestFile_GetPrefix(t *testing.T) {
	testPath := "./test_cache_prefix"
	defer os.RemoveAll(testPath)

	expectedPrefix := "test_prefix"
	config := contracts.Fields{
		"path":   testPath,
		"prefix": expectedPrefix,
	}

	cache := NewFile(config)

	prefix := cache.GetPrefix()
	if prefix != expectedPrefix {
		t.Fatalf("Expected prefix %s, got %s", expectedPrefix, prefix)
	}
}
