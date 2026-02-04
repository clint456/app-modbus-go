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

// ModbusServer 实现Modbus TCP服务器
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

// NewModbusServer 创建新的Modbus TCP服务器
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

// Start 启动Modbus TCP服务器
func (s *ModbusServer) Start(ctx context.Context) error {
	if s.running.Load() {
		return fmt.Errorf("modbus server already running")
	}

	s.ctx, s.cancel = context.WithCancel(ctx)

	// 创建Modbus服务器
	s.server = mbserver.NewServer()

	// 注册读取线圈(0x01)的处理程序
	s.server.RegisterFunctionHandler(1, s.handleReadCoils)

	// 注册读取离散输入(0x02)的处理程序
	s.server.RegisterFunctionHandler(2, s.handleReadDiscreteInputs)

	// 注册读取保持寄存器(0x03)的处理程序
	s.server.RegisterFunctionHandler(3, s.handleReadHoldingRegisters)

	// 注册读取输入寄存器(0x04)的处理程序
	s.server.RegisterFunctionHandler(4, s.handleReadInputRegisters)

	// 注册写入单个线圈(0x05)的处理程序
	s.server.RegisterFunctionHandler(5, s.handleWriteSingleCoil)

	// 注册写入单个寄存器(0x06)的处理程序
	s.server.RegisterFunctionHandler(6, s.handleWriteSingleRegister)

	// 注册写入多个线圈(0x0F)的处理程序
	s.server.RegisterFunctionHandler(15, s.handleWriteMultipleCoils)

	// 注册写入多个寄存器(0x10)的处理程序
	s.server.RegisterFunctionHandler(16, s.handleWriteMultipleRegisters)

	// 根据配置类型启动监听器
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

// handleReadCoils 处理功能码0x01
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

	// 从缓存中读取线圈值
	result, err := s.readCoils(startAddr, quantity)
	if err != nil {
		s.lc.Error(fmt.Sprintf("Read coils error: %s", err.Error()))
		return nil, &mbserver.SlaveDeviceFailure
	}

	return result, &mbserver.Success
}

// handleReadDiscreteInputs 处理功能码0x02
func (s *ModbusServer) handleReadDiscreteInputs(srv *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	// 在此实现中,离散输入的处理方式与线圈相同
	return s.handleReadCoils(srv, frame)
}

// handleReadHoldingRegisters 处理功能码0x03
func (s *ModbusServer) handleReadHoldingRegisters(srv *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	data := frame.GetData()
	if len(data) < 4 {
		return nil, &mbserver.IllegalDataValue
	}

	startAddr := uint16(data[0])<<8 | uint16(data[1])
	quantity := uint16(data[2])<<8 | uint16(data[3])

	s.lc.Debug(fmt.Sprintf("Read holding registers: addr=%d, quantity=%d", startAddr, quantity))

	// 从缓存中读取
	result, err := s.readRegisters(startAddr, quantity)
	if err != nil {
		s.lc.Error(fmt.Sprintf("Read registers error: %s", err.Error()))
		return nil, &mbserver.SlaveDeviceFailure
	}

	return result, &mbserver.Success
}

// handleReadInputRegisters 处理功能码0x04
func (s *ModbusServer) handleReadInputRegisters(srv *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	// 在此实现中,输入寄存器的处理方式与保持寄存器相同
	return s.handleReadHoldingRegisters(srv, frame)
}

// readCoils 从映射管理器缓存中读取线圈值
func (s *ModbusServer) readCoils(startAddr uint16, quantity uint16) ([]byte, error) {
	// 计算字节数(每字节8个线圈,向上取整)
	byteCount := (quantity + 7) / 8

	// 构建响应:字节数+线圈值
	result := make([]byte, 1+byteCount)
	result[0] = byte(byteCount)

	// 读取每个线圈值并打包成字节
	for i := uint16(0); i < quantity; i++ {
		addr := startAddr + i
		cachedData, ok := s.mappingManager.GetCachedValue(addr)

		var bitValue bool
		if ok && cachedData != nil {
			// 将缓存值转换为布尔值
			bitValue = s.valueToBool(cachedData.Value)
		}

		// 将位打包到字节中
		if bitValue {
			byteIndex := i / 8
			bitIndex := i % 8
			result[1+byteIndex] |= (1 << bitIndex)
		}
	}

	return result, nil
}

// valueToBool 将各种值类型转换为布尔值
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

