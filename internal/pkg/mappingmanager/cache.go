package mappingmanager

import (
	"sync"
	"time"
)

// CachedData 表示带有TTL的缓存数据
type CachedData struct {
	Value         interface{} // 原始值
	Timestamp     time.Time   // 数据被缓存的时间
	TTL           time.Duration
	NorthDevName  string // 北向设备名称
	ResourceName  string // 资源名称
	ValueType     string // 数据类型 (int16, float32, etc.)
	Scale         float64
	Offset        float64
	ModbusAddress uint16 // Modbus寄存器地址
}

// IsExpired 检查缓存的数据是否已过期
func (c *CachedData) IsExpired() bool {
	return time.Since(c.Timestamp) > c.TTL
}

// Cache 提供线程安全的缓存操作
type Cache struct {
	data       map[uint16]*CachedData
	mu         sync.RWMutex
	defaultTTL time.Duration
	stopCh     chan struct{}
}

// NewCache 创建新的缓存实例
func NewCache(defaultTTL time.Duration) *Cache {
	return &Cache{
		data:       make(map[uint16]*CachedData),
		defaultTTL: defaultTTL,
		stopCh:     make(chan struct{}),
	}
}

// Set 将值存储在缓存中
func (c *Cache) Set(addr uint16, data *CachedData) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if data.TTL == 0 {
		data.TTL = c.defaultTTL
	}
	data.Timestamp = time.Now()
	c.data[addr] = data
}

// Get 从缓存中检索值
func (c *Cache) Get(addr uint16) (*CachedData, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	data, ok := c.data[addr]
	if !ok {
		return nil, false
	}
	if data.IsExpired() {
		return nil, false
	}
	return data, true
}

// GetRange 从缓存中检索多个连续的值
func (c *Cache) GetRange(startAddr uint16, quantity uint16) ([]*CachedData, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*CachedData, quantity)
	for i := uint16(0); i < quantity; i++ {
		addr := startAddr + i
		data, ok := c.data[addr]
		if ok && !data.IsExpired() {
			result[i] = data
		} else {
			result[i] = nil // 此地址没有数据
		}
	}
	return result, nil
}

// Delete 从缓存中删除值
func (c *Cache) Delete(addr uint16) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, addr)
}

// Clear 从缓存中删除所有值
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[uint16]*CachedData)
}

// Cleanup 从缓存中删除过期条目
func (c *Cache) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	for addr, data := range c.data {
		if data.IsExpired() {
			delete(c.data, addr)
			count++
		}
	}
	return count
}

// StartPeriodicCleanup 启动一个goroutine，定期清理过期条目
func (c *Cache) StartPeriodicCleanup(interval time.Duration, callback func(int)) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				count := c.Cleanup()
				if callback != nil && count > 0 {
					callback(count)
				}
			case <-c.stopCh:
				return
			}
		}
	}()
}

// Stop 停止定期清理goroutine
func (c *Cache) Stop() {
	close(c.stopCh)
}

// Size 返回缓存中的项目数
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}

// GetAll 返回所有缓存数据（包括过期的）
func (c *Cache) GetAll() map[uint16]*CachedData {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[uint16]*CachedData, len(c.data))
	for k, v := range c.data {
		result[k] = v
	}
	return result
}
