package modbusserver

import (
	"app-modbus-go/internal/pkg/config"
	"app-modbus-go/internal/pkg/logger"
	"app-modbus-go/internal/pkg/mappingmanager"
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/goburrow/serial"
	"github.com/tbrandon/mbserver"
)

// ModbusServer implements the Modbus TCP server
type ModbusServer struct {
	config         *config.ModbusConfig
	server         *mbserver.Server
	mappingManager mappingmanager.MappingManagerInterface
	converter      *Converter
	lc             logger.LoggingClient
	running        atomic.Bool
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewModbusServer creates a new Modbus TCP server
func NewModbusServer(
	cfg *config.ModbusConfig,
	mappingManager mappingmanager.MappingManagerInterface,
	lc logger.LoggingClient,
) *ModbusServer {
	return &ModbusServer{
		config:         cfg,
		mappingManager: mappingManager,
		converter:      NewConverter(BigEndian),
		lc:             lc,
	}
}

// Start starts the Modbus TCP server
func (s *ModbusServer) Start(ctx context.Context) error {
	if s.running.Load() {
		return fmt.Errorf("modbus server already running")
	}

	s.ctx, s.cancel = context.WithCancel(ctx)

	// Create Modbus server
	s.server = mbserver.NewServer()

	// Register handlers for reading coils (0x01)
	s.server.RegisterFunctionHandler(1, s.handleReadCoils)

	// Register handlers for reading discrete inputs (0x02)
	s.server.RegisterFunctionHandler(2, s.handleReadDiscreteInputs)

	// Register handlers for reading holding registers (0x03)
	s.server.RegisterFunctionHandler(3, s.handleReadHoldingRegisters)

	// Register handlers for reading input registers (0x04)
	s.server.RegisterFunctionHandler(4, s.handleReadInputRegisters)

	// Register handler for writing single coil (0x05)
	s.server.RegisterFunctionHandler(5, s.handleWriteSingleCoil)

	// Register handler for writing single register (0x06)
	s.server.RegisterFunctionHandler(6, s.handleWriteSingleRegister)

	// Register handler for writing multiple coils (0x0F)
	s.server.RegisterFunctionHandler(15, s.handleWriteMultipleCoils)

	// Register handler for writing multiple registers (0x10)
	s.server.RegisterFunctionHandler(16, s.handleWriteMultipleRegisters)

	// Start listener based on configured type
	var err error
	switch s.config.Type {
	case "TCP":
		addr := fmt.Sprintf("%s:%d", s.config.TCP.Host, s.config.TCP.Port)
		err = s.server.ListenTCP(addr)
		if err != nil {
			return fmt.Errorf("failed to start Modbus TCP listener: %w", err)
		}
		s.running.Store(true)
		s.lc.Info(fmt.Sprintf("Modbus TCP server started on %s", addr))

	case "RTU":
		err = s.startRTU()
		if err != nil {
			return fmt.Errorf("failed to start Modbus RTU listener: %w", err)
		}
		s.running.Store(true)
		s.lc.Info(fmt.Sprintf("Modbus RTU server started on %s", s.config.RTU.Port))

	default:
		return fmt.Errorf("unsupported Modbus type: %s (must be TCP or RTU)", s.config.Type)
	}

	return nil
}

// handleReadCoils handles function code 0x01
func (s *ModbusServer) handleReadCoils(srv *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	data := frame.GetData()
	if len(data) < 4 {
		return nil, &mbserver.IllegalDataValue
	}

	startAddr := uint16(data[0])<<8 | uint16(data[1])
	quantity := uint16(data[2])<<8 | uint16(data[3])

	if quantity < 1 || quantity > 2000 {
		return nil, &mbserver.IllegalDataValue
	}

	s.lc.Debug(fmt.Sprintf("Read coils: addr=%d, quantity=%d", startAddr, quantity))

	// Read coil values from cache
	result, err := s.readCoils(startAddr, quantity)
	if err != nil {
		s.lc.Error(fmt.Sprintf("Read coils error: %s", err.Error()))
		return nil, &mbserver.SlaveDeviceFailure
	}

	return result, &mbserver.Success
}

// handleReadDiscreteInputs handles function code 0x02
func (s *ModbusServer) handleReadDiscreteInputs(srv *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	// Discrete inputs are handled the same as coils in this implementation
	return s.handleReadCoils(srv, frame)
}

// handleReadHoldingRegisters handles function code 0x03
func (s *ModbusServer) handleReadHoldingRegisters(srv *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	data := frame.GetData()
	if len(data) < 4 {
		return nil, &mbserver.IllegalDataValue
	}

	startAddr := uint16(data[0])<<8 | uint16(data[1])
	quantity := uint16(data[2])<<8 | uint16(data[3])

	s.lc.Debug(fmt.Sprintf("Read holding registers: addr=%d, quantity=%d", startAddr, quantity))

	// Read from cache
	result, err := s.readRegisters(startAddr, quantity)
	if err != nil {
		s.lc.Error(fmt.Sprintf("Read registers error: %s", err.Error()))
		return nil, &mbserver.SlaveDeviceFailure
	}

	return result, &mbserver.Success
}

// handleReadInputRegisters handles function code 0x04
func (s *ModbusServer) handleReadInputRegisters(srv *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	// Input registers are handled the same as holding registers in this implementation
	return s.handleReadHoldingRegisters(srv, frame)
}

// readCoils reads coil values from the mapping manager's cache
func (s *ModbusServer) readCoils(startAddr uint16, quantity uint16) ([]byte, error) {
	// Calculate byte count (8 coils per byte, round up)
	byteCount := (quantity + 7) / 8

	// Build response: byte count + coil values
	result := make([]byte, 1+byteCount)
	result[0] = byte(byteCount)

	// Read each coil value and pack into bytes
	for i := uint16(0); i < quantity; i++ {
		addr := startAddr + i
		cachedData, ok := s.mappingManager.GetCachedValue(addr)

		var bitValue bool
		if ok && cachedData != nil {
			// Convert cached value to boolean
			bitValue = s.valueToBool(cachedData.Value)
		}

		// Pack bit into byte
		if bitValue {
			byteIndex := i / 8
			bitIndex := i % 8
			result[1+byteIndex] |= (1 << bitIndex)
		}
	}

	return result, nil
}

// valueToBool converts various value types to boolean
func (s *ModbusServer) valueToBool(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case int:
		return v != 0
	case int8:
		return v != 0
	case int16:
		return v != 0
	case int32:
		return v != 0
	case int64:
		return v != 0
	case uint:
		return v != 0
	case uint8:
		return v != 0
	case uint16:
		return v != 0
	case uint32:
		return v != 0
	case uint64:
		return v != 0
	case float32:
		return v != 0.0
	case float64:
		return v != 0.0
	case string:
		return v == "true" || v == "1" || v == "on"
	default:
		return false
	}
}

