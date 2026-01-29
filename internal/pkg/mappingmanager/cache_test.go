package mappingmanager

import (
	"sync"
	"testing"
	"time"
)

func TestNewCache(t *testing.T) {
	ttl := 30 * time.Second
	c := NewCache(ttl)

	if c == nil {
		t.Fatal("NewCache returned nil")
	}
	if c.defaultTTL != ttl {
		t.Errorf("expected defaultTTL %v, got %v", ttl, c.defaultTTL)
	}
	if c.Size() != 0 {
		t.Errorf("expected empty cache, got size %d", c.Size())
	}
}

func TestCachedDataIsExpired(t *testing.T) {
	tests := []struct {
		name     string
		ttl      time.Duration
		delay    time.Duration
		expected bool
	}{
		{"not expired", 100 * time.Millisecond, 10 * time.Millisecond, false},
		{"expired", 10 * time.Millisecond, 50 * time.Millisecond, true},
		{"just expired", 10 * time.Millisecond, 11 * time.Millisecond, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := &CachedData{
				Value:     "test",
				Timestamp: time.Now(),
				TTL:       tt.ttl,
			}
			time.Sleep(tt.delay)
			if data.IsExpired() != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", data.IsExpired(), tt.expected)
			}
		})
	}
}

func TestCacheSet(t *testing.T) {
	c := NewCache(30 * time.Second)
	data := &CachedData{
		Value:        "test",
		NorthDevName: "device1",
		ResourceName: "temp",
	}

	c.Set(1000, data)

	if c.Size() != 1 {
		t.Errorf("expected cache size 1, got %d", c.Size())
	}

	retrieved, ok := c.Get(1000)
	if !ok {
		t.Fatal("expected to retrieve data from cache")
	}
	if retrieved.Value != "test" {
		t.Errorf("expected value 'test', got %v", retrieved.Value)
	}
}

func TestCacheSetWithDefaultTTL(t *testing.T) {
	defaultTTL := 30 * time.Second
	c := NewCache(defaultTTL)
	data := &CachedData{
		Value: "test",
		TTL:   0, // Should use default
	}

	c.Set(1000, data)
	retrieved, _ := c.Get(1000)

	if retrieved.TTL != defaultTTL {
		t.Errorf("expected TTL %v, got %v", defaultTTL, retrieved.TTL)
	}
}

func TestCacheSetWithCustomTTL(t *testing.T) {
	c := NewCache(30 * time.Second)
	customTTL := 5 * time.Second
	data := &CachedData{
		Value: "test",
		TTL:   customTTL,
	}

	c.Set(1000, data)
	retrieved, _ := c.Get(1000)

	if retrieved.TTL != customTTL {
		t.Errorf("expected TTL %v, got %v", customTTL, retrieved.TTL)
	}
}

func TestCacheGet(t *testing.T) {
	c := NewCache(30 * time.Second)

	// Test non-existent key
	_, ok := c.Get(1000)
	if ok {
		t.Error("expected Get to return false for non-existent key")
	}

	// Test existing key
	data := &CachedData{Value: "test"}
	c.Set(1000, data)
	retrieved, ok := c.Get(1000)
	if !ok {
		t.Fatal("expected Get to return true for existing key")
	}
	if retrieved.Value != "test" {
		t.Errorf("expected value 'test', got %v", retrieved.Value)
	}
}

func TestCacheGetExpired(t *testing.T) {
	c := NewCache(10 * time.Millisecond)
	data := &CachedData{Value: "test"}
	c.Set(1000, data)

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	_, ok := c.Get(1000)
	if ok {
		t.Error("expected Get to return false for expired data")
	}
}

func TestCacheGetRange(t *testing.T) {
	c := NewCache(30 * time.Second)

	// Set multiple values
	for i := uint16(1000); i < 1005; i++ {
		data := &CachedData{Value: i}
		c.Set(i, data)
	}

	// Get range
	result, err := c.GetRange(1000, 5)
	if err != nil {
		t.Fatalf("GetRange returned error: %v", err)
	}

	if len(result) != 5 {
		t.Errorf("expected 5 results, got %d", len(result))
	}

	for i, data := range result {
		if data == nil {
			t.Errorf("expected data at index %d, got nil", i)
		}
	}
}

