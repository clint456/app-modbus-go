package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestCacheConfig_GetDefaultTTL tests the GetDefaultTTL method
func TestCacheConfig_GetDefaultTTL(t *testing.T) {
	tests := []struct {
		name       string
		defaultTTL string
		want       time.Duration
	}{
		{
			name:       "valid duration - seconds",
			defaultTTL: "30s",
			want:       30 * time.Second,
		},
		{
			name:       "valid duration - minutes",
			defaultTTL: "5m",
			want:       5 * time.Minute,
		},
		{
			name:       "valid duration - hours",
			defaultTTL: "1h",
			want:       1 * time.Hour,
		},
		{
			name:       "invalid duration",
			defaultTTL: "invalid",
			want:       30 * time.Second, // default
		},
		{
			name:       "empty duration",
			defaultTTL: "",
			want:       30 * time.Second, // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CacheConfig{DefaultTTL: tt.defaultTTL}
			got := c.GetDefaultTTL()
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestCacheConfig_GetCleanupInterval tests the GetCleanupInterval method
func TestCacheConfig_GetCleanupInterval(t *testing.T) {
	tests := []struct {
		name            string
		cleanupInterval string
		want            time.Duration
	}{
		{
			name:            "valid duration - minutes",
			cleanupInterval: "5m",
			want:            5 * time.Minute,
		},
		{
			name:            "valid duration - seconds",
			cleanupInterval: "30s",
			want:            30 * time.Second,
		},
		{
			name:            "valid duration - hours",
			cleanupInterval: "2h",
			want:            2 * time.Hour,
		},
		{
			name:            "invalid duration",
			cleanupInterval: "invalid",
			want:            5 * time.Minute, // default
		},
		{
			name:            "empty duration",
			cleanupInterval: "",
			want:            5 * time.Minute, // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CacheConfig{CleanupInterval: tt.cleanupInterval}
			got := c.GetCleanupInterval()
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestHeartbeatConfig_GetInterval tests the GetInterval method
func TestHeartbeatConfig_GetInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval string
		want     time.Duration
	}{
		{
			name:     "valid duration - minutes",
			interval: "2m",
			want:     2 * time.Minute,
		},
		{
			name:     "valid duration - seconds",
			interval: "30s",
			want:     30 * time.Second,
		},
		{
			name:     "valid duration - hours",
			interval: "1h",
			want:     1 * time.Hour,
		},
		{
			name:     "invalid duration",
			interval: "invalid",
			want:     2 * time.Minute, // default
		},
		{
			name:     "empty duration",
			interval: "",
			want:     2 * time.Minute, // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &HeartbeatConfig{Interval: tt.interval}
			got := h.GetInterval()
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestHeartbeatConfig_GetTimeout tests the GetTimeout method
func TestHeartbeatConfig_GetTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout string
		want    time.Duration
	}{
		{
			name:    "valid duration - seconds",
			timeout: "10s",
			want:    10 * time.Second,
		},
		{
			name:    "valid duration - minutes",
			timeout: "1m",
			want:    1 * time.Minute,
		},
		{
			name:    "invalid duration",
			timeout: "invalid",
			want:    10 * time.Second, // default
		},
		{
			name:    "empty duration",
			timeout: "",
			want:    10 * time.Second, // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &HeartbeatConfig{Timeout: tt.timeout}
			got := h.GetTimeout()
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestDefaultConfig tests the DefaultConfig function
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.NotNil(t, cfg)
	assert.Equal(t, "DEBUG", cfg.Writable.LogLevel)
	assert.Equal(t, "localhost", cfg.Service.Host)
	assert.Equal(t, 59711, cfg.Service.Port)
	assert.Equal(t, "modbus-node-001", cfg.NodeID)
	assert.Equal(t, "tcp://localhost:1883", cfg.Mqtt.Broker)
	assert.Equal(t, "app-modbus-go-001", cfg.Mqtt.ClientID)
	assert.Equal(t, 1, cfg.Mqtt.QoS)
	assert.Equal(t, 60, cfg.Mqtt.KeepAlive)
	assert.Equal(t, 4, cfg.Mqtt.Workers)
	assert.Equal(t, "TCP", cfg.Modbus.Type)
	assert.Equal(t, "0.0.0.0", cfg.Modbus.TCP.Host)
	assert.Equal(t, 502, cfg.Modbus.TCP.Port)
	assert.Equal(t, byte(1), cfg.Modbus.TCP.SlaveID)
	assert.Equal(t, "30s", cfg.Cache.DefaultTTL)
	assert.Equal(t, "5m", cfg.Cache.CleanupInterval)
	assert.Equal(t, "2m", cfg.Heartbeat.Interval)
	assert.Equal(t, "10s", cfg.Heartbeat.Timeout)
}

// TestAppConfig_Validate tests the Validate method
func TestAppConfig_Validate(t *testing.T) {
	t.Run("missing NodeID", func(t *testing.T) {
		cfg := &AppConfig{
			NodeID: "",
			Mqtt: MqttConfig{
				Broker:   "tcp://localhost:1883",
				ClientID: "test-client",
			},
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "NodeID cannot be empty")
	})

	t.Run("missing MQTT Broker", func(t *testing.T) {
		cfg := &AppConfig{
			NodeID: "node1",
			Mqtt: MqttConfig{
				Broker:   "",
				ClientID: "test-client",
			},
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MQTT Broker cannot be empty")
	})

	t.Run("missing MQTT ClientID", func(t *testing.T) {
		cfg := &AppConfig{
			NodeID: "node1",
			Mqtt: MqttConfig{
				Broker:   "tcp://localhost:1883",
				ClientID: "",
			},
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MQTT ClientID cannot be empty")
	})

	t.Run("invalid MQTT QoS - negative", func(t *testing.T) {
		cfg := &AppConfig{
			NodeID: "node1",
			Mqtt: MqttConfig{
				Broker:   "tcp://localhost:1883",
				ClientID: "test-client",
				QoS:      -1,
			},
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MQTT QoS must be 0, 1, or 2")
	})

	t.Run("invalid MQTT QoS - too high", func(t *testing.T) {
		cfg := &AppConfig{
			NodeID: "node1",
			Mqtt: MqttConfig{
				Broker:   "tcp://localhost:1883",
				ClientID: "test-client",
				QoS:      3,
			},
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MQTT QoS must be 0, 1, or 2")
	})

	t.Run("sets default MQTT Workers", func(t *testing.T) {
		cfg := &AppConfig{
			NodeID: "node1",
			Mqtt: MqttConfig{
				Broker:   "tcp://localhost:1883",
				ClientID: "test-client",
				QoS:      1,
				Workers:  0,
			},
		}
		err := cfg.Validate()
		assert.NoError(t, err)
		assert.Equal(t, 4, cfg.Mqtt.Workers)
	})

	t.Run("sets default MQTT KeepAlive", func(t *testing.T) {
		cfg := &AppConfig{
			NodeID: "node1",
			Mqtt: MqttConfig{
				Broker:    "tcp://localhost:1883",
				ClientID:  "test-client",
				QoS:       1,
				KeepAlive: 0,
			},
		}
		err := cfg.Validate()
		assert.NoError(t, err)
		assert.Equal(t, 60, cfg.Mqtt.KeepAlive)
	})
}
