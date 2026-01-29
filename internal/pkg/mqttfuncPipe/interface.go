package mqttfuncPipe

import "context"

type MqttServiceInterface interface {
	NewAppService(broker string, clientID string, workers int) *AppService
	AddFunctionsPipelineForTopics(pipeName string, topics []string, funcs ...PipelineFunc) error
	StartWorkers(ctx context.Context)
	Stop()
}