func TestCacheGetRangeWithGaps(t *testing.T) {
	c := NewCache(30 * time.Second)

	// Set only some values
	c.Set(1000, &CachedData{Value: "a"})
	c.Set(1002, &CachedData{Value: "c"})
	c.Set(1004, &CachedData{Value: "e"})

	result, _ := c.GetRange(1000, 5)

	if result[0] == nil {
		t.Error("expected data at index 0")
	}
	if result[1] != nil {
		t.Error("expected nil at index 1")
	}
	if result[2] == nil {
		t.Error("expected data at index 2")
	}
	if result[3] != nil {
		t.Error("expected nil at index 3")
	}
	if result[4] == nil {
		t.Error("expected data at index 4")
	}
}

func TestCacheDelete(t *testing.T) {
	c := NewCache(30 * time.Second)
	data := &CachedData{Value: "test"}
	c.Set(1000, data)

	if c.Size() != 1 {
		t.Errorf("expected size 1, got %d", c.Size())
	}

	c.Delete(1000)

	if c.Size() != 0 {
		t.Errorf("expected size 0 after delete, got %d", c.Size())
	}

	_, ok := c.Get(1000)
	if ok {
		t.Error("expected Get to return false after delete")
	}
}

func TestCacheClear(t *testing.T) {
	c := NewCache(30 * time.Second)

	// Add multiple items
	for i := uint16(1000); i < 1010; i++ {
		c.Set(i, &CachedData{Value: i})
	}

	if c.Size() != 10 {
		t.Errorf("expected size 10, got %d", c.Size())
	}

	c.Clear()

	if c.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", c.Size())
	}
}

func TestCacheCleanup(t *testing.T) {
	c := NewCache(10 * time.Millisecond)

	// Add items
	for i := uint16(1000); i < 1005; i++ {
		c.Set(i, &CachedData{Value: i})
	}

	if c.Size() != 5 {
		t.Errorf("expected size 5, got %d", c.Size())
	}

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Cleanup
	count := c.Cleanup()

	if count != 5 {
		t.Errorf("expected cleanup to remove 5 items, got %d", count)
	}

	if c.Size() != 0 {
		t.Errorf("expected size 0 after cleanup, got %d", c.Size())
	}
}

func TestCacheCleanupPartial(t *testing.T) {
	c := NewCache(10 * time.Millisecond)

	// Add first batch
	for i := uint16(1000); i < 1003; i++ {
		c.Set(i, &CachedData{Value: i})
	}

	// Wait for first batch to expire
	time.Sleep(20 * time.Millisecond)

	// Add second batch (fresh)
	for i := uint16(1003); i < 1005; i++ {
		c.Set(i, &CachedData{Value: i})
	}

	// Cleanup should only remove expired items
	count := c.Cleanup()

	if count != 3 {
		t.Errorf("expected cleanup to remove 3 items, got %d", count)
	}

	if c.Size() != 2 {
		t.Errorf("expected size 2 after partial cleanup, got %d", c.Size())
	}
}

func TestCacheStartPeriodicCleanup(t *testing.T) {
	c := NewCache(10 * time.Millisecond)
	cleanupCount := 0
	var mu sync.Mutex

	// Add items
	for i := uint16(1000); i < 1005; i++ {
		c.Set(i, &CachedData{Value: i})
	}

	// Start periodic cleanup
	c.StartPeriodicCleanup(15*time.Millisecond, func(count int) {
		mu.Lock()
		cleanupCount += count
		mu.Unlock()
	})

	// Wait for cleanup to run
	time.Sleep(50 * time.Millisecond)

	c.Stop()

	mu.Lock()
	if cleanupCount == 0 {
		t.Error("expected periodic cleanup to run and remove items")
	}
	mu.Unlock()
}

func TestCacheSize(t *testing.T) {
	c := NewCache(30 * time.Second)

	if c.Size() != 0 {
		t.Errorf("expected initial size 0, got %d", c.Size())
	}

	for i := uint16(1000); i < 1010; i++ {
		c.Set(i, &CachedData{Value: i})
	}

	if c.Size() != 10 {
		t.Errorf("expected size 10, got %d", c.Size())
	}
}

