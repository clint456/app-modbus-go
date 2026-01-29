package service

import (
	"app-modbus-go/internal/pkg/config"
	"app-modbus-go/internal/pkg/forwardlog"
	"app-modbus-go/internal/pkg/logger"
	"app-modbus-go/internal/pkg/mappingmanager"
	"app-modbus-go/internal/pkg/modbusserver"
	"app-modbus-go/internal/pkg/mqtt"
	"context"
)

// AppServiceInterface defines the application service operations
type AppServiceInterface interface {
	// Initialize initializes the service with configuration
	Initialize(configPath string) error

	// Run runs the service until stop is called
	Run() error

	// Stop stops the service
	Stop() error

	// GetLoggingClient returns the logging client
	GetLoggingClient() logger.LoggingClient

	// GetMappingManager returns the mapping manager
	GetMappingManager() mappingmanager.MappingManagerInterface

	// GetModbusServer returns the Modbus server
	GetModbusServer() modbusserver.ModbusServerInterface

	// GetMQTTClient returns the MQTT client manager
	GetMQTTClient() *mqtt.ClientManager

	// GetForwardLogManager returns the forward log manager
	GetForwardLogManager() *forwardlog.Manager

	// GetAppConfig returns the application configuration
	GetAppConfig() *config.AppConfig

	// GetContext returns the service context
	GetContext() context.Context
}