// readRegisters reads register values from the mapping manager's cache
func (s *ModbusServer) readRegisters(startAddr uint16, quantity uint16) ([]byte, error) {
	// Get cached data for all requested addresses
	cachedData, err := s.mappingManager.GetCachedRegisters(startAddr, quantity)
	if err != nil {
		return nil, err
	}

	// Build response: byte count + register values
	result := make([]byte, 1+quantity*2)
	result[0] = byte(quantity * 2) // Byte count

	offset := 1
	for i := uint16(0); i < quantity; i++ {
		data := cachedData[i]
		if data == nil {
			// No data for this address, return zeros
			result[offset] = 0
			result[offset+1] = 0
		} else {
			// Convert value to bytes
			bytes, err := s.converter.ToRegisters(data.Value, data.ValueType, data.Scale, data.Offset)
			if err != nil {
				s.lc.Warn(fmt.Sprintf("Conversion error for addr %d: %s", startAddr+i, err.Error()))
				result[offset] = 0
				result[offset+1] = 0
			} else {
				// Copy first 2 bytes (one register)
				if len(bytes) >= 2 {
					copy(result[offset:offset+2], bytes[:2])
				}
			}
		}
		offset += 2
	}

	return result, nil
}

// handleWriteSingleCoil handles function code 0x05
func (s *ModbusServer) handleWriteSingleCoil(srv *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	data := frame.GetData()
	if len(data) < 4 {
		return nil, &mbserver.IllegalDataValue
	}

	addr := uint16(data[0])<<8 | uint16(data[1])
	value := uint16(data[2])<<8 | uint16(data[3])

	// Value must be 0x0000 (OFF) or 0xFF00 (ON)
	if value != 0x0000 && value != 0xFF00 {
		return nil, &mbserver.IllegalDataValue
	}

	s.lc.Debug(fmt.Sprintf("Write single coil: addr=%d, value=0x%04X", addr, value))

	// Check if this address has a mapping
	mapping, ok := s.mappingManager.GetMappingByAddress(addr)
	if !ok {
		s.lc.Warn(fmt.Sprintf("No mapping for address %d", addr))
		return nil, &mbserver.IllegalDataAddress
	}

	// Check if writeable
	if mapping.SouthResource != nil && mapping.SouthResource.ReadWrite == "R" {
		s.lc.Warn(fmt.Sprintf("Address %d is read-only", addr))
		return nil, &mbserver.IllegalDataAddress
	}

	// Echo request data for success response
	return data, &mbserver.Success
}

