package modbusserver

import (
	"encoding/binary"
	"fmt"
	"math"
)

// ByteOrder defines the byte ordering for multi-byte values
type ByteOrder int

const (
	BigEndian ByteOrder = iota
	LittleEndian
)

// Converter handles data type conversions between Go types and Modbus registers
type Converter struct {
	byteOrder ByteOrder
}

// NewConverter creates a new converter with the specified byte order
func NewConverter(order ByteOrder) *Converter {
	return &Converter{byteOrder: order}
}

// ToRegisters converts a value to Modbus register bytes based on value type
func (c *Converter) ToRegisters(value interface{}, valueType string, scale, offset float64) ([]byte, error) {
	// Apply scale and offset to numeric values
	scaledValue := c.applyScaleOffset(value, scale, offset)

	switch valueType {
	case "bool":
		return c.boolToBytes(scaledValue)
	case "int16":
		return c.int16ToBytes(scaledValue)
	case "uint16":
		return c.uint16ToBytes(scaledValue)
	case "int32":
		return c.int32ToBytes(scaledValue)
	case "uint32":
		return c.uint32ToBytes(scaledValue)
	case "float32":
		return c.float32ToBytes(scaledValue)
	case "float64":
		return c.float64ToBytes(scaledValue)
	case "int64":
		return c.int64ToBytes(scaledValue)
	case "uint64":
		return c.uint64ToBytes(scaledValue)
	default:
		// Default to uint16
		return c.uint16ToBytes(scaledValue)
	}
}

// GetRegisterCount returns the number of registers needed for a value type
func (c *Converter) GetRegisterCount(valueType string) int {
	switch valueType {
	case "bool", "int16", "uint16":
		return 1
	case "int32", "uint32", "float32":
		return 2
	case "float64", "int64", "uint64":
		return 4
	default:
		return 1
	}
}

// applyScaleOffset applies scale and offset to a value
func (c *Converter) applyScaleOffset(value interface{}, scale, offset float64) interface{} {
	if scale == 0 {
		scale = 1
	}

	var floatVal float64
	switch v := value.(type) {
	case float64:
		floatVal = v
	case float32:
		floatVal = float64(v)
	case int:
		floatVal = float64(v)
	case int16:
		floatVal = float64(v)
	case int32:
		floatVal = float64(v)
	case int64:
		floatVal = float64(v)
	case uint:
		floatVal = float64(v)
	case uint16:
		floatVal = float64(v)
	case uint32:
		floatVal = float64(v)
	case uint64:
		floatVal = float64(v)
	case bool:
		if v {
			return int16(1)
		}
		return int16(0)
	default:
		return value
	}

	// Apply: result = (value - offset) / scale
	return (floatVal - offset) / scale
}

// putUint16 writes a uint16 value to bytes with the configured byte order
func (c *Converter) putUint16(result []byte, v uint16) {
	if c.byteOrder == BigEndian {
		binary.BigEndian.PutUint16(result, v)
	} else {
		binary.LittleEndian.PutUint16(result, v)
	}
}

// putUint32 writes a uint32 value to bytes with the configured byte order
func (c *Converter) putUint32(result []byte, v uint32) {
	if c.byteOrder == BigEndian {
		binary.BigEndian.PutUint32(result, v)
	} else {
		binary.LittleEndian.PutUint32(result, v)
	}
}

// putUint64 writes a uint64 value to bytes with the configured byte order
func (c *Converter) putUint64(result []byte, v uint64) {
	if c.byteOrder == BigEndian {
		binary.BigEndian.PutUint64(result, v)
	} else {
		binary.LittleEndian.PutUint64(result, v)
	}
}

// getUint16 reads a uint16 value from bytes with the configured byte order
func (c *Converter) getUint16(data []byte) uint16 {
	if c.byteOrder == BigEndian {
		return binary.BigEndian.Uint16(data)
	}
	return binary.LittleEndian.Uint16(data)
}

