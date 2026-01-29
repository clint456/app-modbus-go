package modbusserver

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestNewConverter(t *testing.T) {
	tests := []struct {
		name      string
		byteOrder ByteOrder
	}{
		{"BigEndian", BigEndian},
		{"LittleEndian", LittleEndian},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewConverter(tt.byteOrder)
			if c == nil {
				t.Fatal("NewConverter returned nil")
			}
			if c.byteOrder != tt.byteOrder {
				t.Errorf("expected byteOrder %v, got %v", tt.byteOrder, c.byteOrder)
			}
		})
	}
}

func TestGetRegisterCount(t *testing.T) {
	c := NewConverter(BigEndian)
	tests := []struct {
		valueType string
		expected  int
	}{
		{"bool", 1},
		{"int16", 1},
		{"uint16", 1},
		{"int32", 2},
		{"uint32", 2},
		{"float32", 2},
		{"int64", 4},
		{"uint64", 4},
		{"float64", 4},
		{"unknown", 1}, // default
	}

	for _, tt := range tests {
		t.Run(tt.valueType, func(t *testing.T) {
			count := c.GetRegisterCount(tt.valueType)
			if count != tt.expected {
				t.Errorf("GetRegisterCount(%s) = %d, want %d", tt.valueType, count, tt.expected)
			}
		})
	}
}

func TestApplyScaleOffset(t *testing.T) {
	c := NewConverter(BigEndian)
	tests := []struct {
		name     string
		value    interface{}
		scale    float64
		offset   float64
		expected interface{}
	}{
		{"float64 with scale", float64(100), 10.0, 0, float64(10)},
		{"float64 with offset", float64(100), 1.0, 50, float64(50)},
		{"float64 with both", float64(100), 2.0, 20, float64(40)},
		{"int with scale", int(50), 5.0, 0, float64(10)},
		{"uint16 with scale", uint16(200), 10.0, 0, float64(20)},
		{"zero scale defaults to 1", float64(100), 0, 0, float64(100)},
		{"bool true", true, 1.0, 0, int16(1)},
		{"bool false", false, 1.0, 0, int16(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.applyScaleOffset(tt.value, tt.scale, tt.offset)
			if result != tt.expected {
				t.Errorf("applyScaleOffset(%v, %v, %v) = %v, want %v",
					tt.value, tt.scale, tt.offset, result, tt.expected)
			}
		})
	}
}

