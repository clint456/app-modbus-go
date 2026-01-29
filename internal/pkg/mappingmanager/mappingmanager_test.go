package mappingmanager

import (
	"app-modbus-go/internal/pkg/config"
	"app-modbus-go/internal/pkg/logger"
	"app-modbus-go/internal/pkg/mqtt"
	"testing"
)

// MockForwardLogHandler for testing
type MockForwardLogHandler struct {
	successCalls int
	failureCalls int
	lastDevice   string
	lastData     map[string]interface{}
}

func (m *MockForwardLogHandler) LogSuccess(northDeviceName string, data map[string]interface{}) {
	m.successCalls++
	m.lastDevice = northDeviceName
	m.lastData = data
}

func (m *MockForwardLogHandler) LogFailure(northDeviceName string, data map[string]interface{}) {
	m.failureCalls++
	m.lastDevice = northDeviceName
	m.lastData = data
}

func createTestMappingManager(t *testing.T) (*MappingManager, *mqtt.ClientManager, logger.LoggingClient) {
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
	mm := NewMappingManager(mqttClient, lc, cacheConfig)
	return mm, mqttClient, lc
}

func TestNewMappingManager(t *testing.T) {
	mm, _, _ := createTestMappingManager(t)

	if mm == nil {
		t.Fatal("NewMappingManager returned nil")
	}
	if mm.cache == nil {
		t.Fatal("cache not initialized")
	}
	if len(mm.deviceMappings) != 0 {
		t.Errorf("expected empty deviceMappings, got %d", len(mm.deviceMappings))
	}
	if len(mm.addressMappings) != 0 {
		t.Errorf("expected empty addressMappings, got %d", len(mm.addressMappings))
	}
}

func TestSetForwardLogHandler(t *testing.T) {
	mm, _, _ := createTestMappingManager(t)
	handler := &MockForwardLogHandler{}

	mm.SetForwardLogHandler(handler)

	mm.mu.RLock()
	if mm.forwardLogHandler != handler {
		t.Error("forward log handler not set correctly")
	}
	mm.mu.RUnlock()
}

func TestUpdateMappings(t *testing.T) {
	mm, _, _ := createTestMappingManager(t)

	nr := &mqtt.NorthResource{
		Name:        "temperature",
		ValueType:   "float32",
		Scale:       1.0,
		OffsetValue: 0,
	}
	nr.OtherParameters.Modbus.Address = 1000

	mappings := []*mqtt.DeviceMapping{
		{
			NorthDeviceName: "device1",
			Resources: []*mqtt.ResourceMapping{
				{
					NorthResource: nr,
					SouthResource: &mqtt.SouthResource{
						Name: "temp",
					},
				},
			},
		},
	}

	err := mm.UpdateMappings(mappings)
	if err != nil {
		t.Fatalf("UpdateMappings failed: %v", err)
	}

	if len(mm.deviceMappings) != 1 {
		t.Errorf("expected 1 device mapping, got %d", len(mm.deviceMappings))
	}

	if len(mm.addressMappings) != 1 {
		t.Errorf("expected 1 address mapping, got %d", len(mm.addressMappings))
	}
}

func TestUpdateMappingsDuplicateAddress(t *testing.T) {
	mm, _, _ := createTestMappingManager(t)

	nr1 := &mqtt.NorthResource{Name: "temp1"}
	nr1.OtherParameters.Modbus.Address = 1000

	nr2 := &mqtt.NorthResource{Name: "temp2"}
	nr2.OtherParameters.Modbus.Address = 1000

	mappings := []*mqtt.DeviceMapping{
		{
			NorthDeviceName: "device1",
			Resources: []*mqtt.ResourceMapping{
				{NorthResource: nr1},
				{NorthResource: nr2},
			},
		},
	}

	err := mm.UpdateMappings(mappings)
	if err != nil {
		t.Fatalf("UpdateMappings failed: %v", err)
	}

	rm, ok := mm.GetMappingByAddress(1000)
	if !ok {
		t.Fatal("address 1000 mapping not found")
	}
	if rm.NorthResource.Name != "temp2" {
		t.Errorf("expected last resource 'temp2', got %s", rm.NorthResource.Name)
	}
}