// getUint32 reads a uint32 value from bytes with the configured byte order
func (c *Converter) getUint32(data []byte) uint32 {
	if c.byteOrder == BigEndian {
		return binary.BigEndian.Uint32(data)
	}
	return binary.LittleEndian.Uint32(data)
}

// getUint64 reads a uint64 value from bytes with the configured byte order
func (c *Converter) getUint64(data []byte) uint64 {
	if c.byteOrder == BigEndian {
		return binary.BigEndian.Uint64(data)
	}
	return binary.LittleEndian.Uint64(data)
}

func (c *Converter) boolToBytes(value interface{}) ([]byte, error) {
	var v bool
	switch val := value.(type) {
	case bool:
		v = val
	case int, int16, int32, int64:
		v = val != 0
	case uint, uint16, uint32, uint64:
		v = val != 0
	case float64:
		v = val != 0
	default:
		return nil, fmt.Errorf("cannot convert %T to bool", value)
	}

	result := make([]byte, 2)
	if v {
		result[0] = 0xFF
		result[1] = 0x00
	}
	return result, nil
}

func (c *Converter) int16ToBytes(value interface{}) ([]byte, error) {
	var v int16
	switch val := value.(type) {
	case int16:
		v = val
	case int:
		v = int16(val)
	case int32:
		v = int16(val)
	case int64:
		v = int16(val)
	case float64:
		v = int16(val)
	case uint16:
		v = int16(val)
	default:
		return nil, fmt.Errorf("cannot convert %T to int16", value)
	}

	result := make([]byte, 2)
	c.putUint16(result, uint16(v))
	return result, nil
}

func (c *Converter) uint16ToBytes(value interface{}) ([]byte, error) {
	var v uint16
	switch val := value.(type) {
	case uint16:
		v = val
	case int:
		v = uint16(val)
	case int16:
		v = uint16(val)
	case int32:
		v = uint16(val)
	case int64:
		v = uint16(val)
	case uint:
		v = uint16(val)
	case uint32:
		v = uint16(val)
	case uint64:
		v = uint16(val)
	case float64:
		v = uint16(val)
	default:
		return nil, fmt.Errorf("cannot convert %T to uint16", value)
	}

	result := make([]byte, 2)
	c.putUint16(result, v)
	return result, nil
}

func (c *Converter) int32ToBytes(value interface{}) ([]byte, error) {
	var v int32
	switch val := value.(type) {
	case int32:
		v = val
	case int:
		v = int32(val)
	case int16:
		v = int32(val)
	case int64:
		v = int32(val)
	case float64:
		v = int32(val)
	default:
		return nil, fmt.Errorf("cannot convert %T to int32", value)
	}

	result := make([]byte, 4)
	c.putUint32(result, uint32(v))
	return result, nil
}

func (c *Converter) uint32ToBytes(value interface{}) ([]byte, error) {
	var v uint32
	switch val := value.(type) {
	case uint32:
		v = val
	case int:
		v = uint32(val)
	case int32:
		v = uint32(val)
	case int64:
		v = uint32(val)
	case uint:
		v = uint32(val)
	case uint64:
		v = uint32(val)
	case float64:
		v = uint32(val)
	default:
		return nil, fmt.Errorf("cannot convert %T to uint32", value)
	}

	result := make([]byte, 4)
	c.putUint32(result, v)
	return result, nil
}

func (c *Converter) float32ToBytes(value interface{}) ([]byte, error) {
	var v float32
	switch val := value.(type) {
	case float32:
		v = val
	case float64:
		v = float32(val)
	case int:
		v = float32(val)
	case int32:
		v = float32(val)
	case int64:
		v = float32(val)
	default:
		return nil, fmt.Errorf("cannot convert %T to float32", value)
	}

	result := make([]byte, 4)
	bits := math.Float32bits(v)
	c.putUint32(result, bits)
	return result, nil
}

