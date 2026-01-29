package service

import (
	"app-modbus-go/internal/pkg/config"
	"app-modbus-go/internal/pkg/forwardlog"
	"app-modbus-go/internal/pkg/logger"
	"app-modbus-go/internal/pkg/mappingmanager"
	"app-modbus-go/internal/pkg/modbusserver"
	"app-modbus-go/internal/pkg/mqtt"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// AppService is the main application service
type AppService struct {
	appName    string
	version    string
	configPath string

	lc            logger.LoggingClient
	mqttClient    *mqtt.ClientManager
	mapManage     *mappingmanager.MappingManager
	mdbsServer    *modbusserver.ModbusServer
	forwardLogMgr *forwardlog.Manager
	config        *config.AppConfig

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewAppService creates a new application service
func NewAppService(name string, version string) (AppServiceInterface, error) {
	if name == "" {
		return nil, errors.New("please specify service name")
	}
	if version == "" {
		return nil, errors.New("please specify service version")
	}

	return &AppService{
		appName: name,
		version: version,
	}, nil
}

// Initialize initializes the service with configuration
func (s *AppService) Initialize(configPath string) error {
	s.configPath = configPath

	// Initialize logger first with default level
	s.lc = logger.NewClient("INFO")
	s.lc.Info("Initializing service:", s.appName, "version:", s.version)

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		// Try default config if file not found
		s.lc.Warn("Failed to load config file, using defaults:", err.Error())
		cfg = config.DefaultConfig()
	}
	s.config = cfg

	// Update log level from config
	if err := s.lc.SetLogLevel(cfg.Writable.LogLevel); err != nil {
		s.lc.Warn("Failed to set log level:", err.Error())
	}

	// Create context
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// Create MQTT client manager
	s.mqttClient = mqtt.NewClientManager(
		cfg.NodeID,
		mqtt.ClientConfig{
			Broker:    cfg.Mqtt.Broker,
			ClientID:  cfg.Mqtt.ClientID,
			Username:  cfg.Mqtt.Username,
			Password:  cfg.Mqtt.Password,
			QoS:       byte(cfg.Mqtt.QoS),
			KeepAlive: cfg.Mqtt.KeepAlive,
		},
		s.lc,
	)

	// Create mapping manager
	s.mapManage = mappingmanager.NewMappingManager(s.mqttClient, s.lc, &cfg.Cache)

	// Create forward log manager
	s.forwardLogMgr = forwardlog.NewManager(s.mqttClient, s.lc)

	// Create Modbus server
	s.mdbsServer = modbusserver.NewModbusServer(&cfg.Modbus, s.mapManage, s.lc)

	s.lc.Info("Service initialized successfully")
	return nil
}

// Run runs the service
func (s *AppService) Run() error {
	s.lc.Info("Starting service:", s.appName)

	// Connect MQTT
	mqttCfg := mqtt.ClientConfig{
		Broker:    s.config.Mqtt.Broker,
		ClientID:  s.config.Mqtt.ClientID,
		Username:  s.config.Mqtt.Username,
		Password:  s.config.Mqtt.Password,
		QoS:       byte(s.config.Mqtt.QoS),
		KeepAlive: s.config.Mqtt.KeepAlive,
	}
	if err := s.mqttClient.Connect(mqttCfg); err != nil {
		return fmt.Errorf("MQTT connect failed: %w", err)
	}

	// Register message handlers
	s.registerMQTTHandlers()

	// Subscribe to topics
	if err := s.mqttClient.Subscribe(); err != nil {
		return fmt.Errorf("MQTT subscribe failed: %w", err)
	}

	// Query device attributes from data center
	if err := s.mapManage.QueryDeviceAttributes(); err != nil {
		s.lc.Warn("Failed to query device attributes:", err.Error())
		s.lc.Info("Service will continue with empty mappings, waiting for data push")
	}

	// Start heartbeat
	s.mqttClient.StartHeartbeat(s.config.Heartbeat.GetInterval())

	// Start cache cleanup
	s.mapManage.StartCleanup()

	// Start forward log manager
	s.forwardLogMgr.Start()

	// Start Modbus server
	if err := s.mdbsServer.Start(s.ctx); err != nil {
		return fmt.Errorf("Modbus server start failed: %w", err)
	}

	s.lc.Info("Service started successfully")

	// Wait for shutdown signal
	s.waitForShutdown()

	return nil
}

// registerMQTTHandlers registers all MQTT message handlers
func (s *AppService) registerMQTTHandlers() {
	// Type 1: Heartbeat response
	s.mqttClient.RegisterResponseHandler(mqtt.TypeHeartbeat, func(resp *mqtt.MQTTResponse) error {
		s.lc.Debug("Heartbeat response received")
		return nil
	})

	// Type 2: Query device response is handled by PublishAndWait

	// Type 3: Device attribute push
	s.mqttClient.RegisterMessageHandler(mqtt.TypeDeviceAttributePush, func(msg *mqtt.MQTTMessage) error {
		return s.mapManage.HandleAttributeUpdate(msg)
	})

	// Type 4: Sensor data
	s.mqttClient.RegisterMessageHandler(mqtt.TypeSensorData, func(msg *mqtt.MQTTMessage) error {
		return s.mapManage.HandleSensorData(msg)
	})

	// Type 6: Command
	s.mqttClient.RegisterMessageHandler(mqtt.TypeCommand, func(msg *mqtt.MQTTMessage) error {
		return s.handleCommand(msg)
	})
}

// handleCommand handles type=6 command messages
func (s *AppService) handleCommand(msg *mqtt.MQTTMessage) error {
	payload, err := msg.GetCommandPayload()
	if err != nil {
		return err
	}

	s.lc.Debug(fmt.Sprintf("Received command: type=%s, device=%s, resource=%s",
		payload.CmdType, payload.CmdContent.NorthDeviceName, payload.CmdContent.NorthResourceName))

	var respPayload *mqtt.CommandResponsePayload

	switch payload.CmdType {
	case "GET":
		respPayload = s.handleGetCommand(payload)
	case "PUT":
		respPayload = s.handlePutCommand(payload)
	default:
		respPayload = &mqtt.CommandResponsePayload{
			CmdType:    payload.CmdType,
			StatusCode: 400,
			CmdContent: mqtt.CommandResponseContent{
				NorthDeviceName:   payload.CmdContent.NorthDeviceName,
				NorthResourceName: payload.CmdContent.NorthResourceName,
			},
		}
	}

	resp := mqtt.NewResponse(msg.RequestID, mqtt.TypeCommand, 200, "success", respPayload)
	return s.mqttClient.PublishResponse(resp)
}

// handleGetCommand handles GET commands
func (s *AppService) handleGetCommand(payload *mqtt.CommandPayload) *mqtt.CommandResponsePayload {
	dm, ok := s.mapManage.GetDeviceMapping(payload.CmdContent.NorthDeviceName)
	if !ok {
		return &mqtt.CommandResponsePayload{
			CmdType:    "GET",
			StatusCode: 404,
			CmdContent: mqtt.CommandResponseContent{
				NorthDeviceName:   payload.CmdContent.NorthDeviceName,
				NorthResourceName: payload.CmdContent.NorthResourceName,
			},
		}
	}

	// Find the resource and its Modbus address
	for _, rm := range dm.Resources {
		if rm.NorthResource != nil && rm.NorthResource.Name == payload.CmdContent.NorthResourceName {
			addr := rm.NorthResource.OtherParameters.Modbus.Address
			cachedData, ok := s.mapManage.GetCachedValue(addr)
			if !ok {
				return &mqtt.CommandResponsePayload{
					CmdType:    "GET",
					StatusCode: 404,
					CmdContent: mqtt.CommandResponseContent{
						NorthDeviceName:   payload.CmdContent.NorthDeviceName,
						NorthResourceName: payload.CmdContent.NorthResourceName,
					},
				}
			}

			return &mqtt.CommandResponsePayload{
				CmdType:    "GET",
				StatusCode: 200,
				CmdContent: mqtt.CommandResponseContent{
					NorthDeviceName:    payload.CmdContent.NorthDeviceName,
					NorthResourceName:  payload.CmdContent.NorthResourceName,
					NorthResourceValue: fmt.Sprintf("%v", cachedData.Value),
				},
			}
		}
	}

	return &mqtt.CommandResponsePayload{
		CmdType:    "GET",
		StatusCode: 404,
		CmdContent: mqtt.CommandResponseContent{
			NorthDeviceName:   payload.CmdContent.NorthDeviceName,
			NorthResourceName: payload.CmdContent.NorthResourceName,
		},
	}
}

// handlePutCommand handles PUT commands
func (s *AppService) handlePutCommand(payload *mqtt.CommandPayload) *mqtt.CommandResponsePayload {
	// For now, just acknowledge the PUT command
	// In a full implementation, this would write to the device via MQTT
	s.lc.Info(fmt.Sprintf("PUT command: %s/%s = %s",
		payload.CmdContent.NorthDeviceName,
		payload.CmdContent.NorthResourceName,
		payload.CmdContent.NorthResourceValue))

	return &mqtt.CommandResponsePayload{
		CmdType:    "PUT",
		StatusCode: 200,
		CmdContent: mqtt.CommandResponseContent{
			NorthDeviceName:    payload.CmdContent.NorthDeviceName,
			NorthResourceName:  payload.CmdContent.NorthResourceName,
			NorthResourceValue: payload.CmdContent.NorthResourceValue,
		},
	}
}

// waitForShutdown waits for a shutdown signal
func (s *AppService) waitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	s.lc.Info("Received signal:", sig.String())
	s.Stop()
}

