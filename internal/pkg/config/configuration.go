package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// ModbusTcpConfig holds Modbus TCP specific configuration
type ModbusTcpConfig struct {
	Host    string `yaml:"Host"`
	Port    int    `yaml:"Port"`
	SlaveID byte   `yaml:"SlaveID"`
}

// ModbusRtuConfig holds Modbus RTU specific configuration
type ModbusRtuConfig struct {
	Port     string `yaml:"Port"`
	BaudRate int    `yaml:"BaudRate"`
	DataBits int    `yaml:"DataBits"`
	Parity   string `yaml:"Parity"`
	StopBits int    `yaml:"StopBits"`
	SlaveID  byte   `yaml:"SlaveID"`
}

// ModbusConfig holds all Modbus configuration
type ModbusConfig struct {
	Type        string          `yaml:"Type"` // "TCP" or "RTU"
	TCP         ModbusTcpConfig `yaml:"TCP"`
	RTU         ModbusRtuConfig `yaml:"RTU"`
	Timeout     int             `yaml:"Timeout"`     // in milliseconds
	PollingRate int             `yaml:"PollingRate"` // in milliseconds
}

// MqttConfig holds MQTT client configuration
type MqttConfig struct {
	Broker    string `yaml:"Broker"`
	ClientID  string `yaml:"ClientID"`
	Username  string `yaml:"Username"`
	Password  string `yaml:"Password"`
	QoS       int    `yaml:"QoS"`
	KeepAlive int    `yaml:"KeepAlive"` // seconds
	Workers   int    `yaml:"Workers"`
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	DefaultTTL      string `yaml:"DefaultTTL"`      // e.g., "30s"
	CleanupInterval string `yaml:"CleanupInterval"` // e.g., "5m"
}

// GetDefaultTTL returns the default TTL as time.Duration
func (c *CacheConfig) GetDefaultTTL() time.Duration {
	d, err := time.ParseDuration(c.DefaultTTL)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

// GetCleanupInterval returns the cleanup interval as time.Duration
func (c *CacheConfig) GetCleanupInterval() time.Duration {
	d, err := time.ParseDuration(c.CleanupInterval)
	if err != nil {
		return 5 * time.Minute
	}
	return d
}

// HeartbeatConfig holds heartbeat configuration
type HeartbeatConfig struct {
	Interval string `yaml:"Interval"` // e.g., "2m"
	Timeout  string `yaml:"Timeout"`  // e.g., "10s"
}

// GetInterval returns the heartbeat interval as time.Duration
func (h *HeartbeatConfig) GetInterval() time.Duration {
	d, err := time.ParseDuration(h.Interval)
	if err != nil {
		return 2 * time.Minute
	}
	return d
}

// GetTimeout returns the heartbeat timeout as time.Duration
func (h *HeartbeatConfig) GetTimeout() time.Duration {
	d, err := time.ParseDuration(h.Timeout)
	if err != nil {
		return 10 * time.Second
	}
	return d
}

// WritableConfig holds runtime-changeable configuration
type WritableConfig struct {
	LogLevel string `yaml:"LogLevel"`
}

// ServiceConfig holds service HTTP endpoint configuration
type ServiceConfig struct {
	Host string `yaml:"Host"`
	Port int    `yaml:"Port"`
}

// AppConfig is the main configuration structure
type AppConfig struct {
	Writable  WritableConfig  `yaml:"Writable"`
	Service   ServiceConfig   `yaml:"Service"`
	NodeID    string          `yaml:"NodeID"`
	Mqtt      MqttConfig      `yaml:"Mqtt"`
	Modbus    ModbusConfig    `yaml:"Modbus"`
	Cache     CacheConfig     `yaml:"Cache"`
	Heartbeat HeartbeatConfig `yaml:"Heartbeat"`
}

// Validate validates the configuration
func (c *AppConfig) Validate() error {
	if c.NodeID == "" {
		return errors.New("NodeID cannot be empty")
	}
	if c.Mqtt.Broker == "" {
		return errors.New("MQTT Broker cannot be empty")
	}
	if c.Mqtt.ClientID == "" {
		return errors.New("MQTT ClientID cannot be empty")
	}
	if c.Mqtt.Workers <= 0 {
		c.Mqtt.Workers = 4 // default
	}
	if c.Mqtt.QoS < 0 || c.Mqtt.QoS > 2 {
		return errors.New("MQTT QoS must be 0, 1, or 2")
	}
	if c.Mqtt.KeepAlive <= 0 {
		c.Mqtt.KeepAlive = 60 // default
	}

	// Validate Modbus config based on type
	switch c.Modbus.Type {
	case "TCP":
		if c.Modbus.TCP.Host == "" {
			c.Modbus.TCP.Host = "0.0.0.0"
		}
		if c.Modbus.TCP.Port <= 0 {
			c.Modbus.TCP.Port = 502
		}
		if c.Modbus.TCP.SlaveID == 0 {
			c.Modbus.TCP.SlaveID = 1
		}
	case "RTU":
		if c.Modbus.RTU.Port == "" {
			return errors.New("Modbus RTU Port cannot be empty")
		}
		if c.Modbus.RTU.BaudRate <= 0 {
			c.Modbus.RTU.BaudRate = 9600
		}
		if c.Modbus.RTU.DataBits <= 0 {
			c.Modbus.RTU.DataBits = 8
		}
		if c.Modbus.RTU.Parity == "" {
			c.Modbus.RTU.Parity = "N"
		}
		if c.Modbus.RTU.StopBits <= 0 {
			c.Modbus.RTU.StopBits = 1
		}
		if c.Modbus.RTU.SlaveID == 0 {
			c.Modbus.RTU.SlaveID = 1
		}
	default:
		c.Modbus.Type = "TCP" // default to TCP
	}

	// Set defaults for cache and heartbeat
	if c.Cache.DefaultTTL == "" {
		c.Cache.DefaultTTL = "30s"
	}
	if c.Cache.CleanupInterval == "" {
		c.Cache.CleanupInterval = "5m"
	}
	if c.Heartbeat.Interval == "" {
		c.Heartbeat.Interval = "2m"
	}
	if c.Heartbeat.Timeout == "" {
		c.Heartbeat.Timeout = "10s"
	}

	// Set defaults for writable
	if c.Writable.LogLevel == "" {
		c.Writable.LogLevel = "INFO"
	}

	// Set defaults for service
	if c.Service.Host == "" {
		c.Service.Host = "localhost"
	}
	if c.Service.Port <= 0 {
		c.Service.Port = 59711
	}

	return nil
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config AppConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// DefaultConfig returns a default configuration
func DefaultConfig() *AppConfig {
	return &AppConfig{
		Writable: WritableConfig{
			LogLevel: "DEBUG",
		},
		Service: ServiceConfig{
			Host: "localhost",
			Port: 59711,
		},
		NodeID: "modbus-node-001",
		Mqtt: MqttConfig{
			Broker:    "tcp://localhost:1883",
			ClientID:  "app-modbus-go-001",
			QoS:       1,
			KeepAlive: 60,
			Workers:   4,
		},
		Modbus: ModbusConfig{
			Type: "TCP",
			TCP: ModbusTcpConfig{
				Host:    "0.0.0.0",
				Port:    502,
				SlaveID: 1,
			},
		},
		Cache: CacheConfig{
			DefaultTTL:      "30s",
			CleanupInterval: "5m",
		},
		Heartbeat: HeartbeatConfig{
			Interval: "2m",
			Timeout:  "10s",
		},
	}
}