func (c *Converter) float64ToBytes(value interface{}) ([]byte, error) {
	var v float64
	switch val := value.(type) {
	case float64:
		v = val
	case float32:
		v = float64(val)
	case int:
		v = float64(val)
	case int64:
		v = float64(val)
	default:
		return nil, fmt.Errorf("cannot convert %T to float64", value)
	}

	result := make([]byte, 8)
	bits := math.Float64bits(v)
	c.putUint64(result, bits)
	return result, nil
}

func (c *Converter) int64ToBytes(value interface{}) ([]byte, error) {
	var v int64
	switch val := value.(type) {
	case int64:
		v = val
	case int:
		v = int64(val)
	case int32:
		v = int64(val)
	case float64:
		v = int64(val)
	default:
		return nil, fmt.Errorf("cannot convert %T to int64", value)
	}

	result := make([]byte, 8)
	c.putUint64(result, uint64(v))
	return result, nil
}

func (c *Converter) uint64ToBytes(value interface{}) ([]byte, error) {
	var v uint64
	switch val := value.(type) {
	case uint64:
		v = val
	case int:
		v = uint64(val)
	case int64:
		v = uint64(val)
	case uint:
		v = uint64(val)
	case uint32:
		v = uint64(val)
	case float64:
		v = uint64(val)
	default:
		return nil, fmt.Errorf("cannot convert %T to uint64", value)
	}

	result := make([]byte, 8)
	c.putUint64(result, v)
	return result, nil
}

// FromBytes converts Modbus register bytes back to a value based on value type
func (c *Converter) FromBytes(data []byte, valueType string, scale, offset float64) (interface{}, error) {
	if scale == 0 {
		scale = 1
	}

	var rawValue float64

	switch valueType {
	case "bool":
		if len(data) < 2 {
			return nil, fmt.Errorf("insufficient data for bool")
		}
		return data[0] != 0 || data[1] != 0, nil
	case "int16":
		if len(data) < 2 {
			return nil, fmt.Errorf("insufficient data for int16")
		}
		var v int16
		if c.byteOrder == BigEndian {
			v = int16(binary.BigEndian.Uint16(data))
		} else {
			v = int16(binary.LittleEndian.Uint16(data))
		}
		rawValue = float64(v)
	case "uint16":
		if len(data) < 2 {
			return nil, fmt.Errorf("insufficient data for uint16")
		}
		var v uint16
		if c.byteOrder == BigEndian {
			v = binary.BigEndian.Uint16(data)
		} else {
			v = binary.LittleEndian.Uint16(data)
		}
		rawValue = float64(v)
	case "int32":
		if len(data) < 4 {
			return nil, fmt.Errorf("insufficient data for int32")
		}
		var v int32
		if c.byteOrder == BigEndian {
			v = int32(binary.BigEndian.Uint32(data))
		} else {
			v = int32(binary.LittleEndian.Uint32(data))
		}
		rawValue = float64(v)
	case "uint32":
		if len(data) < 4 {
			return nil, fmt.Errorf("insufficient data for uint32")
		}
		var v uint32
		if c.byteOrder == BigEndian {
			v = binary.BigEndian.Uint32(data)
		} else {
			v = binary.LittleEndian.Uint32(data)
		}
		rawValue = float64(v)
	case "float32":
		if len(data) < 4 {
			return nil, fmt.Errorf("insufficient data for float32")
		}
		var bits uint32
		if c.byteOrder == BigEndian {
			bits = binary.BigEndian.Uint32(data)
		} else {
			bits = binary.LittleEndian.Uint32(data)
		}
		rawValue = float64(math.Float32frombits(bits))
	default:
		// Default to uint16
		if len(data) < 2 {
			return nil, fmt.Errorf("insufficient data")
		}
		var v uint16
		if c.byteOrder == BigEndian {
			v = binary.BigEndian.Uint16(data)
		} else {
			v = binary.LittleEndian.Uint16(data)
		}
		rawValue = float64(v)
	}

	// Apply inverse: value = raw * scale + offset
	return rawValue*scale + offset, nil
}
