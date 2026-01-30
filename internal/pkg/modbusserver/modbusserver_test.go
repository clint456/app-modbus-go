package modbusserver

import (
	"app-modbus-go/internal/pkg/logger"
	"app-modbus-go/internal/pkg/mappingmanager"
	"app-modbus-go/internal/pkg/mqtt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/tbrandon/mbserver"
)

// MockLogger is a mock implementation of LoggingClient
type MockLogger struct{}

// Ensure MockLogger implements logger.LoggingClient
var _ logger.LoggingClient = (*MockLogger)(nil)

func (m *MockLogger) SetLogLevel(logLevel string) error { return nil }
func (m *MockLogger) LogLevel() string                  { return "DEBUG" }
func (m *MockLogger) Debug(msg string, args ...interface{}) {}
func (m *MockLogger) Error(msg string, args ...interface{}) {}
func (m *MockLogger) Info(msg string, args ...interface{})  {}
func (m *MockLogger) Trace(msg string, args ...interface{}) {}
func (m *MockLogger) Warn(msg string, args ...interface{})  {}
func (m *MockLogger) Debugf(msg string, args ...interface{}) {}
func (m *MockLogger) Errorf(msg string, args ...interface{}) {}
func (m *MockLogger) Infof(msg string, args ...interface{})  {}
func (m *MockLogger) Tracef(msg string, args ...interface{}) {}
func (m *MockLogger) Warnf(msg string, args ...interface{})  {}
func (m *MockLogger) Close() error                           { return nil }

// MockMappingManager is a mock implementation of MappingManagerInterface
type MockMappingManager struct {
	mock.Mock
}

func (m *MockMappingManager) QueryDeviceAttributes() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockMappingManager) UpdateMappings(mappings []*mqtt.DeviceMapping) error {
	args := m.Called(mappings)
	return args.Error(0)
}

func (m *MockMappingManager) GetMappingByAddress(addr uint16) (*mqtt.ResourceMapping, bool) {
	args := m.Called(addr)
	if args.Get(0) == nil {
		return nil, args.Bool(1)
	}
	return args.Get(0).(*mqtt.ResourceMapping), args.Bool(1)
}

func (m *MockMappingManager) GetDeviceMapping(northDeviceName string) (*mqtt.DeviceMapping, bool) {
	args := m.Called(northDeviceName)
	if args.Get(0) == nil {
		return nil, args.Bool(1)
	}
	return args.Get(0).(*mqtt.DeviceMapping), args.Bool(1)
}

func (m *MockMappingManager) UpdateCache(northDevName string, data map[string]interface{}) error {
	args := m.Called(northDevName, data)
	return args.Error(0)
}

func (m *MockMappingManager) GetCachedValue(addr uint16) (*mappingmanager.CachedData, bool) {
	args := m.Called(addr)
	if args.Get(0) == nil {
		return nil, args.Bool(1)
	}
	return args.Get(0).(*mappingmanager.CachedData), args.Bool(1)
}

func (m *MockMappingManager) GetCachedRegisters(startAddr uint16, quantity uint16) ([]*mappingmanager.CachedData, error) {
	args := m.Called(startAddr, quantity)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*mappingmanager.CachedData), args.Error(1)
}

func (m *MockMappingManager) HandleSensorData(msg *mqtt.MQTTMessage) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *MockMappingManager) HandleQueryResponse(resp *mqtt.MQTTResponse) error {
	args := m.Called(resp)
	return args.Error(0)
}

func (m *MockMappingManager) HandleAttributeUpdate(msg *mqtt.MQTTMessage) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *MockMappingManager) StartCleanup() {
	m.Called()
}

func (m *MockMappingManager) Stop() {
	m.Called()
}

// MockFramer is a mock implementation of mbserver.Framer
type MockFramer struct {
	data      []byte
	function  uint8
	exception *byte
}

func (m *MockFramer) GetData() []byte {
	return m.data
}

func (m *MockFramer) GetFunction() uint8 {
	return m.function
}

func (m *MockFramer) Bytes() []byte {
	return m.data
}

func (m *MockFramer) Copy() mbserver.Framer {
	return &MockFramer{
		data:     m.data,
		function: m.function,
	}
}

func (m *MockFramer) SetException(exception *mbserver.Exception) {
	if exception != nil {
		val := byte(*exception)
		m.exception = &val
	}
}

func (m *MockFramer) SetData(data []byte) {
	m.data = data
}

