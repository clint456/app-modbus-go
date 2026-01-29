package startup

import (
	"app-modbus-go/internal/pkg/service"
	"fmt"
	"os"
)

func BootStrap(appName string, version string) {
	println("Bootstrapping application:", appName, "Version:", version)
	appService, err := service.NewAppService(appName, version)
	if err != nil {
		_, _ = fmt.Fprint(os.Stderr, err.Error())
		os.Exit(-1)
	}
	err = appService.Run()
	if err != nil {
		appService.GetLoggingClient().Error("Application run failed:", err)
		os.Exit(-1)
	}
	os.Exit(0)
}

// 省略具体实现细节
