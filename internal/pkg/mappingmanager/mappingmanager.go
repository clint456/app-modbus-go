package mappingmanager

import (
	"app-modbus-go/internal/pkg/config"
	"app-modbus-go/internal/pkg/logger"
	"app-modbus-go/internal/pkg/mqtt"
	"fmt"
	"sync"
	"time"
)

// ForwardLogHandler 定义前向日志处理的接口
type ForwardLogHandler interface {
	LogSuccess(northDeviceName string, data map[string]interface{})
	LogFailure(northDeviceName string, data map[string]interface{})
}

// MappingManager 管理设备到Modbus地址的映射和数据缓存
type MappingManager struct {
	// 按北向设备名称索引的设备映射
	deviceMappings map[string]*mqtt.DeviceMapping

	// 按Modbus地址索引的资源映射
	addressMappings map[uint16]*addressIndex

	// 数据缓存
	cache *Cache

	mqttClient        *mqtt.ClientManager
	forwardLogHandler ForwardLogHandler
	lc                logger.LoggingClient
	config            *config.CacheConfig
	mu                sync.RWMutex
}

// addressIndex 将Modbus地址映射到其资源映射和设备名称
type addressIndex struct {
	DeviceName      string
	ResourceMapping *mqtt.ResourceMapping
}

// NewMappingManager 创建新的MappingManager
func NewMappingManager(mqttClient *mqtt.ClientManager, lc logger.LoggingClient, cacheConfig *config.CacheConfig) *MappingManager {
	return &MappingManager{
		deviceMappings:    make(map[string]*mqtt.DeviceMapping),
		addressMappings:   make(map[uint16]*addressIndex),
		cache:             NewCache(cacheConfig.GetDefaultTTL()),
		mqttClient:        mqttClient,
		forwardLogHandler: nil, // 可选,可以稍后设置
		lc:                lc,
		config:            cacheConfig,
	}
}

// SetForwardLogHandler 设置前向日志处理程序
func (m *MappingManager) SetForwardLogHandler(handler ForwardLogHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.forwardLogHandler = handler
}

// QueryDeviceAttributes 向数据中心发送type=2查询并等待响应
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

// HandleQueryResponse 处理查询设备响应(type=2)
func (m *MappingManager) HandleQueryResponse(resp *mqtt.MQTTResponse) error {
	qdr, err := resp.GetQueryDeviceResponse()
	if err != nil {
		return fmt.Errorf("failed to parse query device response: %w", err)
	}

	m.lc.Info(fmt.Sprintf("Received device attributes: %d devices", len(qdr.Result)))
	return m.UpdateMappings(qdr.Result)
}

// HandleAttributeUpdate 处理设备属性推送(type=3)
func (m *MappingManager) HandleAttributeUpdate(msg *mqtt.MQTTMessage) error {
	payload, err := msg.GetDeviceAttributePushPayload()
	if err != nil {
		return fmt.Errorf("failed to parse attribute update: %w", err)
	}

	m.lc.Info(fmt.Sprintf("Received device attribute update: %d devices", len(payload.Devices)))
	return m.UpdateMappings(payload.Devices)
}

// UpdateMappings 使用验证更新设备到Modbus的映射
func (m *MappingManager) UpdateMappings(mappings []*mqtt.DeviceMapping) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 清除现有映射
	m.deviceMappings = make(map[string]*mqtt.DeviceMapping)
	newAddressMappings := make(map[uint16]*addressIndex)

	for _, dm := range mappings {
		m.deviceMappings[dm.NorthDeviceName] = dm

		for _, rm := range dm.Resources {
			if rm.NorthResource != nil {
				addr := rm.NorthResource.OtherParameters.Modbus.Address

				// 检查重复的地址映射
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

// GetMappingByAddress 返回Modbus地址的资源映射
func (m *MappingManager) GetMappingByAddress(addr uint16) (*mqtt.ResourceMapping, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	idx, ok := m.addressMappings[addr]
	if !ok {
		return nil, false
	}
	return idx.ResourceMapping, true
}

// GetDeviceMapping 按北向设备名称返回设备映射
func (m *MappingManager) GetDeviceMapping(northDeviceName string) (*mqtt.DeviceMapping, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dm, ok := m.deviceMappings[northDeviceName]
	return dm, ok
}

// UpdateCache 从传感器数据更新数据缓存
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

		// 尝试按南向资源名称查找值
		val, ok := data[rm.SouthResource.Name]
		if !ok {
			// 也尝试北向资源名称
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

// GetCachedValue 返回Modbus地址的缓存值
func (m *MappingManager) GetCachedValue(addr uint16) (*CachedData, bool) {
	return m.cache.Get(addr)
}

// GetCachedRegisters 读取多个连续的寄存器
func (m *MappingManager) GetCachedRegisters(startAddr uint16, quantity uint16) ([]*CachedData, error) {
	return m.cache.GetRange(startAddr, quantity)
}

// HandleSensorData 处理传入的传感器数据(type=4)
func (m *MappingManager) HandleSensorData(msg *mqtt.MQTTMessage) error {
	payload, err := msg.GetSensorDataPayload()
	if err != nil {
		return fmt.Errorf("failed to parse sensor data: %w", err)
	}

	m.lc.Debug(fmt.Sprintf("Received sensor data from device: %s", payload.NorthDeviceName))

	if err := m.UpdateCache(payload.NorthDeviceName, payload.Data); err != nil {
		// 如果处理程序可用,记录失败
		m.mu.RLock()
		handler := m.forwardLogHandler
		m.mu.RUnlock()
		if handler != nil {
			handler.LogFailure(payload.NorthDeviceName, payload.Data)
		}
		return err
	}

	// 如果处理程序可用,记录成功
	m.mu.RLock()
	handler := m.forwardLogHandler
	m.mu.RUnlock()
	if handler != nil {
		handler.LogSuccess(payload.NorthDeviceName, payload.Data)
	}
	return nil
}

// StartCleanup 启动定期缓存清理
func (m *MappingManager) StartCleanup() {
	m.cache.StartPeriodicCleanup(m.config.GetCleanupInterval(), func(count int) {
		m.lc.Debug(fmt.Sprintf("Cache cleanup: removed %d expired entries", count))
	})
	m.lc.Info("Cache cleanup started")
}

// Stop 停止映射管理器
func (m *MappingManager) Stop() {
	m.cache.Stop()
}
