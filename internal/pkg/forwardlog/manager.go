package forwardlog

import (
	"app-modbus-go/internal/pkg/logger"
	"app-modbus-go/internal/pkg/mqtt"
	"sync"
	"time"
)

// LogEntry represents a forward log entry
type LogEntry struct {
	Status          int
	NorthDeviceName string
	Data            map[string]interface{}
	Timestamp       time.Time
}

// Manager manages forward log reporting with batching and retry
type Manager struct {
	mqttClient *mqtt.ClientManager
	lc         logger.LoggingClient

	queue      []*LogEntry
	batchSize  int
	flushDelay time.Duration
	maxRetries int

	mu      sync.Mutex
	stopCh  chan struct{}
	flushCh chan struct{}
	doneCh  chan struct{}
}

// NewManager creates a new forward log manager
func NewManager(mqttClient *mqtt.ClientManager, lc logger.LoggingClient) *Manager {
	return &Manager{
		mqttClient: mqttClient,
		lc:         lc,
		queue:      make([]*LogEntry, 0),
		batchSize:  10,
		flushDelay: 5 * time.Second,
		maxRetries: 3,
		stopCh:     make(chan struct{}),
		flushCh:    make(chan struct{}, 1),
		doneCh:     make(chan struct{}),
	}
}

// Start starts the forward log manager
func (m *Manager) Start() {
	go m.run()
	m.lc.Info("Forward log manager started")
}

// Stop stops the forward log manager
func (m *Manager) Stop() {
	close(m.stopCh)
	<-m.doneCh
	m.lc.Info("Forward log manager stopped")
}

// LogSuccess logs a successful data forward
func (m *Manager) LogSuccess(northDeviceName string, data map[string]interface{}) {
	m.addEntry(1, northDeviceName, data)
}

// LogFailure logs a failed data forward
func (m *Manager) LogFailure(northDeviceName string, data map[string]interface{}) {
	m.addEntry(0, northDeviceName, data)
}

func (m *Manager) addEntry(status int, northDeviceName string, data map[string]interface{}) {
	entry := &LogEntry{
		Status:          status,
		NorthDeviceName: northDeviceName,
		Data:            data,
		Timestamp:       time.Now(),
	}

	m.mu.Lock()
	m.queue = append(m.queue, entry)
	shouldFlush := len(m.queue) >= m.batchSize
	m.mu.Unlock()

	if shouldFlush {
		select {
		case m.flushCh <- struct{}{}:
		default:
		}
	}
}

func (m *Manager) run() {
	defer close(m.doneCh)

	ticker := time.NewTicker(m.flushDelay)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			m.flush()
			return
		case <-ticker.C:
			m.flush()
		case <-m.flushCh:
			m.flush()
		}
	}
}

func (m *Manager) flush() {
	m.mu.Lock()
	if len(m.queue) == 0 {
		m.mu.Unlock()
		return
	}
	entries := m.queue
	m.queue = make([]*LogEntry, 0)
	m.mu.Unlock()

	for _, entry := range entries {
		m.sendLogEntry(entry)
	}
}

func (m *Manager) sendLogEntry(entry *LogEntry) {
	// Skip sending if mqttClient is nil (for testing)
	if m.mqttClient == nil {
		return
	}

	payload := &mqtt.ForwardLogPayload{
		Status:          entry.Status,
		NorthDeviceName: entry.NorthDeviceName,
		Data:            entry.Data,
	}
	msg := mqtt.NewMessage(mqtt.TypeForwardLog, payload)

	for attempt := 0; attempt < m.maxRetries; attempt++ {
		if err := m.mqttClient.Publish(msg); err != nil {
			m.lc.Warn("Failed to send forward log (attempt %d): %s", attempt+1, err.Error())
			time.Sleep(time.Second * time.Duration(attempt+1))
			continue
		}
		return
	}
	m.lc.Error("Failed to send forward log after %d attempts", m.maxRetries)
}
