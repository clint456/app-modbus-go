package startup

import (
	"app-modbus-go/internal/pkg/service"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// BootStrap initializes and runs the application
func BootStrap(appName string, version string) {
	// Parse command line flags
	configPath := flag.String("c", "", "Path to configuration file")
	flag.Parse()

	// Determine config path
	var cfgPath string
	if *configPath != "" {
		cfgPath = *configPath
	} else {
		// Default: look for res/configuration.yaml relative to executable
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get executable path: %v\n", err)
			os.Exit(-1)
		}
		cfgPath = filepath.Join(filepath.Dir(exe), "res", "configuration.yaml")
	}

	fmt.Printf("Bootstrapping application: %s Version: %s\n", appName, version)

	// Create application service
	appService, err := service.NewAppService(appName, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create application service: %v\n", err)
		os.Exit(-1)
	}

	// Initialize with configuration
	if err := appService.Initialize(cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize application: %v\n", err)
		os.Exit(-1)
	}

	// Run the application
	if err := appService.Run(); err != nil {
		appService.GetLoggingClient().Error("Application run failed:", err)
		os.Exit(-1)
	}

	os.Exit(0)
}