func TestCacheGetAll(t *testing.T) {
	c := NewCache(30 * time.Second)

	// Add items
	for i := uint16(1000); i < 1005; i++ {
		c.Set(i, &CachedData{Value: i})
	}

	all := c.GetAll()

	if len(all) != 5 {
		t.Errorf("expected 5 items in GetAll, got %d", len(all))
	}

	for i := uint16(1000); i < 1005; i++ {
		if _, ok := all[i]; !ok {
			t.Errorf("expected key %d in GetAll result", i)
		}
	}
}

func TestCacheConcurrentAccess(t *testing.T) {
	c := NewCache(30 * time.Second)
	numGoroutines := 10
	itemsPerGoroutine := 100

	var wg sync.WaitGroup

	// Writers
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < itemsPerGoroutine; i++ {
				addr := uint16(goroutineID*itemsPerGoroutine + i)
				c.Set(addr, &CachedData{Value: addr})
			}
		}(g)
	}

	// Readers
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < itemsPerGoroutine; i++ {
				addr := uint16(goroutineID*itemsPerGoroutine + i)
				c.Get(addr)
			}
		}(g)
	}

	// Cleanup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			c.Cleanup()
			time.Sleep(1 * time.Millisecond)
		}
	}()

	wg.Wait()

	expectedSize := numGoroutines * itemsPerGoroutine
	if c.Size() != expectedSize {
		t.Errorf("expected size %d, got %d", expectedSize, c.Size())
	}
}

func TestCacheStop(t *testing.T) {
	c := NewCache(30 * time.Second)
	cleanupRuns := 0
	var mu sync.Mutex

	c.StartPeriodicCleanup(10*time.Millisecond, func(count int) {
		mu.Lock()
		cleanupRuns++
		mu.Unlock()
	})

	time.Sleep(30 * time.Millisecond)
	initialRuns := cleanupRuns

	c.Stop()

	time.Sleep(30 * time.Millisecond)
	finalRuns := cleanupRuns

	if finalRuns > initialRuns {
		t.Error("expected cleanup to stop after Stop() call")
	}
}

func TestCacheMultipleAddressesWithSameValue(t *testing.T) {
	c := NewCache(30 * time.Second)
	value := "shared_value"

	// Add same value to multiple addresses
	for i := uint16(1000); i < 1005; i++ {
		c.Set(i, &CachedData{Value: value})
	}

	// Verify all addresses have the value
	for i := uint16(1000); i < 1005; i++ {
		data, ok := c.Get(i)
		if !ok || data.Value != value {
			t.Errorf("expected value %s at address %d", value, i)
		}
	}
}

func TestCacheOverwrite(t *testing.T) {
	c := NewCache(30 * time.Second)

	// Set initial value
	c.Set(1000, &CachedData{Value: "initial"})

	// Overwrite with new value
	c.Set(1000, &CachedData{Value: "updated"})

	data, _ := c.Get(1000)
	if data.Value != "updated" {
		t.Errorf("expected value 'updated', got %v", data.Value)
	}
}

func TestCacheMetadata(t *testing.T) {
	c := NewCache(30 * time.Second)

	data := &CachedData{
		Value:        123.45,
		NorthDevName: "device1",
		ResourceName: "temperature",
		ValueType:    "float32",
		Scale:        1.0,
		Offset:       0.0,
		ModbusAddress: 1000,
	}

	c.Set(1000, data)
	retrieved, _ := c.Get(1000)

	if retrieved.NorthDevName != "device1" {
		t.Errorf("expected NorthDevName 'device1', got %s", retrieved.NorthDevName)
	}
	if retrieved.ResourceName != "temperature" {
		t.Errorf("expected ResourceName 'temperature', got %s", retrieved.ResourceName)
	}
	if retrieved.ValueType != "float32" {
		t.Errorf("expected ValueType 'float32', got %s", retrieved.ValueType)
	}
	if retrieved.ModbusAddress != 1000 {
		t.Errorf("expected ModbusAddress 1000, got %d", retrieved.ModbusAddress)
	}
}
