package modbusserver

import (
	"app-modbus-go/internal/pkg/config"
	"app-modbus-go/internal/pkg/logger"
	"app-modbus-go/internal/pkg/mappingmanager"
	"context"
	"fmt"
	"sync/atomic"

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

	// Register handlers for reading holding registers (0x03)
	s.server.RegisterFunctionHandler(3, s.handleReadHoldingRegisters)

	// Register handlers for reading input registers (0x04)
	s.server.RegisterFunctionHandler(4, s.handleReadInputRegisters)

	// Register handler for writing single register (0x06)
	s.server.RegisterFunctionHandler(6, s.handleWriteSingleRegister)

	// Register handler for writing multiple registers (0x10)
	s.server.RegisterFunctionHandler(16, s.handleWriteMultipleRegisters)

	// Start TCP listener
	addr := fmt.Sprintf("%s:%d", s.config.TCP.Host, s.config.TCP.Port)
	err := s.server.ListenTCP(addr)
	if err != nil {
		return fmt.Errorf("failed to start Modbus TCP listener: %w", err)
	}

	s.running.Store(true)
	s.lc.Info(fmt.Sprintf("Modbus TCP server started on %s", addr))

	return nil
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

// Stop stops the Modbus TCP server
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
