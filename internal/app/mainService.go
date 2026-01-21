package app

import (
	"app-demo-go/config"
	functions "app-demo-go/internal/function"
	"context"
	"fmt"
	"reflect"

	"app-demo-go/internal/pkg/logger"

	"github.com/edgexfoundry/app-functions-sdk-go/v4/pkg/interfaces"
)

type AppSerice struct {
	Service        interfaces.ApplicationService
	Lc             logger.LoggingClient
	AppCtx         context.Context
	ServiceConfig  *config.ServiceConfig
	CommandHandler *functions.CommandHandler
}

// setupPipelines 设置数据处理管道 - 仅处理MessageBus控制指令
func (app *AppSerice) SetupPipelines() error {
	app.Lc.Info("设置MessageBus控制指令处理管道...")

	// 检查管道是否启用
	if !app.ServiceConfig.AppCustom.Pipelines.Enabled {
		app.Lc.Warn("Pipelines 未启用，跳过管道设置")
		return nil
	}

	// 设置控制指令处理管道
	controlTopics := app.ServiceConfig.AppCustom.Pipelines.Topics
	if controlTopics == nil {
		// 使用默认控制主题
		app.Lc.Warn("Pipelines.Topics 为空，使用默认主题")
		controlTopics = []string{"command/demo/control"}
	}

	app.Lc.Infof("订阅控制主题: %v", controlTopics)

	err := app.Service.AddFunctionsPipelineForTopics(
		"ControlCommands",
		controlTopics,
		app.CommandHandler.ProcessControlCommand)
	if err != nil {
		app.Lc.Errorf("添加控制指令管道失败: %s", err.Error())
		return err
	}

	app.Lc.Info("MessageBus控制指令管道设置成功")
	return nil
}

// InitializeEdgeXService 初始化EdgeX应用服务
func (app *AppSerice) InitializeEdgeXService(serviceKey string, targetType interface{},
	newServiceFactory func(string, interface{}) (interfaces.ApplicationService, bool)) error {
	var ok bool
	app.Service, ok = newServiceFactory(serviceKey, targetType)
	if !ok {
		return fmt.Errorf("创建EdgeX应用服务失败")
	}
	return nil
}

// LoadAndValidateConfig 加载和验证配置
func (app *AppSerice) LoadAndValidateConfig() error {
	// 获取设备名称配置
	_, err := app.Service.GetAppSettingStrings("DeviceNames")
	if err != nil {
		return err
	}

	// 加载自定义配置
	app.ServiceConfig = &config.ServiceConfig{}
	if err := app.Service.LoadCustomConfig(app.ServiceConfig, "AppCustom"); err != nil {
		return err
	}

	// 自定义日志客户端配置
	app.Lc = logger.NewClient("DEBUG")

	app.Lc.Info(logger.Logo)
	app.Lc.Info("EdgeX应用服务初始化成功")

	// 记录加载的配置信息，便于排查问题
	app.Lc.Debugf("已加载配置: Pipelines.Enabled=%v, Topics=%v (类型: %T)",
		app.ServiceConfig.AppCustom.Pipelines.Enabled,
		app.ServiceConfig.AppCustom.Pipelines.Topics,
		app.ServiceConfig.AppCustom.Pipelines.Topics)

	// 验证配置有效性
	if err := app.ServiceConfig.AppCustom.Validate(); err != nil {
		app.Lc.Errorf("配置验证失败: %s", err.Error())
		app.Lc.Error("提示: 如使用 Consul，请确保 Pipelines.Topics 格式为 [\"topic1\", \"topic2\"]，而非 {0:\"topic1\"}")
		return err
	}

	// 监听配置变更，导入配置变更回调函数
	if err := app.Service.ListenForCustomConfigChanges(&app.ServiceConfig.AppCustom,
		"AppCustom", app.ProcessConfigUpdates); err != nil {
		app.Lc.Errorf("监听配置变更失败: %s", err.Error())
		return err
	}

	app.Lc.Info("配置加载和验证完成")
	return nil
}

// ProcessConfigUpdates 处理配置更新
// 当配置发生变更时，此函数会被调用以处理配置更新
func (app *AppSerice) ProcessConfigUpdates(rawWritableConfig interface{}) {
	updated, ok := rawWritableConfig.(*config.AppCustomConfig)
	if !ok {
		app.Lc.Error("无法处理配置更新: 无法转换配置类型为 'AppCustomConfig'")
		return
	}

	// 先验证新配置的有效性，避免应用错误配置
	if err := updated.Validate(); err != nil {
		app.Lc.Errorf("配置更新验证失败，保留旧配置: %v", err)
		app.Lc.Warn("提示: 检查 Consul 中的配置格式，特别是 Pipelines.Topics 必须是字符串数组")
		return
	}

	previous := app.ServiceConfig.AppCustom
	app.ServiceConfig.AppCustom = *updated

	if reflect.DeepEqual(previous, updated) {
		app.Lc.Info("未检测到配置变更")
		return
	}

	app.Lc.Info("配置更新处理完成")
}

// Shutdown 优雅关闭服务
func (app *AppSerice) Shutdown(ctx context.Context) error {
	app.Lc.Info("开始优雅关闭服务...")
	// 模拟关闭应用服务依赖程序
	app.Lc.Info("服务关闭完成")
	return nil
}
