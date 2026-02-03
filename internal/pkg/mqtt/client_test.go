package mqtt

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestGetNodeID tests the GetNodeID method
func TestGetNodeID(t *testing.T) {
	cm := createTestClientManager(t)
	assert.Equal(t, "test-node", cm.GetNodeID())
}

// TestIsConnected_NotConnected tests IsConnected when client is nil or not connected
func TestIsConnected_NotConnected(t *testing.T) {
	cm := createTestClientManager(t)
	// Client is nil initially (before Connect is called)
	assert.False(t, cm.IsConnected())
}

// TestStopHeartbeat_NoHeartbeatRunning tests stopping heartbeat when none is running
func TestStopHeartbeat_NoHeartbeatRunning(t *testing.T) {
	cm := createTestClientManager(t)
	// Should not panic when stopping non-existent heartbeat
	assert.NotPanics(t, func() {
		cm.StopHeartbeat()
	})
}

// TestStopHeartbeat_WithHeartbeat tests stopping an active heartbeat
func TestStopHeartbeat_WithHeartbeat(t *testing.T) {
	cm := createTestClientManager(t)
	// Simulate heartbeat channel
	cm.heartbeatStop = make(chan struct{})

	// Should close the channel
	assert.NotPanics(t, func() {
		cm.StopHeartbeat()
	})

	// Verify channel is closed
	_, ok := <-cm.heartbeatStop
	assert.False(t, ok, "heartbeatStop channel should be closed")
}

// TestDisconnect_NoClient tests disconnect when client is nil
func TestDisconnect_NoClient(t *testing.T) {
	cm := createTestClientManager(t)
	// Should not panic when client is nil
	assert.NotPanics(t, func() {
		cm.Disconnect()
	})
}

// TestOnMessage_InvalidJSON tests onMessage with invalid JSON
func TestOnMessage_InvalidJSON(t *testing.T) {
	cm := createTestClientManager(t)

	// Create a mock MQTT message with invalid JSON
	mockMsg := &mockMessage{
		topic:   cm.topicUp,
		payload: []byte("invalid json"),
	}

	// Should not panic on invalid JSON
	assert.NotPanics(t, func() {
		cm.onMessage(nil, mockMsg)
	})
}

// TestOnMessage_RegularMessage tests onMessage with a regular message
func TestOnMessage_RegularMessage(t *testing.T) {
	cm := createTestClientManager(t)

	handlerCalled := false
	receivedMsg := (*MQTTMessage)(nil)

	// Register a handler
	cm.RegisterMessageHandler(TypeHeartbeat, func(msg *MQTTMessage) error {
		handlerCalled = true
		receivedMsg = msg
		return nil
	})

	// Create a test message
	msg := NewMessage(TypeHeartbeat, &HeartbeatPayload{})
	data, _ := json.Marshal(msg)

	mockMsg := &mockMessage{
		topic:   cm.topicUp,
		payload: data,
	}

	// Call onMessage
	cm.onMessage(nil, mockMsg)

	// Verify handler was called
	assert.True(t, handlerCalled)
	assert.NotNil(t, receivedMsg)
	assert.Equal(t, TypeHeartbeat, receivedMsg.Type)
}

// TestOnMessage_Response tests onMessage with a response message
func TestOnMessage_Response(t *testing.T) {
	cm := createTestClientManager(t)

	handlerCalled := false
	receivedResp := (*MQTTResponse)(nil)

	// Register a response handler
	cm.RegisterResponseHandler(TypeHeartbeat, func(resp *MQTTResponse) error {
		handlerCalled = true
		receivedResp = resp
		return nil
	})

	// Create a test response
	resp := NewResponse("test-req-1", TypeHeartbeat, 200, "OK", nil)
	data, _ := json.Marshal(resp)

	mockMsg := &mockMessage{
		topic:   cm.topicUp,
		payload: data,
	}

	// Call onMessage
	cm.onMessage(nil, mockMsg)

	// Verify handler was called
	assert.True(t, handlerCalled)
	assert.NotNil(t, receivedResp)
	assert.Equal(t, 200, receivedResp.Code)
}

// TestOnMessage_PendingRequest tests onMessage with a pending request response
func TestOnMessage_PendingRequest(t *testing.T) {
	cm := createTestClientManager(t)

	requestID := "test-request-123"
	ch := make(chan *MQTTResponse, 1)

	// Add a pending request
	cm.pendingMu.Lock()
	cm.pendingRequests[requestID] = ch
	cm.pendingMu.Unlock()

	// Create a response for the pending request
	resp := NewResponse(requestID, TypeHeartbeat, 200, "OK", nil)
	data, _ := json.Marshal(resp)

	mockMsg := &mockMessage{
		topic:   cm.topicUp,
		payload: data,
	}

	// Call onMessage
	cm.onMessage(nil, mockMsg)

	// Verify response was sent to channel
	select {
	case receivedResp := <-ch:
		assert.NotNil(t, receivedResp)
		assert.Equal(t, requestID, receivedResp.RequestID)
		assert.Equal(t, 200, receivedResp.Code)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for response")
	}

	// Verify pending request was removed
	cm.pendingMu.RLock()
	_, exists := cm.pendingRequests[requestID]
	cm.pendingMu.RUnlock()
	assert.False(t, exists)
}

// TestOnMessage_NoHandler tests onMessage when no handler is registered
func TestOnMessage_NoHandler(t *testing.T) {
	cm := createTestClientManager(t)

	// Create a message with a type that has no handler
	msg := NewMessage(999, nil) // Type 999 has no handler
	data, _ := json.Marshal(msg)

	mockMsg := &mockMessage{
		topic:   cm.topicUp,
		payload: data,
	}

	// Should not panic when no handler is registered
	assert.NotPanics(t, func() {
		cm.onMessage(nil, mockMsg)
	})
}

// mockMessage implements pahomqtt.Message for testing
type mockMessage struct {
	topic   string
	payload []byte
}

func (m *mockMessage) Duplicate() bool              { return false }
func (m *mockMessage) Qos() byte                    { return 0 }
func (m *mockMessage) Retained() bool               { return false }
func (m *mockMessage) Topic() string                { return m.topic }
func (m *mockMessage) MessageID() uint16            { return 0 }
func (m *mockMessage) Payload() []byte              { return m.payload }
func (m *mockMessage) Ack()                         {}