func TestGetMappingByAddress(t *testing.T) {
	mm, _, _ := createTestMappingManager(t)

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

	rm, ok := mm.GetMappingByAddress(1000)
	if !ok {
		t.Fatal("expected to find mapping for address 1000")
	}
	if rm.NorthResource.Name != "temperature" {
		t.Errorf("expected resource 'temperature', got %s", rm.NorthResource.Name)
	}

	_, ok = mm.GetMappingByAddress(9999)
	if ok {
		t.Error("expected not to find mapping for address 9999")
	}
}

func TestGetDeviceMapping(t *testing.T) {
	mm, _, _ := createTestMappingManager(t)

	mappings := []*mqtt.DeviceMapping{
		{
			NorthDeviceName: "device1",
			Resources:       []*mqtt.ResourceMapping{},
		},
	}

	mm.UpdateMappings(mappings)

	dm, ok := mm.GetDeviceMapping("device1")
	if !ok {
		t.Fatal("expected to find mapping for device1")
	}
	if dm.NorthDeviceName != "device1" {
		t.Errorf("expected device 'device1', got %s", dm.NorthDeviceName)
	}

	_, ok = mm.GetDeviceMapping("device999")
	if ok {
		t.Error("expected not to find mapping for device999")
	}
}

func TestUpdateCache(t *testing.T) {
	mm, _, _ := createTestMappingManager(t)

	nr := &mqtt.NorthResource{
		Name:        "temperature",
		ValueType:   "float32",
		Scale:       1.0,
		OffsetValue: 0,
	}
	nr.OtherParameters.Modbus.Address = 1000

	mappings := []*mqtt.DeviceMapping{
		{
			NorthDeviceName: "device1",
			Resources: []*mqtt.ResourceMapping{
				{
					NorthResource: nr,
					SouthResource: &mqtt.SouthResource{
						Name: "temp",
					},
				},
			},
		},
	}

	mm.UpdateMappings(mappings)

	data := map[string]interface{}{
		"temp": 25.5,
	}

	err := mm.UpdateCache("device1", data)
	if err != nil {
		t.Fatalf("UpdateCache failed: %v", err)
	}

	cached, ok := mm.GetCachedValue(1000)
	if !ok {
		t.Fatal("expected cached value at address 1000")
	}
	if cached.Value != 25.5 {
		t.Errorf("expected value 25.5, got %v", cached.Value)
	}
}

func TestUpdateCacheUnknownDevice(t *testing.T) {
	mm, _, _ := createTestMappingManager(t)

	data := map[string]interface{}{
		"temp": 25.5,
	}

	err := mm.UpdateCache("unknown_device", data)
	if err == nil {
		t.Error("expected error for unknown device")
	}
}

func TestHandleSensorDataWithHandler(t *testing.T) {
	mm, _, _ := createTestMappingManager(t)
	handler := &MockForwardLogHandler{}
	mm.SetForwardLogHandler(handler)

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

	msg := &mqtt.MQTTMessage{
		Type: mqtt.TypeSensorData,
		Payload: &mqtt.SensorDataPayload{
			NorthDeviceName: "device1",
			Data: map[string]interface{}{
				"temp": 25.5,
			},
		},
	}

	err := mm.HandleSensorData(msg)
	if err != nil {
		t.Fatalf("HandleSensorData failed: %v", err)
	}

	if handler.successCalls != 1 {
		t.Errorf("expected 1 success call, got %d", handler.successCalls)
	}
	if handler.failureCalls != 0 {
		t.Errorf("expected 0 failure calls, got %d", handler.failureCalls)
	}
}

func TestHandleSensorDataFailure(t *testing.T) {
	mm, _, _ := createTestMappingManager(t)
	handler := &MockForwardLogHandler{}
	mm.SetForwardLogHandler(handler)

	msg := &mqtt.MQTTMessage{
		Type: mqtt.TypeSensorData,
		Payload: &mqtt.SensorDataPayload{
			NorthDeviceName: "unknown_device",
			Data: map[string]interface{}{
				"temp": 25.5,
			},
		},
	}

	err := mm.HandleSensorData(msg)
	if err == nil {
		t.Error("expected error for unknown device")
	}

	if handler.failureCalls != 1 {
		t.Errorf("expected 1 failure call, got %d", handler.failureCalls)
	}
}

func TestConcurrentMappingAccess(t *testing.T) {
	mm, _, _ := createTestMappingManager(t)

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

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			mm.GetMappingByAddress(1000)
			mm.GetDeviceMapping("device1")
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
