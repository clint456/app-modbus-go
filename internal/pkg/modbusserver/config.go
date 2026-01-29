package modbusserver

type ModbusTcpConfig struct {
}

type ModbusRtuConfig struct {
}

type ModbusConfig struct {
	ModbusTcp ModbusTcpConfig `yaml:"ModbusTcp"`
	ModbusRtu ModbusRtuConfig `yaml:"ModbusRtu"`
}

type ModbusType uint8

const (
	MODBUS_TCP ModbusType = 0
	MODBUS_UDP ModbusType = 1
)
