package mqtt

import (
	"app-modbus-go/internal/pkg/logger"
	"encoding/json"
	"testing"
	"time"
)

func createTestClientManager(t *testing.T) *ClientManager {
	lc := logger.NewClient("DEBUG")
	cfg := ClientConfig{
		Broker:    "tcp://localhost:1883",
		ClientID:  "test-client",
		Username:  "",
		Password:  "",
		QoS:       1,
		KeepAlive: 60,
	}
	return NewClientManager("test-node", cfg, lc)
}

func TestNewClientManager(t *testing.T) {
	cm := createTestClientManager(t)

	if cm == nil {
		t.Fatal("NewClientManager returned nil")
	}
	if cm.nodeID != "test-node" {
		t.Errorf("expected nodeID 'test-node', got %s", cm.nodeID)
	}
	if cm.topicUp != "/v1/data/test-node/up" {
		t.Errorf("expected topicUp '/v1/data/test-node/up', got %s", cm.topicUp)
	}
	if cm.topicDown != "/v1/data/test-node/down" {
		t.Errorf("expected topicDown '/v1/data/test-node/down', got %s", cm.topicDown)
	}
	if len(cm.messageHandlers) != 0 {
		t.Errorf("expected empty messageHandlers, got %d", len(cm.messageHandlers))
	}
	if len(cm.responseHandlers) != 0 {
		t.Errorf("expected empty responseHandlers, got %d", len(cm.responseHandlers))
	}
}

func TestRegisterMessageHandler(t *testing.T) {
	cm := createTestClientManager(t)

	handler := func(msg *MQTTMessage) error {
		return nil
	}

	cm.RegisterMessageHandler(TypeSensorData, handler)

	if len(cm.messageHandlers) != 1 {
		t.Errorf("expected 1 handler, got %d", len(cm.messageHandlers))
	}

	if _, ok := cm.messageHandlers[TypeSensorData]; !ok {
		t.Error("handler not registered for TypeSensorData")
	}
}

func TestRegisterResponseHandler(t *testing.T) {
	cm := createTestClientManager(t)

	handler := func(resp *MQTTResponse) error {
		return nil
	}

	cm.RegisterResponseHandler(TypeQueryDevice, handler)

	if len(cm.responseHandlers) != 1 {
		t.Errorf("expected 1 handler, got %d", len(cm.responseHandlers))
	}

	if _, ok := cm.responseHandlers[TypeQueryDevice]; !ok {
		t.Error("handler not registered for TypeQueryDevice")
	}
}

func TestNewMessage(t *testing.T) {
	payload := &HeartbeatPayload{}
	msg := NewMessage(TypeHeartbeat, payload)

	if msg == nil {
		t.Fatal("NewMessage returned nil")
	}
	if msg.Type != TypeHeartbeat {
		t.Errorf("expected type %d, got %d", TypeHeartbeat, msg.Type)
	}
	if msg.Version != "1.0" {
		t.Errorf("expected version '1.0', got %s", msg.Version)
	}
	if msg.RequestID == "" {
		t.Error("expected non-empty RequestID")
	}
	if msg.Timestamp == 0 {
		t.Error("expected non-zero Timestamp")
	}
}

func TestMessageSerialization(t *testing.T) {
	payload := &HeartbeatPayload{}
	msg := NewMessage(TypeHeartbeat, payload)

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal message: %v", err)
	}

	if len(data) == 0 {
		t.Error("marshaled message is empty")
	}

	// Verify it can be unmarshaled
	var unmarshaled MQTTMessage
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal message: %v", err)
	}

	if unmarshaled.Type != TypeHeartbeat {
		t.Errorf("expected type %d, got %d", TypeHeartbeat, unmarshaled.Type)
	}
}

func TestResponseSerialization(t *testing.T) {
	payload := &HeartbeatPayload{}
	resp := NewResponse("test-request-1", TypeHeartbeat, 200, "success", payload)

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var unmarshaled MQTTResponse
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if unmarshaled.Code != 200 {
		t.Errorf("expected code 200, got %d", unmarshaled.Code)
	}
	if unmarshaled.Msg != "success" {
		t.Errorf("expected msg 'success', got %s", unmarshaled.Msg)
	}
}

func TestSensorDataPayloadSerialization(t *testing.T) {
	payload := &SensorDataPayload{
		NorthDeviceName: "device1",
		Data: map[string]interface{}{
			"temperature": 25.5,
			"humidity":    60,
		},
	}

	msg := NewMessage(TypeSensorData, payload)
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled MQTTMessage
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.Type != TypeSensorData {
		t.Errorf("expected type %d, got %d", TypeSensorData, unmarshaled.Type)
	}
}

func TestForwardLogPayloadSerialization(t *testing.T) {
	payload := &ForwardLogPayload{
		Status:          1,
		NorthDeviceName: "device1",
		Data: map[string]interface{}{
			"temperature": 25.5,
		},
	}

	msg := NewMessage(TypeForwardLog, payload)
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled MQTTMessage
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.Type != TypeForwardLog {
		t.Errorf("expected type %d, got %d", TypeForwardLog, unmarshaled.Type)
	}
}

