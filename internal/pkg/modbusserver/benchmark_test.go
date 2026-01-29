package modbusserver

import (
	"app-modbus-go/internal/pkg/config"
	"app-modbus-go/internal/pkg/logger"
	"app-modbus-go/internal/pkg/mappingmanager"
	"app-modbus-go/internal/pkg/mqtt"
	"testing"
	"time"
)

// BenchmarkTypeConversion benchmarks type conversion performance
func BenchmarkInt16ToBytes(b *testing.B) {
	c := NewConverter(BigEndian)
	value := int16(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.int16ToBytes(value)
	}
}

func BenchmarkUint16ToBytes(b *testing.B) {
	c := NewConverter(BigEndian)
	value := uint16(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.uint16ToBytes(value)
	}
}

func BenchmarkInt32ToBytes(b *testing.B) {
	c := NewConverter(BigEndian)
	value := int32(100000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.int32ToBytes(value)
	}
}

func BenchmarkUint32ToBytes(b *testing.B) {
	c := NewConverter(BigEndian)
	value := uint32(100000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.uint32ToBytes(value)
	}
}

func BenchmarkFloat32ToBytes(b *testing.B) {
	c := NewConverter(BigEndian)
	value := float32(123.456)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.float32ToBytes(value)
	}
}

func BenchmarkFloat64ToBytes(b *testing.B) {
	c := NewConverter(BigEndian)
	value := float64(123.456789)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.float64ToBytes(value)
	}
}

func BenchmarkToRegisters(b *testing.B) {
	c := NewConverter(BigEndian)
	value := float64(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.ToRegisters(value, "uint16", 1.0, 0)
	}
}

func BenchmarkToRegistersWithScale(b *testing.B) {
	c := NewConverter(BigEndian)
	value := float64(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.ToRegisters(value, "uint16", 10.0, 50)
	}
}

func BenchmarkFromBytes(b *testing.B) {
	c := NewConverter(BigEndian)
	data := []byte{0x03, 0xE8}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.FromBytes(data, "uint16", 1.0, 0)
	}
}

func BenchmarkFromBytesWithScale(b *testing.B) {
	c := NewConverter(BigEndian)
	data := []byte{0x03, 0xE8}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.FromBytes(data, "uint16", 10.0, 50)
	}
}

func BenchmarkApplyScaleOffset(b *testing.B) {
	c := NewConverter(BigEndian)
	value := float64(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.applyScaleOffset(value, 10.0, 50)
	}
}

func BenchmarkGetRegisterCount(b *testing.B) {
	c := NewConverter(BigEndian)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.GetRegisterCount("float32")
	}
}

// BenchmarkCacheOperations benchmarks cache performance
func BenchmarkCacheSet(b *testing.B) {
	cache := mappingmanager.NewCache(30 * time.Second)
	data := &mappingmanager.CachedData{Value: "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(uint16(i%1000), data)
	}
}

