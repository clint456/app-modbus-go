package main

import (
	app "app-modbus-go"
	"app-modbus-go/internal/pkg/startup"
	"fmt"
)

const AppName = "app-modbus-go"

func main() {
	fmt.Printf("Starting application: %s with app instance: %+v\n", AppName, app.Version)
	startup.BootStrap(AppName, app.Version)
}