func TestCommandPayloadSerialization(t *testing.T) {
	payload := &CommandPayload{
		CmdType: "GET",
		CmdContent: struct {
			NorthDeviceName   string `json:"northDeviceName"`
			NorthResourceName string `json:"northResourceName"`
			NorthResourceValue string `json:"northResourceValue,omitempty"`
		}{
			NorthDeviceName:   "device1",
			NorthResourceName: "temperature",
		},
	}

	msg := NewMessage(TypeCommand, payload)
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled MQTTMessage
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.Type != TypeCommand {
		t.Errorf("expected type %d, got %d", TypeCommand, unmarshaled.Type)
	}
}

func TestMessageRequestIDUniqueness(t *testing.T) {
	msg1 := NewMessage(TypeHeartbeat, &HeartbeatPayload{})
	msg2 := NewMessage(TypeHeartbeat, &HeartbeatPayload{})

	if msg1.RequestID == msg2.RequestID {
		t.Error("expected different RequestIDs for different messages")
	}
}

func TestMessageTimestampProgression(t *testing.T) {
	msg1 := NewMessage(TypeHeartbeat, &HeartbeatPayload{})
	time.Sleep(10 * time.Millisecond)
	msg2 := NewMessage(TypeHeartbeat, &HeartbeatPayload{})

	if msg1.Timestamp >= msg2.Timestamp {
		t.Error("expected msg2 timestamp to be greater than msg1")
	}
}

func TestResponseInheritance(t *testing.T) {
	payload := &HeartbeatPayload{}
	resp := NewResponse("test-request-2", TypeHeartbeat, 200, "OK", payload)

	if resp.Type != TypeHeartbeat {
		t.Errorf("expected type %d, got %d", TypeHeartbeat, resp.Type)
	}
	if resp.Version != "1.0" {
		t.Errorf("expected version '1.0', got %s", resp.Version)
	}
	if resp.RequestID == "" {
		t.Error("expected non-empty RequestID")
	}
}

func TestMultipleMessageTypes(t *testing.T) {
	types := []int{
		TypeHeartbeat,
		TypeQueryDevice,
		TypeDeviceAttributePush,
		TypeSensorData,
		TypeForwardLog,
		TypeCommand,
	}

	for _, msgType := range types {
		msg := NewMessage(msgType, nil)
		if msg.Type != msgType {
			t.Errorf("expected type %d, got %d", msgType, msg.Type)
		}
	}
}

func TestResponseCodeVariations(t *testing.T) {
	codes := []int{200, 400, 401, 403, 404, 500}

	for _, code := range codes {
		resp := NewResponse("test-request-3", TypeHeartbeat, code, "OK", nil)

		if resp.Code != code {
			t.Errorf("expected code %d, got %d", code, resp.Code)
		}
	}
}

func TestMessageWithComplexPayload(t *testing.T) {
	payload := &SensorDataPayload{
		NorthDeviceName: "device1",
		Data: map[string]interface{}{
			"temperature": 25.5,
			"humidity":    60,
			"pressure":    1013.25,
			"status":      "ok",
			"timestamp":   1704067200000,
		},
	}

	msg := NewMessage(TypeSensorData, payload)
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled MQTTMessage
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify payload is preserved
	payloadBytes, _ := json.Marshal(unmarshaled.Payload)
	var unmarshaledPayload SensorDataPayload
	json.Unmarshal(payloadBytes, &unmarshaledPayload)

	if unmarshaledPayload.NorthDeviceName != "device1" {
		t.Errorf("expected device 'device1', got %s", unmarshaledPayload.NorthDeviceName)
	}
}

func TestClientManagerConcurrentHandlerRegistration(t *testing.T) {
	cm := createTestClientManager(t)

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			handler := func(msg *MQTTMessage) error {
				return nil
			}
			cm.RegisterMessageHandler(TypeSensorData+id, handler)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if len(cm.messageHandlers) != 10 {
		t.Errorf("expected 10 handlers, got %d", len(cm.messageHandlers))
	}
}

func TestMessageVersionConsistency(t *testing.T) {
	types := []int{
		TypeHeartbeat,
		TypeQueryDevice,
		TypeSensorData,
		TypeForwardLog,
		TypeCommand,
	}

	for _, msgType := range types {
		msg := NewMessage(msgType, nil)
		if msg.Version != "1.0" {
			t.Errorf("expected version '1.0' for type %d, got %s", msgType, msg.Version)
		}
	}
}

func TestResponseDefaultValues(t *testing.T) {
	resp := NewResponse("test-request-4", TypeHeartbeat, 0, "", nil)

	if resp.Code != 0 {
		t.Errorf("expected default code 0, got %d", resp.Code)
	}
	if resp.Msg != "" {
		t.Errorf("expected default msg '', got %s", resp.Msg)
	}
}

func TestMessagePayloadNil(t *testing.T) {
	msg := NewMessage(TypeHeartbeat, nil)

	if msg.Payload != nil {
		t.Error("expected nil payload")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal message with nil payload: %v", err)
	}

	var unmarshaled MQTTMessage
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal message with nil payload: %v", err)
	}
}