func BenchmarkCacheGet(b *testing.B) {
	cache := mappingmanager.NewCache(30 * time.Second)
	cache.Set(1000, &mappingmanager.CachedData{Value: "test"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(1000)
	}
}

func BenchmarkCacheGetRange(b *testing.B) {
	cache := mappingmanager.NewCache(30 * time.Second)
	for i := uint16(1000); i < 1100; i++ {
		cache.Set(i, &mappingmanager.CachedData{Value: i})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.GetRange(1000, 50)
	}
}

func BenchmarkCacheCleanup(b *testing.B) {
	cache := mappingmanager.NewCache(10 * time.Millisecond)
	for i := uint16(1000); i < 1100; i++ {
		cache.Set(i, &mappingmanager.CachedData{Value: i})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Cleanup()
	}
}

// BenchmarkMappingManagerOperations benchmarks mapping manager performance
func BenchmarkMappingManagerUpdateMappings(b *testing.B) {
	lc := logger.NewClient("ERROR")
	mqttClient := mqtt.NewClientManager("test-node", mqtt.ClientConfig{}, lc)
	cacheConfig := &config.CacheConfig{
		DefaultTTL:      "30s",
		CleanupInterval: "5m",
	}
	mm := mappingmanager.NewMappingManager(mqttClient, lc, cacheConfig)

	nr := &mqtt.NorthResource{
		Name: "temperature",
	}
	nr.OtherParameters.Modbus.Address = 1000

	mappings := []*mqtt.DeviceMapping{
		{
			NorthDeviceName: "device1",
			Resources: []*mqtt.ResourceMapping{
				{
					NorthResource: nr,
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mm.UpdateMappings(mappings)
	}
}

func BenchmarkMappingManagerGetMappingByAddress(b *testing.B) {
	lc := logger.NewClient("ERROR")
	mqttClient := mqtt.NewClientManager("test-node", mqtt.ClientConfig{}, lc)
	cacheConfig := &config.CacheConfig{
		DefaultTTL:      "30s",
		CleanupInterval: "5m",
	}
	mm := mappingmanager.NewMappingManager(mqttClient, lc, cacheConfig)

	nr := &mqtt.NorthResource{
		Name: "temperature",
	}
	nr.OtherParameters.Modbus.Address = 1000

	mappings := []*mqtt.DeviceMapping{
		{
			NorthDeviceName: "device1",
			Resources: []*mqtt.ResourceMapping{
				{
					NorthResource: nr,
				},
			},
		},
	}
	mm.UpdateMappings(mappings)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mm.GetMappingByAddress(1000)
	}
}

func BenchmarkMappingManagerUpdateCache(b *testing.B) {
	lc := logger.NewClient("ERROR")
	mqttClient := mqtt.NewClientManager("test-node", mqtt.ClientConfig{}, lc)
	cacheConfig := &config.CacheConfig{
		DefaultTTL:      "30s",
		CleanupInterval: "5m",
	}
	mm := mappingmanager.NewMappingManager(mqttClient, lc, cacheConfig)

	nr := &mqtt.NorthResource{
		Name: "temperature",
	}
	nr.OtherParameters.Modbus.Address = 1000

	mappings := []*mqtt.DeviceMapping{
		{
			NorthDeviceName: "device1",
			Resources: []*mqtt.ResourceMapping{
				{
					NorthResource: nr,
					SouthResource: &mqtt.SouthResource{Name: "temp"},
				},
			},
		},
	}
	mm.UpdateMappings(mappings)

	data := map[string]interface{}{"temp": 25.5}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mm.UpdateCache("device1", data)
	}
}

func BenchmarkMappingManagerGetCachedValue(b *testing.B) {
	lc := logger.NewClient("ERROR")
	mqttClient := mqtt.NewClientManager("test-node", mqtt.ClientConfig{}, lc)
	cacheConfig := &config.CacheConfig{
		DefaultTTL:      "30s",
		CleanupInterval: "5m",
	}
	mm := mappingmanager.NewMappingManager(mqttClient, lc, cacheConfig)

	nr := &mqtt.NorthResource{
		Name: "temperature",
	}
	nr.OtherParameters.Modbus.Address = 1000

	mappings := []*mqtt.DeviceMapping{
		{
			NorthDeviceName: "device1",
			Resources: []*mqtt.ResourceMapping{
				{
					NorthResource: nr,
					SouthResource: &mqtt.SouthResource{Name: "temp"},
				},
			},
		},
	}
	mm.UpdateMappings(mappings)
	mm.UpdateCache("device1", map[string]interface{}{"temp": 25.5})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mm.GetCachedValue(1000)
	}
}

// BenchmarkConcurrentOperations benchmarks concurrent access patterns
func BenchmarkCacheConcurrentSetGet(b *testing.B) {
	cache := mappingmanager.NewCache(30 * time.Second)

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			addr := uint16(i % 1000)
			cache.Set(addr, &mappingmanager.CachedData{Value: i})
			cache.Get(addr)
			i++
		}
	})
}

func BenchmarkMappingManagerConcurrentAccess(b *testing.B) {
	lc := logger.NewClient("ERROR")
	mqttCfg := mqtt.ClientConfig{
		Broker:    "tcp://localhost:1883",
		ClientID:  "test-client",
		QoS:       1,
		KeepAlive: 60,
	}
	mqttClient := mqtt.NewClientManager("test-node", mqttCfg, lc)
	cacheConfig := &config.CacheConfig{
		DefaultTTL:      "30s",
		CleanupInterval: "5m",
	}
	mm := mappingmanager.NewMappingManager(mqttClient, lc, cacheConfig)

	nr := &mqtt.NorthResource{
		Name: "temperature",
	}
	nr.OtherParameters.Modbus.Address = 1000

	mappings := []*mqtt.DeviceMapping{
		{
			NorthDeviceName: "device1",
			Resources: []*mqtt.ResourceMapping{
				{
					NorthResource: nr,
					SouthResource: &mqtt.SouthResource{Name: "temp"},
				},
			},
		},
	}
	mm.UpdateMappings(mappings)
	mm.UpdateCache("device1", map[string]interface{}{"temp": 25.5})

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mm.GetMappingByAddress(1000)
			mm.GetCachedValue(1000)
		}
	})
}

// BenchmarkTypeConversionVariations benchmarks different type conversions
func BenchmarkBoolToBytes(b *testing.B) {
	c := NewConverter(BigEndian)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.boolToBytes(i%2 == 0)
	}
}

func BenchmarkInt64ToBytes(b *testing.B) {
	c := NewConverter(BigEndian)
	value := int64(1000000000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.int64ToBytes(value)
	}
}

func BenchmarkUint64ToBytes(b *testing.B) {
	c := NewConverter(BigEndian)
	value := uint64(1000000000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.uint64ToBytes(value)
	}
}

// BenchmarkEndianness benchmarks different byte orders
func BenchmarkBigEndianConversion(b *testing.B) {
	c := NewConverter(BigEndian)
	value := uint32(100000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.uint32ToBytes(value)
	}
}

func BenchmarkLittleEndianConversion(b *testing.B) {
	c := NewConverter(LittleEndian)
	value := uint32(100000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.uint32ToBytes(value)
	}
}
