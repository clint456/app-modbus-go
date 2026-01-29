package modbusserver

type ModbusServerInterface interface {
	CreateModbusServer(modbusType ModbusType, config ModbusConfig) (error, ModbusServer)
	StartModbusServer() error
	StopModbusServer() error
	UpdateModbusServerConfig() error
}
