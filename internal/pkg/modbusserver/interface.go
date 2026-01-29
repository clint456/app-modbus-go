package modbusserver

import "context"

// ModbusServerInterface defines the Modbus server operations
type ModbusServerInterface interface {
	// Start starts the Modbus server
	Start(ctx context.Context) error

	// Stop stops the Modbus server
	Stop() error

	// IsRunning returns whether the server is running
	IsRunning() bool
}
