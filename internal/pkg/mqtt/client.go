package mqtt

import (
	"app-modbus-go/internal/pkg/logger"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

// MessageHandler handles incoming MQTT messages of a specific type
type MessageHandler func(msg *MQTTMessage) error

// ResponseHandler handles incoming MQTT responses of a specific type
type ResponseHandler func(resp *MQTTResponse) error

// ClientManager manages MQTT connections and message routing
type ClientManager struct {
	client pahomqtt.Client
	nodeID string

	topicUp   string // subscribe: /v1/data/{nodeId}/up
	topicDown string // publish: /v1/data/{nodeId}/down

	messageHandlers  map[int]MessageHandler
	responseHandlers map[int]ResponseHandler

	// request/response matching
	pendingRequests map[string]chan *MQTTResponse
	pendingMu       sync.RWMutex

	heartbeatStop chan struct{}

	lc logger.LoggingClient
	mu sync.RWMutex
}

// ClientConfig holds MQTT client configuration
type ClientConfig struct {
	Broker    string
	ClientID  string
	Username  string
	Password  string
	QoS       byte
	KeepAlive int // seconds
}

// NewClientManager creates a new MQTT client manager
func NewClientManager(nodeID string, cfg ClientConfig, lc logger.LoggingClient) *ClientManager {
	return &ClientManager{
		nodeID:           nodeID,
		topicUp:          fmt.Sprintf("/v1/data/%s/up", nodeID),
		topicDown:        fmt.Sprintf("/v1/data/%s/down", nodeID),
		messageHandlers:  make(map[int]MessageHandler),
		responseHandlers: make(map[int]ResponseHandler),
		pendingRequests:  make(map[string]chan *MQTTResponse),
		lc:               lc,
	}
}

// Connect establishes the MQTT connection
func (cm *ClientManager) Connect(cfg ClientConfig) error {
	opts := pahomqtt.NewClientOptions()
	opts.AddBroker(cfg.Broker)
	opts.SetClientID(cfg.ClientID)
	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
	}
	if cfg.Password != "" {
		opts.SetPassword(cfg.Password)
	}
	if cfg.KeepAlive > 0 {
		opts.SetKeepAlive(time.Duration(cfg.KeepAlive) * time.Second)
	}
	opts.SetAutoReconnect(true)
	opts.SetCleanSession(true)
	opts.SetOnConnectHandler(func(c pahomqtt.Client) {
		cm.lc.Info("MQTT connected, re-subscribing topics")
		_ = cm.subscribe()
	})
	opts.SetConnectionLostHandler(func(c pahomqtt.Client, err error) {
		cm.lc.Warn("MQTT connection lost:", err.Error())
	})

	cm.client = pahomqtt.NewClient(opts)
	token := cm.client.Connect()
	token.Wait()
	if token.Error() != nil {
		return fmt.Errorf("MQTT connect failed: %w", token.Error())
	}
	cm.lc.Info("MQTT connected to broker:", cfg.Broker)
	return nil
}

// Subscribe subscribes to the up topic for receiving messages
func (cm *ClientManager) Subscribe() error {
	return cm.subscribe()
}

func (cm *ClientManager) subscribe() error {
	token := cm.client.Subscribe(cm.topicUp, 1, cm.onMessage)
	token.Wait()
	if token.Error() != nil {
		return fmt.Errorf("MQTT subscribe failed: %w", token.Error())
	}
	cm.lc.Info("Subscribed to topic:", cm.topicUp)
	return nil
}

// onMessage handles incoming MQTT messages and routes to appropriate handler
func (cm *ClientManager) onMessage(client pahomqtt.Client, msg pahomqtt.Message) {
	cm.lc.Debug("Received MQTT message on topic:", msg.Topic())

	raw := msg.Payload()

	// Try parsing as response first (has code/msg fields)
	var resp MQTTResponse
	if err := json.Unmarshal(raw, &resp); err == nil && resp.Code != 0 {
		cm.lc.Debug(fmt.Sprintf("Received response type=%d requestId=%s code=%d", resp.Type, resp.RequestID, resp.Code))

		// Check if this is a response to a pending request
		cm.pendingMu.RLock()
		ch, exists := cm.pendingRequests[resp.RequestID]
		cm.pendingMu.RUnlock()
		if exists {
			select {
			case ch <- &resp:
			default:
			}
			cm.pendingMu.Lock()
			delete(cm.pendingRequests, resp.RequestID)
			cm.pendingMu.Unlock()
			return
		}

		// Route to response handler
		cm.mu.RLock()
		handler, ok := cm.responseHandlers[resp.Type]
		cm.mu.RUnlock()
		if ok {
			if err := handler(&resp); err != nil {
				cm.lc.Error(fmt.Sprintf("Response handler error for type=%d: %s", resp.Type, err.Error()))
			}
		}
		return
	}

	// Parse as regular message
	var message MQTTMessage
	if err := json.Unmarshal(raw, &message); err != nil {
		cm.lc.Error("Failed to parse MQTT message:", err.Error())
		return
	}
	cm.lc.Debug(fmt.Sprintf("Received message type=%d requestId=%s", message.Type, message.RequestID))

	// Route to message handler
	cm.mu.RLock()
	handler, ok := cm.messageHandlers[message.Type]
	cm.mu.RUnlock()
	if ok {
		if err := handler(&message); err != nil {
			cm.lc.Error(fmt.Sprintf("Message handler error for type=%d: %s", message.Type, err.Error()))
		}
	} else {
		cm.lc.Warn(fmt.Sprintf("No handler registered for message type=%d", message.Type))
	}
}

