package mappingmanager

import (
	"app-modbus-go/internal/pkg/mqtt"
)

// MappingManagerInterface defines the mapping manager operations
type MappingManagerInterface interface {
	// QueryDeviceAttributes queries device attributes from data center at startup
	QueryDeviceAttributes() error

	// UpdateMappings updates the device-to-Modbus mappings
	UpdateMappings(mappings []*mqtt.DeviceMapping) error

	// GetMappingByAddress returns the resource mapping for a Modbus address
	GetMappingByAddress(addr uint16) (*mqtt.ResourceMapping, bool)

	// GetDeviceMapping returns the device mapping by north device name
	GetDeviceMapping(northDeviceName string) (*mqtt.DeviceMapping, bool)

	// UpdateCache updates the data cache from sensor data
	UpdateCache(northDevName string, data map[string]interface{}) error

	// GetCachedValue returns the cached value for a Modbus address
	GetCachedValue(addr uint16) (*CachedData, bool)

	// GetCachedRegisters reads multiple consecutive registers
	GetCachedRegisters(startAddr uint16, quantity uint16) ([]*CachedData, error)

	// HandleSensorData processes incoming sensor data (type=4)
	HandleSensorData(msg *mqtt.MQTTMessage) error

	// HandleQueryResponse processes query device response (type=2)
	HandleQueryResponse(resp *mqtt.MQTTResponse) error

	// HandleAttributeUpdate processes device attribute push (type=3)
	HandleAttributeUpdate(msg *mqtt.MQTTMessage) error

	// LogDataForward 记录数据转发日志（当Modbus客户端读取数据时调用）
	// data: 本次请求读取的所有资源数据 map[resourceName]value
	LogDataForward(northDeviceName string, data map[string]interface{})

	// StartCleanup starts periodic cache cleanup
	StartCleanup()

	// Stop stops the mapping manager
	Stop()
}
