package bootstarp

type BootStrapInterface interface {
	Initialize() error
	Start() error
	Stop() error
}
