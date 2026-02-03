package service

import (
	"app-modbus-go/internal/pkg/logger"
	"app-modbus-go/internal/pkg/mqtt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewAppService tests the NewAppService constructor
func TestNewAppService(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		version     string
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "valid service creation",
			serviceName: "test-service",
			version:     "1.0.0",
			wantErr:     false,
		},
		{
			name:        "empty service name",
			serviceName: "",
			version:     "1.0.0",
			wantErr:     true,
			errMsg:      "please specify service name",
		},
		{
			name:        "empty version",
			serviceName: "test-service",
			version:     "",
			wantErr:     true,
			errMsg:      "please specify service version",
		},
		{
			name:        "both empty",
			serviceName: "",
			version:     "",
			wantErr:     true,
			errMsg:      "please specify service name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := NewAppService(tt.serviceName, tt.version)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, svc)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, svc)

				// Verify the service was created with correct values
				appSvc, ok := svc.(*AppService)
				assert.True(t, ok)
				assert.Equal(t, tt.serviceName, appSvc.appName)
				assert.Equal(t, tt.version, appSvc.version)
			}
		})
	}
}

// TestAppService_GettersBeforeInit tests getter methods before initialization
func TestAppService_GettersBeforeInit(t *testing.T) {
	svc, err := NewAppService("test-service", "1.0.0")
	assert.NoError(t, err)
	assert.NotNil(t, svc)

	// All getters should return nil before initialization
	assert.Nil(t, svc.GetLoggingClient())
	assert.Nil(t, svc.GetMappingManager())
	assert.Nil(t, svc.GetModbusServer())
	assert.Nil(t, svc.GetMQTTClient())
	assert.Nil(t, svc.GetForwardLogManager())
	assert.Nil(t, svc.GetAppConfig())
	assert.Nil(t, svc.GetContext())
}

// TestAppService_HandlePutCommand tests the handlePutCommand method
func TestAppService_HandlePutCommand(t *testing.T) {
	svc, err := NewAppService("test-service", "1.0.0")
	assert.NoError(t, err)

	// Set up logger to avoid nil pointer
	appSvc := svc.(*AppService)
	appSvc.lc = logger.NewClient("INFO")

	tests := []struct {
		name           string
		payload        *mqtt.CommandPayload
		wantStatusCode int
		wantCmdType    string
	}{
		{
			name: "valid PUT command",
			payload: &mqtt.CommandPayload{
				CmdType: "PUT",
				CmdContent: struct {
					NorthDeviceName    string `json:"northDeviceName"`
					NorthResourceName  string `json:"northResourceName"`
					NorthResourceValue string `json:"northResourceValue,omitempty"`
				}{
					NorthDeviceName:    "device1",
					NorthResourceName:  "temperature",
					NorthResourceValue: "25.5",
				},
			},
			wantStatusCode: 200,
			wantCmdType:    "PUT",
		},
		{
			name: "PUT command with empty value",
			payload: &mqtt.CommandPayload{
				CmdType: "PUT",
				CmdContent: struct {
					NorthDeviceName    string `json:"northDeviceName"`
					NorthResourceName  string `json:"northResourceName"`
					NorthResourceValue string `json:"northResourceValue,omitempty"`
				}{
					NorthDeviceName:    "device2",
					NorthResourceName:  "status",
					NorthResourceValue: "",
				},
			},
			wantStatusCode: 200,
			wantCmdType:    "PUT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := appSvc.handlePutCommand(tt.payload)

			assert.NotNil(t, resp)
			assert.Equal(t, tt.wantStatusCode, resp.StatusCode)
			assert.Equal(t, tt.wantCmdType, resp.CmdType)
			assert.Equal(t, tt.payload.CmdContent.NorthDeviceName, resp.CmdContent.NorthDeviceName)
			assert.Equal(t, tt.payload.CmdContent.NorthResourceName, resp.CmdContent.NorthResourceName)
			assert.Equal(t, tt.payload.CmdContent.NorthResourceValue, resp.CmdContent.NorthResourceValue)
		})
	}
}
