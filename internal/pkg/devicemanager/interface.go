package devicemanager

type DeviceManagerInterface interface {
	RegisterDevice() error
	UnregisterDevice() error
	UpdateDeviceConfig() error
	GetDeviceStatus() error
	ListDevices() error
}