// TestValueToBool tests the valueToBool function with various data types
func TestValueToBool(t *testing.T) {
	s := &ModbusServer{}

	tests := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		// Boolean tests
		{"bool true", true, true},
		{"bool false", false, false},

		// Integer tests
		{"int zero", int(0), false},
		{"int non-zero", int(1), true},
		{"int negative", int(-1), true},
		{"int8 zero", int8(0), false},
		{"int8 non-zero", int8(5), true},
		{"int16 zero", int16(0), false},
		{"int16 non-zero", int16(100), true},
		{"int32 zero", int32(0), false},
		{"int32 non-zero", int32(1000), true},
		{"int64 zero", int64(0), false},
		{"int64 non-zero", int64(10000), true},

		// Unsigned integer tests
		{"uint zero", uint(0), false},
		{"uint non-zero", uint(1), true},
		{"uint8 zero", uint8(0), false},
		{"uint8 non-zero", uint8(255), true},
		{"uint16 zero", uint16(0), false},
		{"uint16 non-zero", uint16(1000), true},
		{"uint32 zero", uint32(0), false},
		{"uint32 non-zero", uint32(50000), true},
		{"uint64 zero", uint64(0), false},
		{"uint64 non-zero", uint64(100000), true},

		// Float tests
		{"float32 zero", float32(0.0), false},
		{"float32 non-zero", float32(1.5), true},
		{"float32 negative", float32(-2.5), true},
		{"float64 zero", float64(0.0), false},
		{"float64 non-zero", float64(3.14), true},

		// String tests
		{"string true", "true", true},
		{"string 1", "1", true},
		{"string on", "on", true},
		{"string false", "false", false},
		{"string 0", "0", false},
		{"string empty", "", false},
		{"string other", "hello", false},

		// Other types
		{"nil", nil, false},
		{"struct", struct{}{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.valueToBool(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestReadCoils tests the readCoils function
func TestReadCoils(t *testing.T) {
	mockMM := new(MockMappingManager)
	s := &ModbusServer{
		mappingManager: mockMM,
	}

	t.Run("read 8 coils with mixed values", func(t *testing.T) {
		// Setup mock to return coil values: true, false, true, true, false, false, true, false
		// This should pack into byte: 0b01001101 = 0x4D
		mockMM.On("GetCachedValue", uint16(0)).Return(&mappingmanager.CachedData{Value: true}, true).Once()
		mockMM.On("GetCachedValue", uint16(1)).Return(&mappingmanager.CachedData{Value: false}, true).Once()
		mockMM.On("GetCachedValue", uint16(2)).Return(&mappingmanager.CachedData{Value: true}, true).Once()
		mockMM.On("GetCachedValue", uint16(3)).Return(&mappingmanager.CachedData{Value: true}, true).Once()
		mockMM.On("GetCachedValue", uint16(4)).Return(&mappingmanager.CachedData{Value: false}, true).Once()
		mockMM.On("GetCachedValue", uint16(5)).Return(&mappingmanager.CachedData{Value: false}, true).Once()
		mockMM.On("GetCachedValue", uint16(6)).Return(&mappingmanager.CachedData{Value: true}, true).Once()
		mockMM.On("GetCachedValue", uint16(7)).Return(&mappingmanager.CachedData{Value: false}, true).Once()

		result, err := s.readCoils(0, 8)

		assert.NoError(t, err)
		assert.Equal(t, 2, len(result)) // 1 byte count + 1 byte data
		assert.Equal(t, byte(1), result[0]) // byte count
		assert.Equal(t, byte(0x4D), result[1]) // packed coil values
		mockMM.AssertExpectations(t)
	})

	t.Run("read 10 coils spanning 2 bytes", func(t *testing.T) {
		mockMM := new(MockMappingManager)
		s.mappingManager = mockMM

		// First 8 coils: all true = 0xFF
		for i := uint16(0); i < 8; i++ {
			mockMM.On("GetCachedValue", uint16(100+i)).Return(&mappingmanager.CachedData{Value: true}, true).Once()
		}
		// Next 2 coils: true, false = 0b00000001 = 0x01
		mockMM.On("GetCachedValue", uint16(108)).Return(&mappingmanager.CachedData{Value: true}, true).Once()
		mockMM.On("GetCachedValue", uint16(109)).Return(&mappingmanager.CachedData{Value: false}, true).Once()

		result, err := s.readCoils(100, 10)

		assert.NoError(t, err)
		assert.Equal(t, 3, len(result)) // 1 byte count + 2 bytes data
		assert.Equal(t, byte(2), result[0]) // byte count
		assert.Equal(t, byte(0xFF), result[1]) // first 8 coils
		assert.Equal(t, byte(0x01), result[2]) // last 2 coils
		mockMM.AssertExpectations(t)
	})

	t.Run("read coils with no cached data", func(t *testing.T) {
		mockMM := new(MockMappingManager)
		s.mappingManager = mockMM

		// Return no cached data (should default to false)
		for i := uint16(0); i < 5; i++ {
			mockMM.On("GetCachedValue", uint16(200+i)).Return(nil, false).Once()
		}

		result, err := s.readCoils(200, 5)

		assert.NoError(t, err)
		assert.Equal(t, 2, len(result)) // 1 byte count + 1 byte data
		assert.Equal(t, byte(1), result[0]) // byte count
		assert.Equal(t, byte(0x00), result[1]) // all coils false
		mockMM.AssertExpectations(t)
	})
}

// TestHandleWriteSingleCoil tests the handleWriteSingleCoil function
func TestHandleWriteSingleCoil(t *testing.T) {
	mockMM := new(MockMappingManager)
	mockLogger := &MockLogger{}
	s := &ModbusServer{
		mappingManager: mockMM,
		lc:             mockLogger,
	}

	t.Run("write single coil ON", func(t *testing.T) {
		// Modbus frame: address=100 (0x0064), value=0xFF00 (ON)
		frame := &MockFramer{
			data: []byte{0x00, 0x64, 0xFF, 0x00},
		}

		mapping := &mqtt.ResourceMapping{
			SouthResource: &mqtt.SouthResource{
				ReadWrite: "RW",
			},
		}
		mockMM.On("GetMappingByAddress", uint16(100)).Return(mapping, true).Once()

		result, exception := s.handleWriteSingleCoil(nil, frame)

		assert.NotNil(t, result)
		assert.Equal(t, []byte{0x00, 0x64, 0xFF, 0x00}, result)
		assert.Equal(t, mbserver.Success, *exception)
		mockMM.AssertExpectations(t)
	})

	t.Run("write single coil OFF", func(t *testing.T) {
		mockMM := new(MockMappingManager)
		s.mappingManager = mockMM

		// Modbus frame: address=200 (0x00C8), value=0x0000 (OFF)
		frame := &MockFramer{
			data: []byte{0x00, 0xC8, 0x00, 0x00},
		}

		mapping := &mqtt.ResourceMapping{
			SouthResource: &mqtt.SouthResource{
				ReadWrite: "RW",
			},
		}
		mockMM.On("GetMappingByAddress", uint16(200)).Return(mapping, true).Once()

		result, exception := s.handleWriteSingleCoil(nil, frame)

		assert.NotNil(t, result)
		assert.Equal(t, []byte{0x00, 0xC8, 0x00, 0x00}, result)
		assert.Equal(t, mbserver.Success, *exception)
		mockMM.AssertExpectations(t)
	})

	t.Run("invalid coil value", func(t *testing.T) {
		mockMM := new(MockMappingManager)
		s.mappingManager = mockMM

		// Invalid value: 0x0001 (must be 0x0000 or 0xFF00)
		frame := &MockFramer{
			data: []byte{0x00, 0x64, 0x00, 0x01},
		}

		result, exception := s.handleWriteSingleCoil(nil, frame)

		assert.Nil(t, result)
		assert.Equal(t, mbserver.IllegalDataValue, *exception)
	})

	t.Run("no mapping for address", func(t *testing.T) {
		mockMM := new(MockMappingManager)
		s.mappingManager = mockMM

		frame := &MockFramer{
			data: []byte{0x00, 0x64, 0xFF, 0x00},
		}

		mockMM.On("GetMappingByAddress", uint16(100)).Return(nil, false).Once()

		result, exception := s.handleWriteSingleCoil(nil, frame)

		assert.Nil(t, result)
		assert.Equal(t, mbserver.IllegalDataAddress, *exception)
		mockMM.AssertExpectations(t)
	})

	t.Run("read-only address", func(t *testing.T) {
		mockMM := new(MockMappingManager)
		s.mappingManager = mockMM

		frame := &MockFramer{
			data: []byte{0x00, 0x64, 0xFF, 0x00},
		}

		mapping := &mqtt.ResourceMapping{
			SouthResource: &mqtt.SouthResource{
				ReadWrite: "R",
			},
		}
		mockMM.On("GetMappingByAddress", uint16(100)).Return(mapping, true).Once()

		result, exception := s.handleWriteSingleCoil(nil, frame)

		assert.Nil(t, result)
		assert.Equal(t, mbserver.IllegalDataAddress, *exception)
		mockMM.AssertExpectations(t)
	})
}

// TestHandleWriteMultipleCoils tests the handleWriteMultipleCoils function
func TestHandleWriteMultipleCoils(t *testing.T) {
	mockMM := new(MockMappingManager)
	mockLogger := &MockLogger{}
	s := &ModbusServer{
		mappingManager: mockMM,
		lc:             mockLogger,
	}

	t.Run("write 8 coils successfully", func(t *testing.T) {
		// Write 8 coils starting at address 100
		// Data: start=100, quantity=8, byteCount=1, values=0xFF (all ON)
		frame := &MockFramer{
			data: []byte{0x00, 0x64, 0x00, 0x08, 0x01, 0xFF},
		}

		// Mock all addresses as writeable
		for i := uint16(0); i < 8; i++ {
			mapping := &mqtt.ResourceMapping{
				SouthResource: &mqtt.SouthResource{
					ReadWrite: "RW",
				},
			}
			mockMM.On("GetMappingByAddress", uint16(100+i)).Return(mapping, true).Once()
		}

		result, exception := s.handleWriteMultipleCoils(nil, frame)

		assert.NotNil(t, result)
		assert.Equal(t, []byte{0x00, 0x64, 0x00, 0x08}, result) // start address + quantity
		assert.Equal(t, mbserver.Success, *exception)
		mockMM.AssertExpectations(t)
	})
}
