package service

import (
	"app-modbus-go/internal/pkg/config"
	"app-modbus-go/internal/pkg/devicemanager"
	"app-modbus-go/internal/pkg/logger"
	"app-modbus-go/internal/pkg/mappingmanager"
	"app-modbus-go/internal/pkg/modbusserver"
	"app-modbus-go/internal/pkg/register"
	"context"
	"errors"
	"sync"
)

type AppService struct {
	appName    string
	lc         logger.LoggingClient
	dsManage   devicemanager.DeviceManagerInterface
	mapManage  mappingmanager.MappingManagerInterface
	mdbsServer modbusserver.ModbusServerInterface
	register   register.RegisterInterface
	ctx        context.Context
	wg         *sync.WaitGroup
	config     *config.AppConfig
}

func NewAppService(name string, version string) (AppServiceInterface, error) {
	var service AppService
	if name == "" {
		return nil, errors.New("please specify service name")
	}
	service.appName = name
	if version == "" {
		return nil, errors.New("please specify service version")
	}
	return &service, nil
}

func (s *AppService) Initialize() error {
	s.lc = logger.NewClient("DEBUG")
	s.lc.Info("Initializing service:", s.appName)
	// 初始化各个组件
	// 省略具体实现细节
	s.ctx = context.Background()
	s.wg = &sync.WaitGroup{}
	s.lc.Info("Service initialized successfully")
	return nil
}

func (s *AppService) Run() error {
	s.lc.Info("Starting service:", s.appName)
	// 初始化各个组件
	// 省略具体实现细节
	s.lc.Info("Service started successfully")
	return nil
}

func (s *AppService) Stop() error {
	s.lc.Info("Stopping service:", s.appName)
	// 停止各个组件
	// 省略具体实现细节
	s.lc.Info("Service stopped successfully")
	return nil
}

func (s *AppService) GetLoggingClient() logger.LoggingClient {
	return s.lc
}

func (s *AppService) GetDeviceManager() devicemanager.DeviceManagerInterface {
	return s.dsManage
}

func (s *AppService) GetMappingManager() mappingmanager.MappingManagerInterface {
	return s.mapManage
}

func (s *AppService) GetModbusServer() modbusserver.ModbusServerInterface {
	return s.mdbsServer
}

func (s *AppService) GetRegister() register.RegisterInterface {
	return s.register
}

func (s *AppService) GetAppConifg() *config.AppConfig {
	return s.config
}

func (s *AppService) GetContext() context.Context {
	return s.ctx
}

func (s *AppService) GetWaitGroup() *sync.WaitGroup {
	return s.wg
}
