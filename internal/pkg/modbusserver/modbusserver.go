package modbusserver

import "errors"

type ModbusServer struct {
	modbusType ModbusType
	config     ModbusConfig
}

func NewModbusServer(modbusType ModbusType, config ModbusConfig) (*ModbusServer, error) {
	if modbusType == 0 {
		return nil, errors.New("modbusType is illegal")
	}
	if config == nil {
		return nil, errors.New("config is illegal")
	}
	return &ModbusServer{
		modbusType: modbusType,
		config:     config,
	}, nil
}
