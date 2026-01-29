package mappingmanager

import (
	"app-modbus-go/internal/pkg/config"
	"app-modbus-go/internal/pkg/logger"
	"app-modbus-go/internal/pkg/mqtt"
	"fmt"
	"sync"
	"time"
)

// ForwardLogHandler defines the interface for forward log handling
type ForwardLogHandler interface {
	LogSuccess(northDeviceName string, data map[string]interface{})
	LogFailure(northDeviceName string, data map[string]interface{})
}

// MappingManager manages device-to-Modbus address mappings and data cache
type MappingManager struct {
	// Device mappings indexed by north device name
	deviceMappings map[string]*mqtt.DeviceMapping

	// Resource mappings indexed by Modbus address
	addressMappings map[uint16]*addressIndex

	// Data cache
	cache *Cache

	mqttClient      *mqtt.ClientManager
	forwardLogHandler ForwardLogHandler
	lc              logger.LoggingClient
	config          *config.CacheConfig
	mu              sync.RWMutex
}

// addressIndex maps a Modbus address to its resource mapping and device name
type addressIndex struct {
	DeviceName      string
	ResourceMapping *mqtt.ResourceMapping
}

// NewMappingManager creates a new MappingManager
func NewMappingManager(mqttClient *mqtt.ClientManager, lc logger.LoggingClient, cacheConfig *config.CacheConfig) *MappingManager {
	return &MappingManager{
		deviceMappings:  make(map[string]*mqtt.DeviceMapping),
		addressMappings: make(map[uint16]*addressIndex),
		cache:           NewCache(cacheConfig.GetDefaultTTL()),
		mqttClient:      mqttClient,
		forwardLogHandler: nil, // Optional, can be set later
		lc:              lc,
		config:          cacheConfig,
	}
}

// SetForwardLogHandler sets the forward log handler
func (m *MappingManager) SetForwardLogHandler(handler ForwardLogHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.forwardLogHandler = handler
}

// QueryDeviceAttributes sends a type=2 query to data center and waits for response
func (m *MappingManager) QueryDeviceAttributes() error {
	m.lc.Info("Querying device attributes from data center...")

	payload := &mqtt.QueryDevicePayload{Cmd: "0101"}
	msg := mqtt.NewMessage(mqtt.TypeQueryDevice, payload)

	resp, err := m.mqttClient.PublishAndWait(msg, 30*time.Second)
	if err != nil {
		return fmt.Errorf("query device attributes failed: %w", err)
	}

	if resp.Code != 200 {
		return fmt.Errorf("query device attributes returned code %d: %s", resp.Code, resp.Msg)
	}

	return m.HandleQueryResponse(resp)
}

// HandleQueryResponse processes query device response (type=2)
func (m *MappingManager) HandleQueryResponse(resp *mqtt.MQTTResponse) error {
	qdr, err := resp.GetQueryDeviceResponse()
	if err != nil {
		return fmt.Errorf("failed to parse query device response: %w", err)
	}

	m.lc.Info(fmt.Sprintf("Received device attributes: %d devices", len(qdr.Result)))
	return m.UpdateMappings(qdr.Result)
}

// HandleAttributeUpdate processes device attribute push (type=3)
func (m *MappingManager) HandleAttributeUpdate(msg *mqtt.MQTTMessage) error {
	payload, err := msg.GetDeviceAttributePushPayload()
	if err != nil {
		return fmt.Errorf("failed to parse attribute update: %w", err)
	}

	m.lc.Info(fmt.Sprintf("Received device attribute update: %d devices", len(payload.Devices)))
	return m.UpdateMappings(payload.Devices)
}

// UpdateMappings updates the device-to-Modbus mappings with validation
func (m *MappingManager) UpdateMappings(mappings []*mqtt.DeviceMapping) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear existing mappings
	m.deviceMappings = make(map[string]*mqtt.DeviceMapping)
	newAddressMappings := make(map[uint16]*addressIndex)

	for _, dm := range mappings {
		m.deviceMappings[dm.NorthDeviceName] = dm

		for _, rm := range dm.Resources {
			if rm.NorthResource != nil {
				addr := rm.NorthResource.OtherParameters.Modbus.Address

				// Check for duplicate address mapping
				if existing, ok := newAddressMappings[addr]; ok {
					m.lc.Warn(fmt.Sprintf("Duplicate Modbus address %d detected: %s/%s conflicts with %s/%s",
						addr, dm.NorthDeviceName, rm.NorthResource.Name,
						existing.DeviceName, existing.ResourceMapping.NorthResource.Name))
				}

				newAddressMappings[addr] = &addressIndex{
					DeviceName:      dm.NorthDeviceName,
					ResourceMapping: rm,
				}
				m.lc.Debug(fmt.Sprintf("Mapped address %d -> %s/%s",
					addr, dm.NorthDeviceName, rm.NorthResource.Name))
			}
		}
	}

	m.addressMappings = newAddressMappings
	m.lc.Info(fmt.Sprintf("Updated mappings: %d devices, %d addresses",
		len(m.deviceMappings), len(m.addressMappings)))
	return nil
}

