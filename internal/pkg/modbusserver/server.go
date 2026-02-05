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

// ModbusServer 实现Modbus TCP/RTU服务器
type ModbusServer struct {
	config         *config.ModbusConfig
	server         *mbserver.Server
	mappingManager mappingmanager.MappingManagerInterface
	reader         *RegisterReader
	lc             logger.LoggingClient
	running        atomic.Bool
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewModbusServer 创建新的Modbus服务器
func NewModbusServer(
	cfg *config.ModbusConfig,
	mappingManager mappingmanager.MappingManagerInterface,
	lc logger.LoggingClient,
) *ModbusServer {
	converter := NewConverter(BigEndian)
	return &ModbusServer{
		config:         cfg,
		mappingManager: mappingManager,
		reader:         NewRegisterReader(mappingManager, converter, lc),
		lc:             lc,
	}
}

// Start 启动Modbus服务器
func (s *ModbusServer) Start(ctx context.Context) error {
	if s.running.Load() {
		return fmt.Errorf("modbus server already running")
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.server = mbserver.NewServer()

	// 注册功能码处理程序
	s.registerHandlers()

	// 启动监听器
	var err error
	switch s.config.Type {
	case "TCP":
		err = s.startTCP()
	case "RTU":
		err = s.startRTU()
	default:
		return fmt.Errorf("unsupported Modbus type: %s (must be TCP or RTU)", s.config.Type)
	}

	if err != nil {
		return err
	}

	s.running.Store(true)
	return nil
}

// registerHandlers 注册所有Modbus功能码处理程序
func (s *ModbusServer) registerHandlers() {
	// 读取功能码
	s.server.RegisterFunctionHandler(1, s.handleReadCoils)            // 0x01 读线圈
	s.server.RegisterFunctionHandler(2, s.handleReadDiscreteInputs)   // 0x02 读离散输入
	s.server.RegisterFunctionHandler(3, s.handleReadHoldingRegisters) // 0x03 读保持寄存器
	s.server.RegisterFunctionHandler(4, s.handleReadInputRegisters)   // 0x04 读输入寄存器

	// 写入功能码
	s.server.RegisterFunctionHandler(5, s.handleWriteSingleCoil)         // 0x05 写单个线圈
	s.server.RegisterFunctionHandler(6, s.handleWriteSingleRegister)     // 0x06 写单个寄存器
	s.server.RegisterFunctionHandler(15, s.handleWriteMultipleCoils)     // 0x0F 写多个线圈
	s.server.RegisterFunctionHandler(16, s.handleWriteMultipleRegisters) // 0x10 写多个寄存器
}

// startTCP 启动TCP监听器
func (s *ModbusServer) startTCP() error {
	addr := fmt.Sprintf("%s:%d", s.config.TCP.Host, s.config.TCP.Port)
	if err := s.server.ListenTCP(addr); err != nil {
		return fmt.Errorf("failed to start Modbus TCP listener: %w", err)
	}
	s.lc.Info(fmt.Sprintf("Modbus TCP server started on %s", addr))
	return nil
}

// startRTU 启动RTU监听器
func (s *ModbusServer) startRTU() error {
	serialConfig := &serial.Config{
		Address:  s.config.RTU.Port,
		BaudRate: s.config.RTU.BaudRate,
		DataBits: s.config.RTU.DataBits,
		StopBits: s.config.RTU.StopBits,
		Parity:   s.config.RTU.Parity,
		Timeout:  time.Duration(s.config.Timeout) * time.Millisecond,
	}

	if err := s.server.ListenRTU(serialConfig); err != nil {
		return fmt.Errorf("failed to start Modbus RTU listener: %w", err)
	}
	s.lc.Info(fmt.Sprintf("Modbus RTU server started on %s", s.config.RTU.Port))
	return nil
}

// ============== 读取处理程序 ==============

// handleReadCoils 处理功能码 0x01 - 读取线圈
func (s *ModbusServer) handleReadCoils(srv *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	startAddr, quantity, err := s.parseReadRequest(frame, 1, 2000)
	if err != nil {
		return nil, &mbserver.IllegalDataValue
	}

	s.lc.Debug(fmt.Sprintf("Read coils: addr=%d, quantity=%d", startAddr, quantity))

	result, err := s.reader.ReadCoils(startAddr, quantity)
	if err != nil {
		s.lc.Error(fmt.Sprintf("Read coils error: %s", err.Error()))
		return nil, &mbserver.SlaveDeviceFailure
	}

	// 记录转发日志
	s.logForward(result.ForwardedData)

	return result.Data, &mbserver.Success
}

// handleReadDiscreteInputs 处理功能码 0x02 - 读取离散输入
func (s *ModbusServer) handleReadDiscreteInputs(srv *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	startAddr, quantity, err := s.parseReadRequest(frame, 1, 2000)
	if err != nil {
		return nil, &mbserver.IllegalDataValue
	}

	s.lc.Debug(fmt.Sprintf("Read discrete inputs: addr=%d, quantity=%d", startAddr, quantity))

	result, err := s.reader.ReadDiscreteInputs(startAddr, quantity)
	if err != nil {
		s.lc.Error(fmt.Sprintf("Read discrete inputs error: %s", err.Error()))
		return nil, &mbserver.SlaveDeviceFailure
	}

	s.logForward(result.ForwardedData)
	return result.Data, &mbserver.Success
}

// handleReadHoldingRegisters 处理功能码 0x03 - 读取保持寄存器
func (s *ModbusServer) handleReadHoldingRegisters(srv *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	startAddr, quantity, err := s.parseReadRequest(frame, 1, 125)
	if err != nil {
		return nil, &mbserver.IllegalDataValue
	}

	s.lc.Debug(fmt.Sprintf("Read holding registers: addr=%d, quantity=%d", startAddr, quantity))

	result, err := s.reader.ReadHoldingRegisters(startAddr, quantity)
	if err != nil {
		s.lc.Error(fmt.Sprintf("Read holding registers error: %s", err.Error()))
		return nil, &mbserver.SlaveDeviceFailure
	}

	s.logForward(result.ForwardedData)
	return result.Data, &mbserver.Success
}

// handleReadInputRegisters 处理功能码 0x04 - 读取输入寄存器
func (s *ModbusServer) handleReadInputRegisters(srv *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	startAddr, quantity, err := s.parseReadRequest(frame, 1, 125)
	if err != nil {
		return nil, &mbserver.IllegalDataValue
	}

	s.lc.Debug(fmt.Sprintf("Read input registers: addr=%d, quantity=%d", startAddr, quantity))

	result, err := s.reader.ReadInputRegisters(startAddr, quantity)
	if err != nil {
		s.lc.Error(fmt.Sprintf("Read input registers error: %s", err.Error()))
		return nil, &mbserver.SlaveDeviceFailure
	}

	s.logForward(result.ForwardedData)
	return result.Data, &mbserver.Success
}

// ============== 写入处理程序 ==============

// handleWriteSingleCoil 处理功能码 0x05 - 写单个线圈
func (s *ModbusServer) handleWriteSingleCoil(srv *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	data := frame.GetData()
	if len(data) < 4 {
		return nil, &mbserver.IllegalDataValue
	}

	addr := uint16(data[0])<<8 | uint16(data[1])
	value := uint16(data[2])<<8 | uint16(data[3])

	// 值必须为 0x0000(关) 或 0xFF00(开)
	if value != 0x0000 && value != 0xFF00 {
		return nil, &mbserver.IllegalDataValue
	}

	s.lc.Debug(fmt.Sprintf("Write single coil: addr=%d, value=0x%04X", addr, value))

	// 检查地址映射和写权限
	if exc := s.checkWritePermission(addr); exc != nil {
		return nil, exc
	}

	// TODO: 实现实际写入逻辑（通过MQTT发送到南向设备）

	return data, &mbserver.Success
}

// handleWriteSingleRegister 处理功能码 0x06 - 写单个寄存器
func (s *ModbusServer) handleWriteSingleRegister(srv *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	data := frame.GetData()
	if len(data) < 4 {
		return nil, &mbserver.IllegalDataValue
	}

	addr := uint16(data[0])<<8 | uint16(data[1])
	value := uint16(data[2])<<8 | uint16(data[3])

	s.lc.Debug(fmt.Sprintf("Write single register: addr=%d, value=%d", addr, value))

	if exc := s.checkWritePermission(addr); exc != nil {
		return nil, exc
	}

	// TODO: 实现实际写入逻辑

	return data, &mbserver.Success
}

// handleWriteMultipleCoils 处理功能码 0x0F - 写多个线圈
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
	if byteCount != byte(expectedByteCount) || len(data) < int(5+byteCount) {
		return nil, &mbserver.IllegalDataValue
	}

	s.lc.Debug(fmt.Sprintf("Write multiple coils: addr=%d, quantity=%d", startAddr, quantity))

	// 检查所有地址的写权限
	for i := uint16(0); i < quantity; i++ {
		if exc := s.checkWritePermission(startAddr + i); exc != nil {
			return nil, exc
		}
	}

	// TODO: 实现实际写入逻辑

	return data[:4], &mbserver.Success
}

// handleWriteMultipleRegisters 处理功能码 0x10 - 写多个寄存器
func (s *ModbusServer) handleWriteMultipleRegisters(srv *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	data := frame.GetData()
	if len(data) < 5 {
		return nil, &mbserver.IllegalDataValue
	}

	startAddr := uint16(data[0])<<8 | uint16(data[1])
	quantity := uint16(data[2])<<8 | uint16(data[3])

	s.lc.Debug(fmt.Sprintf("Write multiple registers: addr=%d, quantity=%d", startAddr, quantity))

	// TODO: 实现实际写入逻辑

	return data[:4], &mbserver.Success
}

// ============== 辅助方法 ==============

// parseReadRequest 解析读取请求的起始地址和数量
func (s *ModbusServer) parseReadRequest(frame mbserver.Framer, minQty, maxQty uint16) (uint16, uint16, error) {
	data := frame.GetData()
	if len(data) < 4 {
		return 0, 0, fmt.Errorf("invalid data length")
	}

	startAddr := uint16(data[0])<<8 | uint16(data[1])
	quantity := uint16(data[2])<<8 | uint16(data[3])

	if quantity < minQty || quantity > maxQty {
		return 0, 0, fmt.Errorf("quantity out of range: %d", quantity)
	}

	return startAddr, quantity, nil
}

// checkWritePermission 检查地址的写权限
func (s *ModbusServer) checkWritePermission(addr uint16) *mbserver.Exception {
	mapping, ok := s.mappingManager.GetMappingByAddress(addr)
	if !ok {
		s.lc.Warn(fmt.Sprintf("No mapping for address %d", addr))
		return &mbserver.IllegalDataAddress
	}

	if mapping.SouthResource != nil && mapping.SouthResource.ReadWrite == "R" {
		s.lc.Warn(fmt.Sprintf("Address %d is read-only", addr))
		return &mbserver.IllegalDataAddress
	}

	return nil
}

// logForward 记录数据转发日志
func (s *ModbusServer) logForward(forwardedData map[string]map[string]interface{}) {
	if len(forwardedData) > 0 {
		s.mappingManager.LogDataForward(forwardedData)
	}
}

// Stop 停止Modbus服务器
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

	s.lc.Info("Modbus server stopped")
	return nil
}

// IsRunning 返回服务器是否正在运行
func (s *ModbusServer) IsRunning() bool {
	return s.running.Load()
}
