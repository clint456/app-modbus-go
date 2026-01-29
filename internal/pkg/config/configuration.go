package config

// This file contains example of custom configuration that can be loaded from the service's configuration.yaml
// and/or the Configuration Provider, aka keeper (if enabled).
// For more details see https://docs.edgexfoundry.org/latest/microservices/application/GeneralAppServiceConfig/#custom-configuration

import (
	"errors"
)

type ModbusConfig struct {
	Port        string `yaml:"Port"`
	BaudRate    int    `yaml:"BaudRate"`
	DataBits    int    `yaml:"DataBits"`
	Parity      string `yaml:"Parity"`
	StopBits    int    `yaml:"StopBits"`
	SlaveID     int    `yaml:"SlaveID"`
	Timeout     int    `yaml:"Timeout"`     // in milliseconds
	PollingRate int    `yaml:"PollingRate"` // in milliseconds
}

type MqttConfig struct {
	Broker   string `yaml:"Broker"`
	ClientID string `yaml:"ClientID"`
	Username string `yaml:"Username"`
	Password string `yaml:"Password"`
	Workers  int    `yaml:"Workers"`
}

type AppConfig struct {
	NodeID string       `yaml:"NodeID"`
	Modbus ModbusConfig `yaml:"Modbus"`
	Mqtt   MqttConfig   `yaml:"Mqtt"`
}

func (c *AppConfig) Validate() error {
	if c.Modbus.Port == "" {
		return errors.New("Modbus Port cannot be empty")
	}
	if c.Modbus.BaudRate <= 0 {
		return errors.New("Modbus BaudRate must be positive")
	}
	if c.Mqtt.Broker == "" {
		return errors.New("MQTT Broker cannot be empty")
	}
	if c.Mqtt.ClientID == "" {
		return errors.New("MQTT ClientID cannot be empty")
	}
	if c.Mqtt.Workers <= 0 {
		return errors.New("MQTT Workers must be positive")
	}
	return nil
}