// handleWriteSingleRegister handles function code 0x06
func (s *ModbusServer) handleWriteSingleRegister(srv *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	data := frame.GetData()
	if len(data) < 4 {
		return nil, &mbserver.IllegalDataValue
	}

	addr := uint16(data[0])<<8 | uint16(data[1])
	value := uint16(data[2])<<8 | uint16(data[3])

	s.lc.Debug(fmt.Sprintf("Write single register: addr=%d, value=%d", addr, value))

	// Check if this address has a mapping
	mapping, ok := s.mappingManager.GetMappingByAddress(addr)
	if !ok {
		s.lc.Warn(fmt.Sprintf("No mapping for address %d", addr))
		return nil, &mbserver.IllegalDataAddress
	}

	// Check if writeable
	if mapping.SouthResource != nil && mapping.SouthResource.ReadWrite == "R" {
		s.lc.Warn(fmt.Sprintf("Address %d is read-only", addr))
		return nil, &mbserver.IllegalDataAddress
	}

	// Echo request data for success response
	return data, &mbserver.Success
}

// handleWriteMultipleCoils handles function code 0x0F (15)
func (s *ModbusServer) handleWriteMultipleCoils(srv *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	data := frame.GetData()
	if len(data) < 5 {
		return nil, &mbserver.IllegalDataValue
	}

	startAddr := uint16(data[0])<<8 | uint16(data[1])
	quantity := uint16(data[2])<<8 | uint16(data[3])
	byteCount := data[4]

	if quantity < 1 || quantity > 1968 {
		return nil, &mbserver.IllegalDataValue
	}

	expectedByteCount := (quantity + 7) / 8
	if byteCount != byte(expectedByteCount) {
		return nil, &mbserver.IllegalDataValue
	}

	if len(data) < int(5+byteCount) {
		return nil, &mbserver.IllegalDataValue
	}

	s.lc.Debug(fmt.Sprintf("Write multiple coils: addr=%d, quantity=%d", startAddr, quantity))

	// Validate all addresses are writeable
	for i := uint16(0); i < quantity; i++ {
		addr := startAddr + i
		mapping, ok := s.mappingManager.GetMappingByAddress(addr)
		if ok && mapping.SouthResource != nil && mapping.SouthResource.ReadWrite == "R" {
			s.lc.Warn(fmt.Sprintf("Address %d is read-only", addr))
			return nil, &mbserver.IllegalDataAddress
		}
	}

	// Return start address and quantity for success response
	return data[:4], &mbserver.Success
}

// handleWriteMultipleRegisters handles function code 0x10
func (s *ModbusServer) handleWriteMultipleRegisters(srv *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	data := frame.GetData()
	if len(data) < 5 {
		return nil, &mbserver.IllegalDataValue
	}

	startAddr := uint16(data[0])<<8 | uint16(data[1])
	quantity := uint16(data[2])<<8 | uint16(data[3])

	s.lc.Debug(fmt.Sprintf("Write multiple registers: addr=%d, quantity=%d", startAddr, quantity))

	// Return start address and quantity for success response
	return data[:4], &mbserver.Success
}

// startRTU starts the Modbus RTU server
func (s *ModbusServer) startRTU() error {
	// Create serial port configuration
	serialConfig := &serial.Config{
		Address:  s.config.RTU.Port,
		BaudRate: s.config.RTU.BaudRate,
		DataBits: s.config.RTU.DataBits,
		StopBits: s.config.RTU.StopBits,
		Parity:   s.config.RTU.Parity,
		Timeout:  time.Duration(s.config.Timeout) * time.Millisecond,
	}

	s.lc.Debug(fmt.Sprintf("Starting Modbus RTU on %s (baud=%d, data=%d, parity=%s, stop=%d)",
		serialConfig.Address,
		serialConfig.BaudRate,
		serialConfig.DataBits,
		serialConfig.Parity,
		serialConfig.StopBits,
	))

	// Start RTU listener
	err := s.server.ListenRTU(serialConfig)
	if err != nil {
		return fmt.Errorf("failed to start RTU listener: %w", err)
	}

	return nil
}

// Stop stops the Modbus server (TCP or RTU)
func (s *ModbusServer) Stop() error {
	if !s.running.Load() {
		return nil
	}

	s.running.Store(false)
	if s.cancel != nil {
		s.cancel()
	}

	if s.server != nil {
		s.server.Close()
	}

	s.lc.Info("Modbus TCP server stopped")
	return nil
}

// IsRunning returns whether the server is running
func (s *ModbusServer) IsRunning() bool {
	return s.running.Load()
}
