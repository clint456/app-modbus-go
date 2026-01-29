package integration

import (
	"app-modbus-go/internal/pkg/config"
	"app-modbus-go/internal/pkg/logger"
	"app-modbus-go/internal/pkg/mappingmanager"
	"app-modbus-go/internal/pkg/mqtt"
	"app-modbus-go/internal/pkg/modbusserver"
	"encoding/json"
	"testing"
	"time"
)

// TestDataFlowMQTTToModbus tests the complete data flow from MQTT to Modbus
func TestDataFlowMQTTToModbus(t *testing.T) {
	// Setup
	lc := logger.NewClient("DEBUG")
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

	// Setup mappings
	nrTemp := &mqtt.NorthResource{
		Name:        "temperature",
		ValueType:   "float32",
		Scale:       1.0,
		OffsetValue: 0,
	}
	nrTemp.OtherParameters.Modbus.Address = 1000

	nrHumidity := &mqtt.NorthResource{
		Name:        "humidity",
		ValueType:   "uint16",
		Scale:       1.0,
		OffsetValue: 0,
	}
	nrHumidity.OtherParameters.Modbus.Address = 1001

	mappings := []*mqtt.DeviceMapping{
		{
			NorthDeviceName: "device1",
			Resources: []*mqtt.ResourceMapping{
				{
					NorthResource: nrTemp,
					SouthResource: &mqtt.SouthResource{
						Name: "temp",
					},
				},
				{
					NorthResource: nrHumidity,
					SouthResource: &mqtt.SouthResource{
						Name: "humidity",
					},
				},
			},
		},
	}

	err := mm.UpdateMappings(mappings)
	if err != nil {
		t.Fatalf("failed to update mappings: %v", err)
	}

	// Simulate sensor data reception
	sensorData := &mqtt.SensorDataPayload{
		NorthDeviceName: "device1",
		Data: map[string]interface{}{
			"temp":     25.5,
			"humidity": 60.0,
		},
	}

	msg := &mqtt.MQTTMessage{
		Type:    mqtt.TypeSensorData,
		Payload: sensorData,
	}

	// Process sensor data
	err = mm.HandleSensorData(msg)
	if err != nil {
		t.Fatalf("failed to handle sensor data: %v", err)
	}

	// Verify cache was updated
	tempData, ok := mm.GetCachedValue(1000)
	if !ok {
		t.Fatal("temperature data not cached")
	}
	if tempData.Value != 25.5 {
		t.Errorf("expected temperature 25.5, got %v", tempData.Value)
	}

	humidityData, ok := mm.GetCachedValue(1001)
	if !ok {
		t.Fatal("humidity data not cached")
	}
	if humidityData.Value != 60.0 {
		t.Errorf("expected humidity 60, got %v", humidityData.Value)
	}

	// Verify Modbus read would return correct data
	converter := modbusserver.NewConverter(modbusserver.BigEndian)

	// Convert temperature to Modbus bytes
	tempBytes, err := converter.ToRegisters(tempData.Value, tempData.ValueType, tempData.Scale, tempData.Offset)
	if err != nil {
		t.Fatalf("failed to convert temperature: %v", err)
	}
	if len(tempBytes) != 4 {
		t.Errorf("expected 4 bytes for float32, got %d", len(tempBytes))
	}

	// Convert humidity to Modbus bytes
	humidityBytes, err := converter.ToRegisters(humidityData.Value, humidityData.ValueType, humidityData.Scale, humidityData.Offset)
	if err != nil {
		t.Fatalf("failed to convert humidity: %v", err)
	}
	if len(humidityBytes) != 2 {
		t.Errorf("expected 2 bytes for uint16, got %d", len(humidityBytes))
	}
}

// TestMultipleDevicesDataFlow tests data flow with multiple devices
func TestMultipleDevicesDataFlow(t *testing.T) {
	lc := logger.NewClient("DEBUG")
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

	// Setup mappings for multiple devices
	nrTemp := &mqtt.NorthResource{Name: "temperature"}
	nrTemp.OtherParameters.Modbus.Address = 1000

	nrPressure := &mqtt.NorthResource{Name: "pressure"}
	nrPressure.OtherParameters.Modbus.Address = 2000

	mappings := []*mqtt.DeviceMapping{
		{
			NorthDeviceName: "device1",
			Resources: []*mqtt.ResourceMapping{
				{
					NorthResource: nrTemp,
					SouthResource: &mqtt.SouthResource{Name: "temp"},
				},
			},
		},
		{
			NorthDeviceName: "device2",
			Resources: []*mqtt.ResourceMapping{
				{
					NorthResource: nrPressure,
					SouthResource: &mqtt.SouthResource{Name: "pressure"},
				},
			},
		},
	}

	mm.UpdateMappings(mappings)

	// Send data from device1
	msg1 := &mqtt.MQTTMessage{
		Type: mqtt.TypeSensorData,
		Payload: &mqtt.SensorDataPayload{
			NorthDeviceName: "device1",
			Data: map[string]interface{}{"temp": 25.5},
		},
	}
	mm.HandleSensorData(msg1)

	// Send data from device2
	msg2 := &mqtt.MQTTMessage{
		Type: mqtt.TypeSensorData,
		Payload: &mqtt.SensorDataPayload{
			NorthDeviceName: "device2",
			Data: map[string]interface{}{"pressure": 1013.25},
		},
	}
	mm.HandleSensorData(msg2)

	// Verify both devices' data are cached
	data1, ok := mm.GetCachedValue(1000)
	if !ok || data1.Value != 25.5 {
		t.Error("device1 data not properly cached")
	}

	data2, ok := mm.GetCachedValue(2000)
	if !ok || data2.Value != 1013.25 {
		t.Error("device2 data not properly cached")
	}
}