// Publish publishes a message to the down topic
func (cm *ClientManager) Publish(msg *MQTTMessage) error {
	data, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}
	token := cm.client.Publish(cm.topicDown, 1, false, data)
	token.Wait()
	if token.Error() != nil {
		return fmt.Errorf("MQTT publish failed: %w", token.Error())
	}
	cm.lc.Debug(fmt.Sprintf("Published message type=%d to %s", msg.Type, cm.topicDown))
	return nil
}

// PublishResponse publishes a response message to the down topic
func (cm *ClientManager) PublishResponse(resp *MQTTResponse) error {
	data, err := resp.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize response: %w", err)
	}
	token := cm.client.Publish(cm.topicDown, 1, false, data)
	token.Wait()
	if token.Error() != nil {
		return fmt.Errorf("MQTT publish response failed: %w", token.Error())
	}
	cm.lc.Debug(fmt.Sprintf("Published response type=%d to %s", resp.Type, cm.topicDown))
	return nil
}

// PublishAndWait publishes a message and waits for a matching response
func (cm *ClientManager) PublishAndWait(msg *MQTTMessage, timeout time.Duration) (*MQTTResponse, error) {
	ch := make(chan *MQTTResponse, 1)

	cm.pendingMu.Lock()
	cm.pendingRequests[msg.RequestID] = ch
	cm.pendingMu.Unlock()

	if err := cm.Publish(msg); err != nil {
		cm.pendingMu.Lock()
		delete(cm.pendingRequests, msg.RequestID)
		cm.pendingMu.Unlock()
		return nil, err
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(timeout):
		cm.pendingMu.Lock()
		delete(cm.pendingRequests, msg.RequestID)
		cm.pendingMu.Unlock()
		return nil, fmt.Errorf("request %s timed out after %v", msg.RequestID, timeout)
	}
}

// StartHeartbeat starts periodic heartbeat sending
func (cm *ClientManager) StartHeartbeat(interval time.Duration) {
	cm.heartbeatStop = make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Send initial heartbeat immediately
		cm.sendHeartbeat()

		for {
			select {
			case <-ticker.C:
				cm.sendHeartbeat()
			case <-cm.heartbeatStop:
				cm.lc.Info("Heartbeat stopped")
				return
			}
		}
	}()
	cm.lc.Info(fmt.Sprintf("Heartbeat started with interval %v", interval))
}

func (cm *ClientManager) sendHeartbeat() {
	msg := NewMessage(TypeHeartbeat, nil)
	if err := cm.Publish(msg); err != nil {
		cm.lc.Error("Failed to send heartbeat:", err.Error())
	} else {
		cm.lc.Debug("Heartbeat sent")
	}
}

// StopHeartbeat stops the heartbeat goroutine
func (cm *ClientManager) StopHeartbeat() {
	if cm.heartbeatStop != nil {
		close(cm.heartbeatStop)
	}
}

// RegisterMessageHandler registers a handler for a specific message type
func (cm *ClientManager) RegisterMessageHandler(msgType int, handler MessageHandler) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.messageHandlers[msgType] = handler
}

// RegisterResponseHandler registers a handler for a specific response type
func (cm *ClientManager) RegisterResponseHandler(msgType int, handler ResponseHandler) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.responseHandlers[msgType] = handler
}

// Disconnect cleanly disconnects the MQTT client
func (cm *ClientManager) Disconnect() {
	cm.StopHeartbeat()
	if cm.client != nil && cm.client.IsConnected() {
		cm.client.Disconnect(1000)
		cm.lc.Info("MQTT disconnected")
	}
}

// GetNodeID returns the node ID
func (cm *ClientManager) GetNodeID() string {
	return cm.nodeID
}

// IsConnected returns whether the MQTT client is connected
func (cm *ClientManager) IsConnected() bool {
	return cm.client != nil && cm.client.IsConnected()
}
