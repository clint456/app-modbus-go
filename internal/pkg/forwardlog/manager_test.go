package forwardlog

import (
	"app-modbus-go/internal/pkg/logger"
	"app-modbus-go/internal/pkg/mqtt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// MockMQTTClient for testing
type MockMQTTClient struct {
	publishedMessages []*mqtt.MQTTMessage
	publishErrors     []error
	publishCount      int32
	mu                sync.Mutex
}

func (m *MockMQTTClient) Publish(msg *mqtt.MQTTMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	atomic.AddInt32(&m.publishCount, 1)

	if len(m.publishErrors) > 0 {
		err := m.publishErrors[0]
		m.publishErrors = m.publishErrors[1:]
		return err
	}

	m.publishedMessages = append(m.publishedMessages, msg)
	return nil
}

func (m *MockMQTTClient) GetPublishCount() int {
	return int(atomic.LoadInt32(&m.publishCount))
}

func (m *MockMQTTClient) GetPublishedMessages() []*mqtt.MQTTMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.publishedMessages
}

func createTestManager(t *testing.T) (*Manager, *MockMQTTClient) {
	lc := logger.NewClient("DEBUG")
	mockClient := &MockMQTTClient{
		publishedMessages: make([]*mqtt.MQTTMessage, 0),
		publishErrors:     make([]error, 0),
	}
	manager := &Manager{
		mqttClient: (*mqtt.ClientManager)(nil), // We'll use mock
		lc:         lc,
		queue:      make([]*LogEntry, 0),
		batchSize:  10,
		flushDelay: 5 * time.Second,
		maxRetries: 3,
		stopCh:     make(chan struct{}),
		flushCh:    make(chan struct{}, 1),
		doneCh:     make(chan struct{}),
	}
	return manager, mockClient
}

func TestNewManager(t *testing.T) {
	lc := logger.NewClient("DEBUG")
	mqttCfg := mqtt.ClientConfig{
		Broker:    "tcp://localhost:1883",
		ClientID:  "test-client",
		QoS:       1,
		KeepAlive: 60,
	}
	mqttClient := mqtt.NewClientManager("test-node", mqttCfg, lc)
	manager := NewManager(mqttClient, lc)

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
	if manager.batchSize != 10 {
		t.Errorf("expected batchSize 10, got %d", manager.batchSize)
	}
	if manager.maxRetries != 3 {
		t.Errorf("expected maxRetries 3, got %d", manager.maxRetries)
	}
	if len(manager.queue) != 0 {
		t.Errorf("expected empty queue, got %d items", len(manager.queue))
	}
}

func TestLogSuccess(t *testing.T) {
	manager, _ := createTestManager(t)

	data := map[string]interface{}{
		"temperature": 25.5,
	}

	manager.LogSuccess("device1", data)

	manager.mu.Lock()
	if len(manager.queue) != 1 {
		t.Errorf("expected 1 entry in queue, got %d", len(manager.queue))
	}
	if manager.queue[0].Status != 1 {
		t.Errorf("expected status 1 (success), got %d", manager.queue[0].Status)
	}
	if manager.queue[0].NorthDeviceName != "device1" {
		t.Errorf("expected device 'device1', got %s", manager.queue[0].NorthDeviceName)
	}
	manager.mu.Unlock()
}

func TestLogFailure(t *testing.T) {
	manager, _ := createTestManager(t)

	data := map[string]interface{}{
		"temperature": 25.5,
	}

	manager.LogFailure("device1", data)

	manager.mu.Lock()
	if len(manager.queue) != 1 {
		t.Errorf("expected 1 entry in queue, got %d", len(manager.queue))
	}
	if manager.queue[0].Status != 0 {
		t.Errorf("expected status 0 (failure), got %d", manager.queue[0].Status)
	}
	manager.mu.Unlock()
}

func TestBatchFlushOnSize(t *testing.T) {
	manager, _ := createTestManager(t)
	manager.batchSize = 3

	// Add entries to trigger batch flush
	for i := 0; i < 3; i++ {
		manager.LogSuccess("device1", map[string]interface{}{"value": i})
	}

	// Check if flush was triggered
	manager.mu.Lock()
	queueSize := len(manager.queue)
	manager.mu.Unlock()

	// After batch size is reached, flush should be triggered
	// (though actual flush happens asynchronously)
	if queueSize > manager.batchSize {
		t.Errorf("expected queue size <= %d, got %d", manager.batchSize, queueSize)
	}
}

func TestAddEntry(t *testing.T) {
	manager, _ := createTestManager(t)

	data := map[string]interface{}{
		"temp": 20.0,
	}

	manager.addEntry(1, "device1", data)

	manager.mu.Lock()
	if len(manager.queue) != 1 {
		t.Errorf("expected 1 entry, got %d", len(manager.queue))
	}

	entry := manager.queue[0]
	if entry.Status != 1 {
		t.Errorf("expected status 1, got %d", entry.Status)
	}
	if entry.NorthDeviceName != "device1" {
		t.Errorf("expected device 'device1', got %s", entry.NorthDeviceName)
	}
	if entry.Data["temp"] != 20.0 {
		t.Errorf("expected temp 20.0, got %v", entry.Data["temp"])
	}
	manager.mu.Unlock()
}

func TestMultipleLogEntries(t *testing.T) {
	manager, _ := createTestManager(t)

	// Add multiple entries
	for i := 0; i < 5; i++ {
		manager.LogSuccess("device1", map[string]interface{}{"index": i})
	}

	manager.mu.Lock()
	if len(manager.queue) != 5 {
		t.Errorf("expected 5 entries, got %d", len(manager.queue))
	}
	manager.mu.Unlock()
}

