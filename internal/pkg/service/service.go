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

// AppService 是主应用服务
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

// NewAppService 创建新的应用服务
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

// Initialize 使用配置初始化服务
func (s *AppService) Initialize(configPath string) error {
	s.configPath = configPath

	// 首先使用默认级别初始化记录器
	s.lc = logger.NewClient("INFO")
	s.lc.Info("Initializing service:", s.appName, "version:", s.version)

	// 加载配置
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		// 如果文件未找到,尝试默认配置
		s.lc.Warn("Failed to load config file, using defaults:", err.Error())
		cfg = config.DefaultConfig()
	}
	s.config = cfg

	// 从配置更新日志级别
	if err := s.lc.SetLogLevel(cfg.Writable.LogLevel); err != nil {
		s.lc.Warn("Failed to set log level:", err.Error())
	}

	// 创建上下文
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// 创建MQTT客户端管理器
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

	// 创建映射管理器
	s.mapManage = mappingmanager.NewMappingManager(s.mqttClient, s.lc, &cfg.Cache)

	// 创建前向日志管理器
	s.forwardLogMgr = forwardlog.NewManager(s.mqttClient, s.lc)

	// 创建Modbus服务器
	s.mdbsServer = modbusserver.NewModbusServer(&cfg.Modbus, s.mapManage, s.lc)

	s.lc.Info("Service initialized successfully")
	return nil
}

// Run 运行服务
func (s *AppService) Run() error {
	s.lc.Info("Starting service:", s.appName)

	// 连接MQTT
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

	// 注册消息处理程序
	s.registerMQTTHandlers()

	// 订阅主题
	if err := s.mqttClient.Subscribe(); err != nil {
		return fmt.Errorf("MQTT subscribe failed: %w", err)
	}

	// 从数据中心查询设备属性
	if err := s.mapManage.QueryDeviceAttributes(); err != nil {
		s.lc.Warn("Failed to query device attributes:", err.Error())
		s.lc.Info("Service will continue with empty mappings, waiting for data push")
	}

	// 启动心跳
	s.mqttClient.StartHeartbeat(s.config.Heartbeat.GetInterval())

	// 启动缓存清理
	s.mapManage.StartCleanup()

	// 启动前向日志管理器
	s.forwardLogMgr.Start()

	// 启动Modbus服务器
	if err := s.mdbsServer.Start(s.ctx); err != nil {
		return fmt.Errorf("Modbus server start failed: %w", err)
	}

	s.lc.Info("Service started successfully")

	// 等待关闭信号
	s.waitForShutdown()

	return nil
}

// registerMQTTHandlers 注册所有MQTT消息处理程序
func (s *AppService) registerMQTTHandlers() {
	// Type 1: 心跳响应
	s.mqttClient.RegisterResponseHandler(mqtt.TypeHeartbeat, func(resp *mqtt.MQTTResponse) error {
		s.lc.Debug("Heartbeat response received")
		return nil
	})

	// Type 2: 查询设备响应由PublishAndWait处理

	// Type 3: 设备属性推送
	s.mqttClient.RegisterMessageHandler(mqtt.TypeDeviceAttributePush, func(msg *mqtt.MQTTMessage) error {
		return s.mapManage.HandleAttributeUpdate(msg)
	})

	// Type 4: 传感器数据
	s.mqttClient.RegisterMessageHandler(mqtt.TypeSensorData, func(msg *mqtt.MQTTMessage) error {
		return s.mapManage.HandleSensorData(msg)
	})

	// Type 6: 命令
	s.mqttClient.RegisterMessageHandler(mqtt.TypeCommand, func(msg *mqtt.MQTTMessage) error {
		return s.handleCommand(msg)
	})
}

// handleCommand 处理type=6命令消息
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

// handleGetCommand 处理GET命令
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

	// 查找资源及其Modbus地址
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

// handlePutCommand 处理PUT命令
func (s *AppService) handlePutCommand(payload *mqtt.CommandPayload) *mqtt.CommandResponsePayload {
	// 现在只是确认PUT命令
	// 在完整的实现中,这会通过MQTT向设备写入
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

// waitForShutdown 等待关闭信号
func (s *AppService) waitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	s.lc.Info("Received signal:", sig.String())
	s.Stop()
}

// Stop 停止服务
func (s *AppService) Stop() error {
	s.lc.Info("Stopping service:", s.appName)

	// 取消上下文
	if s.cancel != nil {
		s.cancel()
	}

	// 停止Modbus服务器
	if s.mdbsServer != nil {
		s.mdbsServer.Stop()
	}

	// 停止前向日志管理器
	if s.forwardLogMgr != nil {
		s.forwardLogMgr.Stop()
	}

	// 停止映射管理器
	if s.mapManage != nil {
		s.mapManage.Stop()
	}

	// 断开MQTT连接
	if s.mqttClient != nil {
		s.mqttClient.Disconnect()
	}

	s.lc.Info("Service stopped successfully")
	return nil
}

// Getter methods (获取器方法)

// GetLoggingClient 返回日志客户端
func (s *AppService) GetLoggingClient() logger.LoggingClient {
	return s.lc
}

// GetMappingManager 返回映射管理器
func (s *AppService) GetMappingManager() mappingmanager.MappingManagerInterface {
	return s.mapManage
}

// GetModbusServer 返回Modbus服务器
func (s *AppService) GetModbusServer() modbusserver.ModbusServerInterface {
	return s.mdbsServer
}

// GetMQTTClient 返回MQTT客户端管理器
func (s *AppService) GetMQTTClient() *mqtt.ClientManager {
	return s.mqttClient
}

// GetForwardLogManager 返回前向日志管理器
func (s *AppService) GetForwardLogManager() *forwardlog.Manager {
	return s.forwardLogMgr
}

// GetAppConfig 返回应用配置
func (s *AppService) GetAppConfig() *config.AppConfig {
	return s.config
}

// GetContext 返回服务上下文
func (s *AppService) GetContext() context.Context {
	return s.ctx
}
