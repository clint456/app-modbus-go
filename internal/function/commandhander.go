package functions

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"

	"app-demo-go/internal/pkg/logger"

	"github.com/edgexfoundry/app-functions-sdk-go/v4/pkg/interfaces"
)

// CommandHandler MessageBus控制指令处理器
type CommandHandler struct {
	logger logger.LoggingClient
}

// CommandRequest 控制指令请求结构
type CommandRequest struct {
	Command string                 `json:"command"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// CommandResponse 控制指令响应结构
type CommandResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// NewCommandHandler 创建新的控制指令处理器
func NewCommandHandler(
	logger logger.LoggingClient,
) *CommandHandler {
	return &CommandHandler{
		logger: logger,
	}
}

// ProcessControlCommand 处理控制指令 - EdgeX函数管道入口
func (ch *CommandHandler) ProcessControlCommand(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {
	if ch == nil {
		return false, errors.New("CommandHandler is nil")
	}
	if ch.logger == nil {
		// 如果没有 logger，降级使用标准日志，避免 panic
		log.Printf("warning: logger 未初始化，继续处理")
	}
	ch.logger.Debug("开始处理MessageBus控制指令")
	ch.logger.Infof("接收到MessageBus :%s ,类型:%v", data, reflect.TypeOf(data))

	// 解析EdgeX事件
	cmdRequest, ok := data.(CommandRequest)
	if !ok {
		ch.logger.Error("数据类型错误，期望CommandRequest")
		response := &CommandResponse{
			Success: false,
			Error:   "数据类型错误",
		}
		return ch.setResponseData(ctx, response)
	}

	// 从事件中提取控制指令
	if len(cmdRequest.Command) == 0 {
		ch.logger.Error("事件中没有控制指令")
		response := &CommandResponse{
			Success: false,
			Error:   "事件中没有控制指令",
		}
		return ch.setResponseData(ctx, response)
	}

	ch.logger.Infof("接收到控制指令: %s", cmdRequest.Command)

	// 处理具体的控制指令
	response := ch.handleCommand(&cmdRequest)

	// 设置响应数据
	return ch.setResponseData(ctx, response)
}

// handleCommand 处理具体的控制指令
func (h *CommandHandler) handleCommand(request *CommandRequest) *CommandResponse {
	switch strings.ToLower(request.Command) {
	case "get_version":
		return h.handleGetVersion(request.Data)
	default:
		return &CommandResponse{
			Success: false,
			Error:   fmt.Sprintf("未知的控制指令: %s", request.Command),
		}
	}
}

// handleGetVersion 处理获取版本号指令
func (h *CommandHandler) handleGetVersion(params map[string]interface{}) *CommandResponse {
	h.logger.Info("处理获取版本号指令")
	// 模拟从硬件获取版本信息
	versionData := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	h.logger.Info("版本信息获取成功")
	return &CommandResponse{
		Success: true,
		Message: "版本信息获取成功",
		Data: map[string]interface{}{
			"hardware_version": string(versionData),
			"software_version": "app-demo-go v0.0.0",
			"version_data":     fmt.Sprintf("%x", versionData), // 十六进制格式
		},
	}
}

// setResponseData 设置响应数据到EdgeX上下文
func (h *CommandHandler) setResponseData(ctx interfaces.AppFunctionContext, response *CommandResponse) (bool, interface{}) {
	// 序列化响应数据
	responseData, err := json.Marshal(response)
	if err != nil {
		h.logger.Errorf("序列化响应数据失败: %v", err)
		return false, fmt.Errorf("序列化响应数据失败: %v", err)
	}

	// 设置响应数据，这将通过MessageBus发送回调用方
	ctx.SetResponseData(responseData)
	ctx.SetResponseContentType("application/json")

	h.logger.Debugf("设置响应数据: %s", string(responseData))
	return true, response
}
