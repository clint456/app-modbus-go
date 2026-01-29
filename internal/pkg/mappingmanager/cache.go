package mappingmanager

import (
	"sync"
	"time"
)

// CachedData represents cached data with TTL
type CachedData struct {
	Value         interface{} // The raw value
	Timestamp     time.Time   // When the data was cached
	TTL           time.Duration
	NorthDevName  string // North device name
	ResourceName  string // Resource name
	ValueType     string // Data type (int16, float32, etc.)
	Scale         float64
	Offset        float64
	ModbusAddress uint16 // Modbus register address
}

// IsExpired checks if the cached data has expired
func (c *CachedData) IsExpired() bool {
	return time.Since(c.Timestamp) > c.TTL
}

// Cache provides thread-safe cache operations
type Cache struct {
	data       map[uint16]*CachedData
	mu         sync.RWMutex
	defaultTTL time.Duration
	stopCh     chan struct{}
}

// NewCache creates a new cache instance
func NewCache(defaultTTL time.Duration) *Cache {
	return &Cache{
		data:       make(map[uint16]*CachedData),
		defaultTTL: defaultTTL,
		stopCh:     make(chan struct{}),
	}
}

// Set stores a value in the cache
func (c *Cache) Set(addr uint16, data *CachedData) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if data.TTL == 0 {
		data.TTL = c.defaultTTL
	}
	data.Timestamp = time.Now()
	c.data[addr] = data
}

// Get retrieves a value from the cache
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

// GetRange retrieves multiple consecutive values from the cache
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
			result[i] = nil // No data for this address
		}
	}
	return result, nil
}

// Delete removes a value from the cache
func (c *Cache) Delete(addr uint16) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, addr)
}

// Clear removes all values from the cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[uint16]*CachedData)
}

// Cleanup removes expired entries from the cache
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

// StartPeriodicCleanup starts a goroutine that periodically cleans up expired entries
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

// Stop stops the periodic cleanup goroutine
func (c *Cache) Stop() {
	close(c.stopCh)
}

// Size returns the number of items in the cache
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}

// GetAll returns all cached data (including expired)
func (c *Cache) GetAll() map[uint16]*CachedData {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[uint16]*CachedData, len(c.data))
	for k, v := range c.data {
		result[k] = v
	}
	return result
}
