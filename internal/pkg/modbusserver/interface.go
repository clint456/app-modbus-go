package modbusserver

type ModbusServerInterface interface {
	CreateModbusServer() error
	StartModbusServer() error
	StopModbusServer() error
	UpdateModbusServerConfig() error
}