// Stop stops the service
func (s *AppService) Stop() error {
	s.lc.Info("Stopping service:", s.appName)

	// Cancel context
	if s.cancel != nil {
		s.cancel()
	}

	// Stop Modbus server
	if s.mdbsServer != nil {
		s.mdbsServer.Stop()
	}

	// Stop forward log manager
	if s.forwardLogMgr != nil {
		s.forwardLogMgr.Stop()
	}

	// Stop mapping manager
	if s.mapManage != nil {
		s.mapManage.Stop()
	}

	// Disconnect MQTT
	if s.mqttClient != nil {
		s.mqttClient.Disconnect()
	}

	s.lc.Info("Service stopped successfully")
	return nil
}

// Getter methods

// GetLoggingClient returns the logging client
func (s *AppService) GetLoggingClient() logger.LoggingClient {
	return s.lc
}

// GetMappingManager returns the mapping manager
func (s *AppService) GetMappingManager() mappingmanager.MappingManagerInterface {
	return s.mapManage
}

// GetModbusServer returns the Modbus server
func (s *AppService) GetModbusServer() modbusserver.ModbusServerInterface {
	return s.mdbsServer
}

// GetMQTTClient returns the MQTT client manager
func (s *AppService) GetMQTTClient() *mqtt.ClientManager {
	return s.mqttClient
}

// GetForwardLogManager returns the forward log manager
func (s *AppService) GetForwardLogManager() *forwardlog.Manager {
	return s.forwardLogMgr
}

// GetAppConfig returns the application configuration
func (s *AppService) GetAppConfig() *config.AppConfig {
	return s.config
}

// GetContext returns the service context
func (s *AppService) GetContext() context.Context {
	return s.ctx
}