// GetMappingByAddress returns the resource mapping for a Modbus address
func (m *MappingManager) GetMappingByAddress(addr uint16) (*mqtt.ResourceMapping, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	idx, ok := m.addressMappings[addr]
	if !ok {
		return nil, false
	}
	return idx.ResourceMapping, true
}

// GetDeviceMapping returns the device mapping by north device name
func (m *MappingManager) GetDeviceMapping(northDeviceName string) (*mqtt.DeviceMapping, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dm, ok := m.deviceMappings[northDeviceName]
	return dm, ok
}

// UpdateCache updates the data cache from sensor data
func (m *MappingManager) UpdateCache(northDevName string, data map[string]interface{}) error {
	m.mu.RLock()
	dm, ok := m.deviceMappings[northDevName]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("unknown north device: %s", northDevName)
	}

	updatedCount := 0
	for _, rm := range dm.Resources {
		if rm.NorthResource == nil || rm.SouthResource == nil {
			continue
		}

		// Try to find the value by south resource name
		val, ok := data[rm.SouthResource.Name]
		if !ok {
			// Also try north resource name
			val, ok = data[rm.NorthResource.Name]
			if !ok {
				continue
			}
		}

		addr := rm.NorthResource.OtherParameters.Modbus.Address
		m.cache.Set(addr, &CachedData{
			Value:         val,
			NorthDevName:  northDevName,
			ResourceName:  rm.NorthResource.Name,
			ValueType:     rm.NorthResource.ValueType,
			Scale:         rm.NorthResource.Scale,
			Offset:        rm.NorthResource.OffsetValue,
			ModbusAddress: addr,
		})
		updatedCount++
	}

	m.lc.Debug(fmt.Sprintf("Updated cache for device %s: %d values", northDevName, updatedCount))
	return nil
}

// GetCachedValue returns the cached value for a Modbus address
func (m *MappingManager) GetCachedValue(addr uint16) (*CachedData, bool) {
	return m.cache.Get(addr)
}

// GetCachedRegisters reads multiple consecutive registers
func (m *MappingManager) GetCachedRegisters(startAddr uint16, quantity uint16) ([]*CachedData, error) {
	return m.cache.GetRange(startAddr, quantity)
}

// HandleSensorData processes incoming sensor data (type=4)
func (m *MappingManager) HandleSensorData(msg *mqtt.MQTTMessage) error {
	payload, err := msg.GetSensorDataPayload()
	if err != nil {
		return fmt.Errorf("failed to parse sensor data: %w", err)
	}

	m.lc.Debug(fmt.Sprintf("Received sensor data from device: %s", payload.NorthDeviceName))

	if err := m.UpdateCache(payload.NorthDeviceName, payload.Data); err != nil {
		// Log failure if handler is available
		m.mu.RLock()
		handler := m.forwardLogHandler
		m.mu.RUnlock()
		if handler != nil {
			handler.LogFailure(payload.NorthDeviceName, payload.Data)
		}
		return err
	}

	// Log success if handler is available
	m.mu.RLock()
	handler := m.forwardLogHandler
	m.mu.RUnlock()
	if handler != nil {
		handler.LogSuccess(payload.NorthDeviceName, payload.Data)
	}
	return nil
}

// StartCleanup starts periodic cache cleanup
func (m *MappingManager) StartCleanup() {
	m.cache.StartPeriodicCleanup(m.config.GetCleanupInterval(), func(count int) {
		m.lc.Debug(fmt.Sprintf("Cache cleanup: removed %d expired entries", count))
	})
	m.lc.Info("Cache cleanup started")
}

// Stop stops the mapping manager
func (m *MappingManager) Stop() {
	m.cache.Stop()
}
