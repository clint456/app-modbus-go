package mappingmanager

type MappingManagerInterface interface {
	GetMapping() error
	UpdateMapping() error
	ValidateMapping() error
	SendMapping() error
}