func TestBoolToBytes(t *testing.T) {
	tests := []struct {
		name      string
		converter *Converter
		value     interface{}
		expected  []byte
		wantErr   bool
	}{
		{"true", NewConverter(BigEndian), true, []byte{0xFF, 0x00}, false},
		{"false", NewConverter(BigEndian), false, []byte{0x00, 0x00}, false},
		{"int non-zero", NewConverter(BigEndian), int(1), []byte{0xFF, 0x00}, false},
		{"int zero", NewConverter(BigEndian), int(0), []byte{0x00, 0x00}, false},
		{"float64 non-zero", NewConverter(BigEndian), float64(1.5), []byte{0xFF, 0x00}, false},
		{"invalid type", NewConverter(BigEndian), "string", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.converter.boolToBytes(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("boolToBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !bytesEqual(result, tt.expected) {
				t.Errorf("boolToBytes() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestInt16ToBytes(t *testing.T) {
	tests := []struct {
		name      string
		converter *Converter
		value     interface{}
		expected  []byte
		wantErr   bool
	}{
		{"positive BigEndian", NewConverter(BigEndian), int16(1000), []byte{0x03, 0xE8}, false},
		{"positive LittleEndian", NewConverter(LittleEndian), int16(1000), []byte{0xE8, 0x03}, false},
		{"negative BigEndian", NewConverter(BigEndian), int16(-1000), []byte{0xFC, 0x18}, false},
		{"zero", NewConverter(BigEndian), int16(0), []byte{0x00, 0x00}, false},
		{"from int", NewConverter(BigEndian), int(500), []byte{0x01, 0xF4}, false},
		{"from float64", NewConverter(BigEndian), float64(250.7), []byte{0x00, 0xFA}, false},
		{"invalid type", NewConverter(BigEndian), "string", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.converter.int16ToBytes(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("int16ToBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !bytesEqual(result, tt.expected) {
				t.Errorf("int16ToBytes() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestUint16ToBytes(t *testing.T) {
	tests := []struct {
		name      string
		converter *Converter
		value     interface{}
		expected  []byte
		wantErr   bool
	}{
		{"BigEndian", NewConverter(BigEndian), uint16(1000), []byte{0x03, 0xE8}, false},
		{"LittleEndian", NewConverter(LittleEndian), uint16(1000), []byte{0xE8, 0x03}, false},
		{"zero", NewConverter(BigEndian), uint16(0), []byte{0x00, 0x00}, false},
		{"max value", NewConverter(BigEndian), uint16(65535), []byte{0xFF, 0xFF}, false},
		{"from int", NewConverter(BigEndian), int(500), []byte{0x01, 0xF4}, false},
		{"invalid type", NewConverter(BigEndian), "string", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.converter.uint16ToBytes(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("uint16ToBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !bytesEqual(result, tt.expected) {
				t.Errorf("uint16ToBytes() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestInt32ToBytes(t *testing.T) {
	tests := []struct {
		name      string
		converter *Converter
		value     interface{}
		expected  []byte
		wantErr   bool
	}{
		{"positive BigEndian", NewConverter(BigEndian), int32(100000), []byte{0x00, 0x01, 0x86, 0xA0}, false},
		{"positive LittleEndian", NewConverter(LittleEndian), int32(100000), []byte{0xA0, 0x86, 0x01, 0x00}, false},
		{"negative", NewConverter(BigEndian), int32(-100000), []byte{0xFF, 0xFE, 0x79, 0x60}, false},
		{"zero", NewConverter(BigEndian), int32(0), []byte{0x00, 0x00, 0x00, 0x00}, false},
		{"invalid type", NewConverter(BigEndian), "string", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.converter.int32ToBytes(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("int32ToBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !bytesEqual(result, tt.expected) {
				t.Errorf("int32ToBytes() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestUint32ToBytes(t *testing.T) {
	tests := []struct {
		name      string
		converter *Converter
		value     interface{}
		expected  []byte
		wantErr   bool
	}{
		{"BigEndian", NewConverter(BigEndian), uint32(100000), []byte{0x00, 0x01, 0x86, 0xA0}, false},
		{"LittleEndian", NewConverter(LittleEndian), uint32(100000), []byte{0xA0, 0x86, 0x01, 0x00}, false},
		{"zero", NewConverter(BigEndian), uint32(0), []byte{0x00, 0x00, 0x00, 0x00}, false},
		{"max value", NewConverter(BigEndian), uint32(4294967295), []byte{0xFF, 0xFF, 0xFF, 0xFF}, false},
		{"invalid type", NewConverter(BigEndian), "string", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.converter.uint32ToBytes(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("uint32ToBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !bytesEqual(result, tt.expected) {
				t.Errorf("uint32ToBytes() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFloat32ToBytes(t *testing.T) {
	tests := []struct {
		name      string
		converter *Converter
		value     interface{}
		wantErr   bool
	}{
		{"positive BigEndian", NewConverter(BigEndian), float32(123.456), false},
		{"positive LittleEndian", NewConverter(LittleEndian), float32(123.456), false},
		{"negative", NewConverter(BigEndian), float32(-123.456), false},
		{"zero", NewConverter(BigEndian), float32(0), false},
		{"from float64", NewConverter(BigEndian), float64(123.456), false},
		{"from int", NewConverter(BigEndian), int(123), false},
		{"invalid type", NewConverter(BigEndian), "string", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.converter.float32ToBytes(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("float32ToBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(result) != 4 {
					t.Errorf("float32ToBytes() returned %d bytes, want 4", len(result))
				}
				// Verify round-trip conversion
				var bits uint32
				if tt.converter.byteOrder == BigEndian {
					bits = binary.BigEndian.Uint32(result)
				} else {
					bits = binary.LittleEndian.Uint32(result)
				}
				recovered := math.Float32frombits(bits)
				var expected float32
				switch v := tt.value.(type) {
				case float32:
					expected = v
				case float64:
					expected = float32(v)
				case int:
					expected = float32(v)
				}
				if math.Abs(float64(recovered-expected)) > 0.001 {
					t.Errorf("float32ToBytes() round-trip failed: got %v, want %v", recovered, expected)
				}
			}
		})
	}
}

func TestFloat64ToBytes(t *testing.T) {
	tests := []struct {
		name      string
		converter *Converter
		value     interface{}
		wantErr   bool
	}{
		{"positive BigEndian", NewConverter(BigEndian), float64(123.456789), false},
		{"positive LittleEndian", NewConverter(LittleEndian), float64(123.456789), false},
		{"negative", NewConverter(BigEndian), float64(-123.456789), false},
		{"zero", NewConverter(BigEndian), float64(0), false},
		{"from float32", NewConverter(BigEndian), float32(123.456), false},
		{"from int", NewConverter(BigEndian), int(123), false},
		{"invalid type", NewConverter(BigEndian), "string", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.converter.float64ToBytes(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("float64ToBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(result) != 8 {
					t.Errorf("float64ToBytes() returned %d bytes, want 8", len(result))
				}
				// Verify round-trip conversion
				var bits uint64
				if tt.converter.byteOrder == BigEndian {
					bits = binary.BigEndian.Uint64(result)
				} else {
					bits = binary.LittleEndian.Uint64(result)
				}
				recovered := math.Float64frombits(bits)
				var expected float64
				switch v := tt.value.(type) {
				case float64:
					expected = v
				case float32:
					expected = float64(v)
				case int:
					expected = float64(v)
				}
				if math.Abs(recovered-expected) > 0.000001 {
					t.Errorf("float64ToBytes() round-trip failed: got %v, want %v", recovered, expected)
				}
			}
		})
	}
}

func TestInt64ToBytes(t *testing.T) {
	tests := []struct {
		name      string
		converter *Converter
		value     interface{}
		expected  []byte
		wantErr   bool
	}{
		{"positive BigEndian", NewConverter(BigEndian), int64(1000000000),
			[]byte{0x00, 0x00, 0x00, 0x00, 0x3B, 0x9A, 0xCA, 0x00}, false},
		{"positive LittleEndian", NewConverter(LittleEndian), int64(1000000000),
			[]byte{0x00, 0xCA, 0x9A, 0x3B, 0x00, 0x00, 0x00, 0x00}, false},
		{"negative", NewConverter(BigEndian), int64(-1000000000),
			[]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xC4, 0x65, 0x36, 0x00}, false},
		{"zero", NewConverter(BigEndian), int64(0),
			[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, false},
		{"invalid type", NewConverter(BigEndian), "string", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.converter.int64ToBytes(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("int64ToBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !bytesEqual(result, tt.expected) {
				t.Errorf("int64ToBytes() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestUint64ToBytes(t *testing.T) {
	tests := []struct {
		name      string
		converter *Converter
		value     interface{}
		expected  []byte
		wantErr   bool
	}{
		{"BigEndian", NewConverter(BigEndian), uint64(1000000000),
			[]byte{0x00, 0x00, 0x00, 0x00, 0x3B, 0x9A, 0xCA, 0x00}, false},
		{"LittleEndian", NewConverter(LittleEndian), uint64(1000000000),
			[]byte{0x00, 0xCA, 0x9A, 0x3B, 0x00, 0x00, 0x00, 0x00}, false},
		{"zero", NewConverter(BigEndian), uint64(0),
			[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, false},
		{"invalid type", NewConverter(BigEndian), "string", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.converter.uint64ToBytes(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("uint64ToBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !bytesEqual(result, tt.expected) {
				t.Errorf("uint64ToBytes() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestToRegisters(t *testing.T) {
	c := NewConverter(BigEndian)
	tests := []struct {
		name      string
		value     interface{}
		valueType string
		scale     float64
		offset    float64
		wantLen   int
		wantErr   bool
	}{
		{"bool", true, "bool", 1.0, 0, 2, false},
		{"int16", int16(1000), "int16", 1.0, 0, 2, false},
		{"uint16", uint16(1000), "uint16", 1.0, 0, 2, false},
		{"int32", int32(100000), "int32", 1.0, 0, 4, false},
		{"uint32", uint32(100000), "uint32", 1.0, 0, 4, false},
		{"float32", float32(123.456), "float32", 1.0, 0, 4, false},
		{"float64", float64(123.456), "float64", 1.0, 0, 8, false},
		{"int64", int64(1000000), "int64", 1.0, 0, 8, false},
		{"uint64", uint64(1000000), "uint64", 1.0, 0, 8, false},
		{"with scale", float64(100), "uint16", 10.0, 0, 2, false},
		{"with offset", float64(100), "uint16", 1.0, 50, 2, false},
		{"unknown type defaults to uint16", uint16(500), "unknown", 1.0, 0, 2, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := c.ToRegisters(tt.value, tt.valueType, tt.scale, tt.offset)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToRegisters() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(result) != tt.wantLen {
				t.Errorf("ToRegisters() returned %d bytes, want %d", len(result), tt.wantLen)
			}
		})
	}
}

func TestFromBytes(t *testing.T) {
	tests := []struct {
		name      string
		converter *Converter
		data      []byte
		valueType string
		scale     float64
		offset    float64
		expected  interface{}
		wantErr   bool
	}{
		{"bool true", NewConverter(BigEndian), []byte{0xFF, 0x00}, "bool", 1.0, 0, true, false},
		{"bool false", NewConverter(BigEndian), []byte{0x00, 0x00}, "bool", 1.0, 0, false, false},
		{"int16 BigEndian", NewConverter(BigEndian), []byte{0x03, 0xE8}, "int16", 1.0, 0, float64(1000), false},
		{"int16 LittleEndian", NewConverter(LittleEndian), []byte{0xE8, 0x03}, "int16", 1.0, 0, float64(1000), false},
		{"uint16 BigEndian", NewConverter(BigEndian), []byte{0x03, 0xE8}, "uint16", 1.0, 0, float64(1000), false},
		{"int32 BigEndian", NewConverter(BigEndian), []byte{0x00, 0x01, 0x86, 0xA0}, "int32", 1.0, 0, float64(100000), false},
		{"uint32 BigEndian", NewConverter(BigEndian), []byte{0x00, 0x01, 0x86, 0xA0}, "uint32", 1.0, 0, float64(100000), false},
		{"with scale", NewConverter(BigEndian), []byte{0x00, 0x0A}, "uint16", 10.0, 0, float64(100), false},
		{"with offset", NewConverter(BigEndian), []byte{0x00, 0x32}, "uint16", 1.0, 50, float64(100), false},
		{"insufficient data bool", NewConverter(BigEndian), []byte{0x00}, "bool", 1.0, 0, nil, true},
		{"insufficient data int16", NewConverter(BigEndian), []byte{0x00}, "int16", 1.0, 0, nil, true},
		{"insufficient data int32", NewConverter(BigEndian), []byte{0x00, 0x01}, "int32", 1.0, 0, nil, true},
		{"unknown type defaults to uint16", NewConverter(BigEndian), []byte{0x01, 0xF4}, "unknown", 1.0, 0, float64(500), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.converter.FromBytes(tt.data, tt.valueType, tt.scale, tt.offset)
			if (err != nil) != tt.wantErr {
				t.Errorf("FromBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("FromBytes() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRoundTripConversion(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		valueType string
		scale     float64
		offset    float64
	}{
		{"int16", int16(1234), "int16", 1.0, 0},
		{"uint16", uint16(5678), "uint16", 1.0, 0},
		{"int32", int32(123456), "int32", 1.0, 0},
		{"uint32", uint32(987654), "uint32", 1.0, 0},
		{"float32", float32(123.456), "float32", 1.0, 0},
		{"with scale", float64(100), "uint16", 10.0, 0},
		{"with offset", float64(150), "uint16", 1.0, 50},
		{"with both", float64(200), "uint16", 2.0, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewConverter(BigEndian)

			// Convert to bytes
			bytes, err := c.ToRegisters(tt.value, tt.valueType, tt.scale, tt.offset)
			if err != nil {
				t.Fatalf("ToRegisters() error = %v", err)
			}

			// Convert back
			result, err := c.FromBytes(bytes, tt.valueType, tt.scale, tt.offset)
			if err != nil {
				t.Fatalf("FromBytes() error = %v", err)
			}

			// Compare (with tolerance for floating point)
			var expected float64
			switch v := tt.value.(type) {
			case int16:
				expected = float64(v)
			case uint16:
				expected = float64(v)
			case int32:
				expected = float64(v)
			case uint32:
				expected = float64(v)
			case float32:
				expected = float64(v)
			case float64:
				expected = v
			}

			resultFloat, ok := result.(float64)
			if !ok {
				t.Fatalf("FromBytes() returned %T, want float64", result)
			}

			if math.Abs(resultFloat-expected) > 0.01 {
				t.Errorf("Round-trip conversion failed: got %v, want %v", resultFloat, expected)
			}
		})
	}
}

// Helper function to compare byte slices
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