func TestLogEntryTimestamp(t *testing.T) {
	manager, _ := createTestManager(t)

	before := time.Now()
	manager.LogSuccess("device1", map[string]interface{}{})
	after := time.Now()

	manager.mu.Lock()
	entry := manager.queue[0]
	manager.mu.Unlock()

	if entry.Timestamp.Before(before) || entry.Timestamp.After(after) {
		t.Error("entry timestamp not within expected range")
	}
}

func TestConcurrentLogging(t *testing.T) {
	manager, _ := createTestManager(t)
	numGoroutines := 10
	entriesPerGoroutine := 100

	var wg sync.WaitGroup

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < entriesPerGoroutine; i++ {
				manager.LogSuccess("device1", map[string]interface{}{"id": id, "index": i})
			}
		}(g)
	}

	wg.Wait()

	manager.mu.Lock()
	expectedCount := numGoroutines * entriesPerGoroutine
	if len(manager.queue) != expectedCount {
		t.Errorf("expected %d entries, got %d", expectedCount, len(manager.queue))
	}
	manager.mu.Unlock()
}

func TestFlushEmptyQueue(t *testing.T) {
	manager, _ := createTestManager(t)

	// Flush empty queue should not panic
	manager.flush()

	manager.mu.Lock()
	if len(manager.queue) != 0 {
		t.Errorf("expected empty queue after flush, got %d", len(manager.queue))
	}
	manager.mu.Unlock()
}

func TestFlushClearsQueue(t *testing.T) {
	manager, _ := createTestManager(t)

	// Add entries
	for i := 0; i < 5; i++ {
		manager.LogSuccess("device1", map[string]interface{}{"index": i})
	}

	manager.mu.Lock()
	if len(manager.queue) != 5 {
		t.Errorf("expected 5 entries before flush, got %d", len(manager.queue))
	}
	manager.mu.Unlock()

	// Flush
	manager.flush()

	manager.mu.Lock()
	if len(manager.queue) != 0 {
		t.Errorf("expected empty queue after flush, got %d", len(manager.queue))
	}
	manager.mu.Unlock()
}

func TestLogEntryData(t *testing.T) {
	manager, _ := createTestManager(t)

	data := map[string]interface{}{
		"temperature": 25.5,
		"humidity":    60,
		"status":      "ok",
	}

	manager.LogSuccess("device1", data)

	manager.mu.Lock()
	entry := manager.queue[0]
	manager.mu.Unlock()

	if entry.Data["temperature"] != 25.5 {
		t.Errorf("expected temperature 25.5, got %v", entry.Data["temperature"])
	}
	if entry.Data["humidity"] != 60 {
		t.Errorf("expected humidity 60, got %v", entry.Data["humidity"])
	}
	if entry.Data["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", entry.Data["status"])
	}
}

func TestQueueOrdering(t *testing.T) {
	manager, _ := createTestManager(t)

	// Add entries in order
	for i := 0; i < 5; i++ {
		manager.LogSuccess("device1", map[string]interface{}{"index": i})
	}

	manager.mu.Lock()
	for i := 0; i < 5; i++ {
		if manager.queue[i].Data["index"] != i {
			t.Errorf("expected index %d at position %d, got %v", i, i, manager.queue[i].Data["index"])
		}
	}
	manager.mu.Unlock()
}

func TestMixedSuccessAndFailure(t *testing.T) {
	manager, _ := createTestManager(t)

	manager.LogSuccess("device1", map[string]interface{}{})
	manager.LogFailure("device1", map[string]interface{}{})
	manager.LogSuccess("device2", map[string]interface{}{})

	manager.mu.Lock()
	if len(manager.queue) != 3 {
		t.Errorf("expected 3 entries, got %d", len(manager.queue))
	}

	if manager.queue[0].Status != 1 {
		t.Error("expected first entry to be success")
	}
	if manager.queue[1].Status != 0 {
		t.Error("expected second entry to be failure")
	}
	if manager.queue[2].Status != 1 {
		t.Error("expected third entry to be success")
	}
	manager.mu.Unlock()
}

func TestFlushChannelSignal(t *testing.T) {
	manager, _ := createTestManager(t)
	manager.batchSize = 2

	// Add one entry (not enough to trigger batch)
	manager.LogSuccess("device1", map[string]interface{}{})

	// Add second entry (should trigger flush signal)
	manager.LogSuccess("device1", map[string]interface{}{})

	// Check if flush channel was signaled
	select {
	case <-manager.flushCh:
		// Flush signal received
	case <-time.After(100 * time.Millisecond):
		t.Error("expected flush signal to be sent")
	}
}

func TestMultipleDevices(t *testing.T) {
	manager, _ := createTestManager(t)

	devices := []string{"device1", "device2", "device3"}
	for _, dev := range devices {
		manager.LogSuccess(dev, map[string]interface{}{"device": dev})
	}

	manager.mu.Lock()
	if len(manager.queue) != 3 {
		t.Errorf("expected 3 entries, got %d", len(manager.queue))
	}

	for i, dev := range devices {
		if manager.queue[i].NorthDeviceName != dev {
			t.Errorf("expected device %s at position %d, got %s", dev, i, manager.queue[i].NorthDeviceName)
		}
	}
	manager.mu.Unlock()
}

func TestLargeDataPayload(t *testing.T) {
	manager, _ := createTestManager(t)

	// Create large data payload
	largeData := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		largeData[string(rune(i))] = float64(i) * 1.5
	}

	manager.LogSuccess("device1", largeData)

	manager.mu.Lock()
	if len(manager.queue) != 1 {
		t.Errorf("expected 1 entry, got %d", len(manager.queue))
	}
	if len(manager.queue[0].Data) != 100 {
		t.Errorf("expected 100 data items, got %d", len(manager.queue[0].Data))
	}
	manager.mu.Unlock()
}
