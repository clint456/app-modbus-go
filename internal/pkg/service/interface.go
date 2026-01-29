package service

import (
	"app-modbus-go/internal/pkg/config"
	"app-modbus-go/internal/pkg/devicemanager"
	"app-modbus-go/internal/pkg/logger"
	"app-modbus-go/internal/pkg/mappingmanager"
	"app-modbus-go/internal/pkg/modbusserver"
	"app-modbus-go/internal/pkg/register"
)

type AppServiceInterface interface {
	Initialize() error
	Run() error
	Stop() error
	GetLoggingClient() logger.LoggingClient
	GetDeviceManager() devicemanager.DeviceManagerInterface
	GetMappingManager() mappingmanager.MappingManagerInterface
	GetModbusServer() modbusserver.ModbusServerInterface
	GetRegister() register.RegisterInterface
	GetAppConifg() *config.AppConfig
}
