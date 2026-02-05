package modbusserver

import (
	"app-modbus-go/internal/pkg/logger"
	"app-modbus-go/internal/pkg/mappingmanager"
	"fmt"
)

// ReadResult 表示一次Modbus读取的结果
type ReadResult struct {
	Data          []byte                            // Modbus响应数据
	ForwardedData map[string]map[string]interface{} // 按设备分组的转发数据: deviceName -> {resourceName: value}
}

// RegisterReader 处理Modbus寄存器读取
type RegisterReader struct {
	mappingManager mappingmanager.MappingManagerInterface
	converter      *Converter
	lc             logger.LoggingClient
}

// NewRegisterReader 创建新的寄存器读取器
func NewRegisterReader(
	mm mappingmanager.MappingManagerInterface,
	conv *Converter,
	lc logger.LoggingClient,
) *RegisterReader {
	return &RegisterReader{
		mappingManager: mm,
		converter:      conv,
		lc:             lc,
	}
}

// ReadHoldingRegisters 读取保持寄存器 (功能码 0x03)
func (r *RegisterReader) ReadHoldingRegisters(startAddr uint16, quantity uint16) (*ReadResult, error) {
	return r.readRegisters(startAddr, quantity, "HoldingRegisters")
}

// ReadInputRegisters 读取输入寄存器 (功能码 0x04)
func (r *RegisterReader) ReadInputRegisters(startAddr uint16, quantity uint16) (*ReadResult, error) {
	return r.readRegisters(startAddr, quantity, "InputRegisters")
}

// readRegisters 通用寄存器读取逻辑
func (r *RegisterReader) readRegisters(startAddr uint16, quantity uint16, regType string) (*ReadResult, error) {
	r.lc.Debug(fmt.Sprintf("[%s] 读取寄存器 - 起始地址:%d, 数量:%d", regType, startAddr, quantity))

	// 构建响应: 字节数 + 寄存器值
	result := &ReadResult{
		Data:          make([]byte, 1+quantity*2),
		ForwardedData: make(map[string]map[string]interface{}),
	}
	result.Data[0] = byte(quantity * 2)

	offset := 1
	currentReg := uint16(0)

	for currentReg < quantity {
		queryAddr := startAddr + currentReg
		data, ok := r.mappingManager.GetCachedValue(queryAddr)

		if !ok || data == nil {
			// 无缓存数据，返回零值
			result.Data[offset] = 0
			result.Data[offset+1] = 0
			offset += 2
			currentReg++
			continue
		}

		// 计算该数据类型需要的寄存器数量
		registerCount := r.converter.GetRegisterCount(data.ValueType)

		// 将值转换为字节
		bytes, err := r.converter.ToRegisters(data.Value, data.ValueType, data.Scale, data.Offset)
		if err != nil {
			r.lc.Warn(fmt.Sprintf("[%s] 地址 %d: 类型转换失败 - %s", regType, queryAddr, err.Error()))
			result.Data[offset] = 0
			result.Data[offset+1] = 0
			offset += 2
			currentReg++
			continue
		}

		// 计算实际需要复制的寄存器数（不超过剩余空间）
		remainingRegs := quantity - currentReg
		regsToFill := uint16(registerCount)
		if regsToFill > remainingRegs {
			regsToFill = remainingRegs
		}
		bytesToCopy := int(regsToFill * 2)

		// 复制数据
		if len(bytes) >= bytesToCopy {
			copy(result.Data[offset:offset+bytesToCopy], bytes[:bytesToCopy])

			// 记录成功读取的数据
			r.collectForwardData(result.ForwardedData, data.NorthDevName, data.ResourceName, data.Value)
		} else {
			// 字节数不足，填充零值
			for j := 0; j < bytesToCopy; j++ {
				result.Data[offset+j] = 0
			}
		}

		offset += bytesToCopy
		currentReg += regsToFill
	}

	r.lc.Debug(fmt.Sprintf("[%s] 完成读取 - 响应字节数:%d, 转发设备数:%d",
		regType, len(result.Data), len(result.ForwardedData)))
	return result, nil
}

// ReadCoils 读取线圈 (功能码 0x01)
func (r *RegisterReader) ReadCoils(startAddr uint16, quantity uint16) (*ReadResult, error) {
	return r.readBits(startAddr, quantity, "Coils")
}

// ReadDiscreteInputs 读取离散输入 (功能码 0x02)
func (r *RegisterReader) ReadDiscreteInputs(startAddr uint16, quantity uint16) (*ReadResult, error) {
	return r.readBits(startAddr, quantity, "DiscreteInputs")
}

// readBits 通用位读取逻辑（线圈和离散输入）
func (r *RegisterReader) readBits(startAddr uint16, quantity uint16, bitType string) (*ReadResult, error) {
	r.lc.Debug(fmt.Sprintf("[%s] 读取位数据 - 起始地址:%d, 数量:%d", bitType, startAddr, quantity))

	// 计算字节数（每字节8位，向上取整）
	byteCount := (quantity + 7) / 8

	result := &ReadResult{
		Data:          make([]byte, 1+byteCount),
		ForwardedData: make(map[string]map[string]interface{}),
	}
	result.Data[0] = byte(byteCount)

	for i := uint16(0); i < quantity; i++ {
		addr := startAddr + i
		data, ok := r.mappingManager.GetCachedValue(addr)

		var bitValue bool
		if ok && data != nil {
			bitValue = r.valueToBool(data.Value)
			// 记录成功读取的数据
			r.collectForwardData(result.ForwardedData, data.NorthDevName, data.ResourceName, data.Value)
		}

		// 将位打包到字节中
		if bitValue {
			byteIndex := i / 8
			bitIndex := i % 8
			result.Data[1+byteIndex] |= (1 << bitIndex)
		}
	}

	r.lc.Debug(fmt.Sprintf("[%s] 完成读取 - 响应字节数:%d, 转发设备数:%d",
		bitType, len(result.Data), len(result.ForwardedData)))
	return result, nil
}

// collectForwardData 收集转发数据（按设备分组）
func (r *RegisterReader) collectForwardData(
	forwardedData map[string]map[string]interface{},
	deviceName string,
	resourceName string,
	value interface{},
) {
	if forwardedData[deviceName] == nil {
		forwardedData[deviceName] = make(map[string]interface{})
	}
	forwardedData[deviceName][resourceName] = value
}

// valueToBool 将各种值类型转换为布尔值
func (r *RegisterReader) valueToBool(value interface{}) bool {
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