// TestCacheExpiration tests that expired cache entries are handled correctly
func TestCacheExpiration(t *testing.T) {
	lc := logger.NewClient("DEBUG")
	mqttCfg := mqtt.ClientConfig{
		Broker:    "tcp://localhost:1883",
		ClientID:  "test-client",
		QoS:       1,
		KeepAlive: 60,
	}
	mqttClient := mqtt.NewClientManager("test-node", mqttCfg, lc)

	cacheConfig := &config.CacheConfig{
		DefaultTTL:      "10ms",
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

	// Add data to cache
	msg := &mqtt.MQTTMessage{
		Type: mqtt.TypeSensorData,
		Payload: &mqtt.SensorDataPayload{
			NorthDeviceName: "device1",
			Data: map[string]interface{}{"temp": 25.5},
		},
	}
	mm.HandleSensorData(msg)

	// Verify data is cached
	_, ok := mm.GetCachedValue(1000)
	if !ok {
		t.Fatal("data not cached")
	}

	// Wait for expiration
	time.Sleep(50 * time.Millisecond)

	// Verify data is expired
	_, ok = mm.GetCachedValue(1000)
	if ok {
		t.Error("expected expired data to be unavailable")
	}
}

// TestMessageSerialization tests MQTT message serialization/deserialization
func TestMessageSerialization(t *testing.T) {
	payload := &mqtt.SensorDataPayload{
		NorthDeviceName: "device1",
		Data: map[string]interface{}{
			"temperature": 25.5,
			"humidity":    60,
		},
	}

	msg := mqtt.NewMessage(mqtt.TypeSensorData, payload)

	// Serialize
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal message: %v", err)
	}

	// Deserialize
	var unmarshaled mqtt.MQTTMessage
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal message: %v", err)
	}

	if unmarshaled.Type != mqtt.TypeSensorData {
		t.Errorf("expected type %d, got %d", mqtt.TypeSensorData, unmarshaled.Type)
	}
}

// TestTypeConversionAccuracy tests accuracy of type conversions
func TestTypeConversionAccuracy(t *testing.T) {
	converter := modbusserver.NewConverter(modbusserver.BigEndian)

	tests := []struct {
		name      string
		value     interface{}
		valueType string
		scale     float64
		offset    float64
	}{
		{"int16", int16(1000), "int16", 1.0, 0},
		{"uint16", uint16(5000), "uint16", 1.0, 0},
		{"float32", float32(123.456), "float32", 1.0, 0},
		{"with scale", float64(100), "uint16", 10.0, 0},
		{"with offset", float64(100), "uint16", 1.0, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to bytes
			bytes, err := converter.ToRegisters(tt.value, tt.valueType, tt.scale, tt.offset)
			if err != nil {
				t.Fatalf("ToRegisters failed: %v", err)
			}

			// Convert back
			result, err := converter.FromBytes(bytes, tt.valueType, tt.scale, tt.offset)
			if err != nil {
				t.Fatalf("FromBytes failed: %v", err)
			}

			// Verify round-trip
			if result == nil {
				t.Error("FromBytes returned nil")
			}
		})
	}
}

// TestConcurrentDataFlow tests concurrent data flow handling
func TestConcurrentDataFlow(t *testing.T) {
	lc := logger.NewClient("DEBUG")
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

	// Setup mappings
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

	// Send concurrent data
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			msg := &mqtt.MQTTMessage{
				Type: mqtt.TypeSensorData,
				Payload: &mqtt.SensorDataPayload{
					NorthDeviceName: "device1",
					Data: map[string]interface{}{"temp": float64(20 + id)},
				},
			}
			mm.HandleSensorData(msg)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final data is cached
	data, ok := mm.GetCachedValue(1000)
	if !ok {
		t.Fatal("data not cached after concurrent updates")
	}
	if data.Value == nil {
		t.Error("cached value is nil")
	}
}

// TestMappingUpdate tests dynamic mapping updates
func TestMappingUpdate(t *testing.T) {
	lc := logger.NewClient("DEBUG")
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

	// Initial mappings
	nr1 := &mqtt.NorthResource{
		Name: "temperature",
	}
	nr1.OtherParameters.Modbus.Address = 1000

	mappings1 := []*mqtt.DeviceMapping{
		{
			NorthDeviceName: "device1",
			Resources: []*mqtt.ResourceMapping{
				{
					NorthResource: nr1,
					SouthResource: &mqtt.SouthResource{Name: "temp"},
				},
			},
		},
	}
	mm.UpdateMappings(mappings1)

	// Verify initial mapping
	_, ok := mm.GetMappingByAddress(1000)
	if !ok {
		t.Fatal("initial mapping not found")
	}

	// Update mappings
	nr2 := &mqtt.NorthResource{
		Name: "temperature",
	}
	nr2.OtherParameters.Modbus.Address = 2000

	mappings2 := []*mqtt.DeviceMapping{
		{
			NorthDeviceName: "device1",
			Resources: []*mqtt.ResourceMapping{
				{
					NorthResource: nr2,
					SouthResource: &mqtt.SouthResource{Name: "temp"},
				},
			},
		},
	}
	mm.UpdateMappings(mappings2)

	// Verify old mapping is gone
	_, ok = mm.GetMappingByAddress(1000)
	if ok {
		t.Error("old mapping should be removed")
	}

	// Verify new mapping exists
	_, ok = mm.GetMappingByAddress(2000)
	if !ok {
		t.Fatal("new mapping not found")
	}
}