// readRegisters 从映射管理器缓存中读取寄存器值
func (s *ModbusServer) readRegisters(startAddr uint16, quantity uint16) ([]byte, error) {
	// 获取所有请求地址的缓存数据
	cachedData, err := s.mappingManager.GetCachedRegisters(startAddr, quantity)
	if err != nil {
		return nil, err
	}

	// 构建响应:字节数+寄存器值
	result := make([]byte, 1+quantity*2)
	result[0] = byte(quantity * 2) // 字节数

	offset := 1
	for i := uint16(0); i < quantity; i++ {
		data := cachedData[i]
		if data == nil {
			// 该地址没有数据,返回零
			result[offset] = 0
			result[offset+1] = 0
		} else {
			// 将值转换为字节
			bytes, err := s.converter.ToRegisters(data.Value, data.ValueType, data.Scale, data.Offset)
			if err != nil {
				s.lc.Warn(fmt.Sprintf("Conversion error for addr %d: %s", startAddr+i, err.Error()))
				result[offset] = 0
				result[offset+1] = 0
			} else {
				// 复制前2个字节(一个寄存器)
				if len(bytes) >= 2 {
					copy(result[offset:offset+2], bytes[:2])
				}
			}
		}
		offset += 2
	}

	return result, nil
}

// handleWriteSingleCoil 处理功能码0x05
func (s *ModbusServer) handleWriteSingleCoil(srv *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	data := frame.GetData()
	if len(data) < 4 {
		return nil, &mbserver.IllegalDataValue
	}

	addr := uint16(data[0])<<8 | uint16(data[1])
	value := uint16(data[2])<<8 | uint16(data[3])

	// 值必须为0x0000(关)或0xFF00(开)
	if value != 0x0000 && value != 0xFF00 {
		return nil, &mbserver.IllegalDataValue
	}

	s.lc.Debug(fmt.Sprintf("Write single coil: addr=%d, value=0x%04X", addr, value))

	// 检查该地址是否有映射
	mapping, ok := s.mappingManager.GetMappingByAddress(addr)
	if !ok {
		s.lc.Warn(fmt.Sprintf("No mapping for address %d", addr))
		return nil, &mbserver.IllegalDataAddress
	}

	// 检查是否可写
	if mapping.SouthResource != nil && mapping.SouthResource.ReadWrite == "R" {
		s.lc.Warn(fmt.Sprintf("Address %d is read-only", addr))
		return nil, &mbserver.IllegalDataAddress
	}

	// 回显请求数据作为成功响应
	return data, &mbserver.Success
}

// handleWriteSingleRegister 处理功能码0x06
func (s *ModbusServer) handleWriteSingleRegister(srv *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	data := frame.GetData()
	if len(data) < 4 {
		return nil, &mbserver.IllegalDataValue
	}

	addr := uint16(data[0])<<8 | uint16(data[1])
	value := uint16(data[2])<<8 | uint16(data[3])

	s.lc.Debug(fmt.Sprintf("Write single register: addr=%d, value=%d", addr, value))

	// 检查该地址是否有映射
	mapping, ok := s.mappingManager.GetMappingByAddress(addr)
	if !ok {
		s.lc.Warn(fmt.Sprintf("No mapping for address %d", addr))
		return nil, &mbserver.IllegalDataAddress
	}

	// 检查是否可写
	if mapping.SouthResource != nil && mapping.SouthResource.ReadWrite == "R" {
		s.lc.Warn(fmt.Sprintf("Address %d is read-only", addr))
		return nil, &mbserver.IllegalDataAddress
	}

	// 回显请求数据作为成功响应
	return data, &mbserver.Success
}

// handleWriteMultipleCoils 处理功能码0x0F (15)
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

	// 验证所有地址都是可写的
	for i := uint16(0); i < quantity; i++ {
		addr := startAddr + i
		mapping, ok := s.mappingManager.GetMappingByAddress(addr)
		if ok && mapping.SouthResource != nil && mapping.SouthResource.ReadWrite == "R" {
			s.lc.Warn(fmt.Sprintf("Address %d is read-only", addr))
			return nil, &mbserver.IllegalDataAddress
		}
	}

	// 返回起始地址和数量作为成功响应
	return data[:4], &mbserver.Success
}

// handleWriteMultipleRegisters 处理功能码0x10
func (s *ModbusServer) handleWriteMultipleRegisters(srv *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	data := frame.GetData()
	if len(data) < 5 {
		return nil, &mbserver.IllegalDataValue
	}

	startAddr := uint16(data[0])<<8 | uint16(data[1])
	quantity := uint16(data[2])<<8 | uint16(data[3])

	s.lc.Debug(fmt.Sprintf("Write multiple registers: addr=%d, quantity=%d", startAddr, quantity))

	// 返回起始地址和数量作为成功响应
	return data[:4], &mbserver.Success
}

// startRTU 启动Modbus RTU服务器
func (s *ModbusServer) startRTU() error {
	// 创建串口配置
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

	// 启动RTU监听器
	err := s.server.ListenRTU(serialConfig)
	if err != nil {
		return fmt.Errorf("failed to start RTU listener: %w", err)
	}

	return nil
}

// Stop 停止Modbus服务器(TCP或RTU)
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

// IsRunning 返回服务器是否正在运行
func (s *ModbusServer) IsRunning() bool {
	return s.running.Load()
}
