package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"app-demo-go/internal/app"
	functions "app-demo-go/internal/function"

	"github.com/edgexfoundry/app-functions-sdk-go/v4/pkg"
	"github.com/edgexfoundry/app-functions-sdk-go/v4/pkg/interfaces"
)

const (
	// serviceKey EdgeX应用服务的唯一标识符
	serviceKey = "app-demo-go"
)

// createAndRunAppService 创建并运行应用服务
func createAndRunAppService(serviceKey string, appService app.AppSerice, targetType interface{},
	newServiceFactory func(string, interface{}) (interfaces.ApplicationService, bool)) int {

	// 初始化EdgeX应用服务框架
	// 创建EdgeX服务实例并获取日志客户端, 导入自定义TargetType
	fmt.Println("开始初始化EdgeX服务...")
	if err := appService.InitializeEdgeXService(serviceKey, targetType, newServiceFactory); err != nil {
		fmt.Printf("初始化EdgeX服务失败: %v\n", err)
		return -1
	}
	fmt.Println("EdgeX服务初始化成功")

	// 加载和验证配置
	// 从EdgeX配置系统加载自定义配置并验证有效性
	// 导入监听配置变更回调函数
	fmt.Println("开始加载配置...")
	if err := appService.LoadAndValidateConfig(); err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		return -1
	}
	fmt.Println("配置加载成功")

	// 创建控制指令处理器
	appService.CommandHandler = functions.NewCommandHandler(appService.Lc)
	// 配置函数管道
	if err := appService.SetupPipelines(); err != nil {
		return -1
	}

	// 创建信号通道监听系统终止信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 在goroutine中运行EdgeX服务
	errChan := make(chan error, 1)
	go func() {
		// 启动HTTP服务器、消息总线监听等，进入主事件循环
		if err := appService.Service.Run(); err != nil {
			errChan <- err
		}
	}()

	// 等待终止信号或服务错误
	select {
	case sig := <-sigChan:
		appService.Lc.Infof("收到信号 %v，开始优雅关闭...", sig)
	case err := <-errChan:
		appService.Lc.Errorf("服务运行失败: %s", err.Error())
		return -1
	}

	// 执行优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := appService.Shutdown(shutdownCtx); err != nil {
		appService.Lc.Errorf("优雅关闭失败: %s", err.Error())
		return -1
	}

	appService.Lc.Info("服务已安全退出")
	appService.Lc.Close()
	return 0
}

// main 应用程序入口点
// 创建并运行EdgeX加密应用服务，导入自定义加密命令结构体
func main() {
	app := app.AppSerice{}
	code := createAndRunAppService(serviceKey, app, &functions.CommandRequest{}, pkg.NewAppServiceWithTargetType)
	os.Exit(code)
}
