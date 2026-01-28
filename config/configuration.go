package config

// This file contains example of custom configuration that can be loaded from the service's configuration.yaml
// and/or the Configuration Provider, aka keeper (if enabled).
// For more details see https://docs.edgexfoundry.org/latest/microservices/application/GeneralAppServiceConfig/#custom-configuration

import (
	"errors"
)

// ServiceConfig 服务配置结构
type ServiceConfig struct {
	AppCustom AppCustomConfig `yaml:"AppCustom"`
}

// AppCustomConfig 加密服务自定义配置
type AppCustomConfig struct {
	Pipelines PipelinesConfig `yaml:"Pipelines"`
}

// PipelinesConfig 管道配置
type PipelinesConfig struct {
	Enabled bool     `yaml:"Enabled"`
	Topics  []string `yaml:"Topics"` // MQTT 主题数组
}

// UpdateFromRaw 从服务提供商接收的原始数据更新服务的完整配置。
// 这里的ServiceConfig将会被 LoadCustomConfig(config interfaces.UpdatableConfig, sectionName string)调用
// 这个config是一个接口并且要求实现UpdateFromRaw方法
// 如果转换失败，返回false
func (c *ServiceConfig) UpdateFromRaw(rawConfig interface{}) bool {
	configuration, ok := rawConfig.(*ServiceConfig)
	if !ok {
		return false //errors.New("unable to cast raw config to type 'ServiceConfig'")
	}

	*c = *configuration

	return true
}

// Validate 验证自定义配置的有效性
func (ac *AppCustomConfig) Validate() error {
	// 验证管道配置
	if ac.Pipelines.Enabled {
		if ac.Pipelines.Topics == nil {
			return errors.New("控制管道启用时 Topics 不能为 nil")
		}
		if len(ac.Pipelines.Topics) == 0 {
			return errors.New("控制管道启用时必须指定订阅主题")
		}
	}

	return nil
}
