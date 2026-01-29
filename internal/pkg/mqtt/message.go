package mqtt

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Message type constants
const (
	TypeHeartbeat           = 1 // 心跳
	TypeQueryDevice         = 2 // 查询设备属性
	TypeDeviceAttributePush = 3 // 下发设备属性
	TypeSensorData          = 4 // 传感器数据
	TypeForwardLog          = 5 // 转发日志
	TypeCommand             = 6 // 命令下发
)

// MQTTMessage represents the base message structure
type MQTTMessage struct {
	RequestID string      `json:"requestId"`
	Version   string      `json:"version"`
	Type      int         `json:"type"`
	Timestamp int64       `json:"timestamp"`
	Payload   interface{} `json:"payload"`
}

// MQTTResponse represents a response message with code and msg
type MQTTResponse struct {
	RequestID string      `json:"requestId"`
	Version   string      `json:"version"`
	Type      int         `json:"type"`
	Timestamp int64       `json:"timestamp"`
	Code      int         `json:"code"`
	Msg       string      `json:"msg"`
	Payload   interface{} `json:"payload"`
}

// NewMessage creates a new MQTTMessage with default values
func NewMessage(msgType int, payload interface{}) *MQTTMessage {
	return &MQTTMessage{
		RequestID: uuid.New().String(),
		Version:   "1.0",
		Type:      msgType,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
	}
}

// NewResponse creates a new MQTTResponse from a request
func NewResponse(requestID string, msgType int, code int, msg string, payload interface{}) *MQTTResponse {
	return &MQTTResponse{
		RequestID: requestID,
		Version:   "1.0",
		Type:      msgType,
		Timestamp: time.Now().UnixMilli(),
		Code:      code,
		Msg:       msg,
		Payload:   payload,
	}
}

// ToJSON serializes the message to JSON bytes
func (m *MQTTMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// ToJSON serializes the response to JSON bytes
func (r *MQTTResponse) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// ParseMessage parses JSON bytes into an MQTTMessage
func ParseMessage(data []byte) (*MQTTMessage, error) {
	var msg MQTTMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse message: %w", err)
	}
	return &msg, nil
}

// ParseResponse parses JSON bytes into an MQTTResponse
func ParseResponse(data []byte) (*MQTTResponse, error) {
	var resp MQTTResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

// ---- Payload Types ----

// HeartbeatPayload for type=1 heartbeat messages
type HeartbeatPayload struct{}

// QueryDevicePayload for type=2 query device request
type QueryDevicePayload struct {
	Cmd string `json:"cmd"` // "0101" for querying device attributes
}

// NorthResource represents a north-side resource definition
type NorthResource struct {
	Name            string  `json:"name"`
	NorthModelName  string  `json:"northModelName"`
	Description     string  `json:"description"`
	ValueType       string  `json:"valueType"` // int16, float32, etc.
	Scale           float64 `json:"scale"`
	OffsetValue     float64 `json:"offsetValue"`
	OtherParameters struct {
		Modbus struct {
			Address uint16 `json:"address"` // Modbus register address
		} `json:"modbus"`
	} `json:"otherParameters"`
}

// SouthResource represents a south-side resource definition
type SouthResource struct {
	Name            string      `json:"name"`
	SouthModelName  string      `json:"southModelName"`
	ReadWrite       string      `json:"readWrite"` // R/W/RW
	ValueType       string      `json:"valueType"`
	Scale           float64     `json:"scale"`
	Offset          float64     `json:"offset"`
	AutoUpload      bool        `json:"autoUpload"`
	OtherParameters interface{} `json:"other_parameters"`
}

// ResourceMapping represents the mapping between north and south resources
type ResourceMapping struct {
	NorthResource *NorthResource `json:"northResource"`
	SouthResource *SouthResource `json:"southResource"`
}

// DeviceMapping represents device level mapping
type DeviceMapping struct {
	NorthDeviceName string             `json:"northDeviceName"`
	Resources       []*ResourceMapping `json:"resources"`
}

// QueryDeviceResponse for type=2 query device response payload
type QueryDeviceResponse struct {
	Cmd    string           `json:"cmd"`
	Result []*DeviceMapping `json:"result"`
}

// DeviceAttributePushPayload for type=3 device attribute push
type DeviceAttributePushPayload struct {
	Devices []*DeviceMapping `json:"devices"`
}

// SensorDataPayload for type=4 sensor data messages
type SensorDataPayload struct {
	NorthDeviceName string                 `json:"northDeviceName"`
	Data            map[string]interface{} `json:"data"`
}

// ForwardLogPayload for type=5 forward log messages
type ForwardLogPayload struct {
	Status          int                    `json:"status"` // 1-success, 0-failure
	NorthDeviceName string                 `json:"northDeviceName"`
	Data            map[string]interface{} `json:"data"`
}

// CommandPayload for type=6 command messages
type CommandPayload struct {
	CmdType    string         `json:"cmdType"` // "GET"/"PUT"
	CmdContent CommandContent `json:"cmdContent"`
}

// CommandContent represents the content of a command
type CommandContent struct {
	NorthDeviceName    string `json:"northDeviceName"`
	NorthResourceName  string `json:"northResourceName"`
	NorthResourceValue string `json:"northResourceValue,omitempty"`
}

// CommandResponse for type=6 command response
type CommandResponsePayload struct {
	CmdType    string                 `json:"cmdType"`
	StatusCode int                    `json:"statusCode"`
	CmdContent CommandResponseContent `json:"cmdContent"`
}

// CommandResponseContent represents the content of a command response
type CommandResponseContent struct {
	NorthDeviceName    string `json:"northDeviceName"`
	NorthResourceName  string `json:"northResourceName"`
	NorthResourceValue string `json:"northResourceValue,omitempty"`
}

// ---- Helper functions for payload extraction ----

// GetSensorDataPayload extracts SensorDataPayload from message
func (m *MQTTMessage) GetSensorDataPayload() (*SensorDataPayload, error) {
	if m.Type != TypeSensorData {
		return nil, fmt.Errorf("message type is not sensor data: %d", m.Type)
	}
	data, err := json.Marshal(m.Payload)
	if err != nil {
		return nil, err
	}
	var payload SensorDataPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

// GetCommandPayload extracts CommandPayload from message
func (m *MQTTMessage) GetCommandPayload() (*CommandPayload, error) {
	if m.Type != TypeCommand {
		return nil, fmt.Errorf("message type is not command: %d", m.Type)
	}
	data, err := json.Marshal(m.Payload)
	if err != nil {
		return nil, err
	}
	var payload CommandPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

// GetQueryDeviceResponse extracts QueryDeviceResponse from response
func (r *MQTTResponse) GetQueryDeviceResponse() (*QueryDeviceResponse, error) {
	if r.Type != TypeQueryDevice {
		return nil, fmt.Errorf("response type is not query device: %d", r.Type)
	}
	data, err := json.Marshal(r.Payload)
	if err != nil {
		return nil, err
	}
	var payload QueryDeviceResponse
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

// GetDeviceAttributePushPayload extracts DeviceAttributePushPayload from message
func (m *MQTTMessage) GetDeviceAttributePushPayload() (*DeviceAttributePushPayload, error) {
	if m.Type != TypeDeviceAttributePush {
		return nil, fmt.Errorf("message type is not device attribute push: %d", m.Type)
	}
	data, err := json.Marshal(m.Payload)
	if err != nil {
		return nil, err
	}
	var payload DeviceAttributePushPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}
