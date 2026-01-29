package mqttfuncPipe

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- Mock MQTT Message ---
type mockMsg struct {
	topic   string
	payload []byte
}

func (m *mockMsg) Duplicate() bool   { return false }
func (m *mockMsg) Qos() byte         { return 1 }
func (m *mockMsg) Retained() bool    { return false }
func (m *mockMsg) Topic() string     { return m.topic }
func (m *mockMsg) MessageID() uint16 { return 0 }
func (m *mockMsg) Payload() []byte   { return m.payload }
func (m *mockMsg) Ack()              {}

// --- 测试用例 ---
func TestAppService_Routing(t *testing.T) {
	// 使用构造函数初始化，避免内部字段为 nil
	app := NewAppService("tcp://127.0.0.1:1883", "test-client", 2)

	// 记录处理次数
	var floatProcessed, intProcessed int32

	// 模拟注册管道（因为 client 不为 nil，这里会尝试连接，但我们可以接受报错或提前处理）
	app.AddFunctionsPipelineForTopics("FloatPipe", []string{"telemetry/float"},
		func(ctx context.Context, data interface{}) (interface{}, error) {
			atomic.AddInt32(&floatProcessed, 1)
			return data, nil
		},
	)

	app.AddFunctionsPipelineForTopics("IntPipe", []string{"telemetry/int"},
		func(ctx context.Context, data interface{}) (interface{}, error) {
			atomic.AddInt32(&intProcessed, 1)
			return data, nil
		},
	)

	// 注意：NewAppService 内部已经初始化了 msgChan 和 workers
	// 如果你不想启动真实连接，可以手动给 processor.wg 加 1 并启动 worker
	// 这里我们手动启动 worker 来消费管道
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := 0; i < app.processor.workers; i++ {
		app.processor.wg.Add(1)
		go app.processor.worker(ctx, i)
	}

	// 3. 模拟发送消息触发回调
	app.routeMessage(nil, &mockMsg{topic: "telemetry/float", payload: []byte("1.2")})
	app.routeMessage(nil, &mockMsg{topic: "telemetry/int", payload: []byte("10")})
	app.routeMessage(nil, &mockMsg{topic: "telemetry/int", payload: []byte("20")})

	// 4. 断言结果
	assert.Eventually(t, func() bool {
		return atomic.LoadInt32(&floatProcessed) == 1 && atomic.LoadInt32(&intProcessed) == 2
	}, 2*time.Second, 100*time.Millisecond)
}

func TestAppService_HighConcurrency(t *testing.T) {
	var totalProcessed int64
	totalMessages := 20000
	workers := 50

	// 初始化 Service
	app := NewAppService("tcp://mock:1883", "test-client", workers)

	// 重要：为了测试不丢包，我们将缓冲区设为和总消息量一样大，或者稍大一点
	app.processor.msgChan = make(chan *task, totalMessages)

	app.AddFunctionsPipelineForTopics("Bench", []string{"test/bench"},
		func(ctx context.Context, data interface{}) (interface{}, error) {
			atomic.AddInt64(&totalProcessed, 1)
			return data, nil
		},
	)

	// 手动启动 Worker
	ctx, cancel := context.WithCancel(context.Background())
	app.StartWorkers(ctx)

	// 并发压入消息：使用 WaitGroup 确保所有消息都“送达”了通道
	var sendWg sync.WaitGroup
	sendWg.Add(1)
	go func() {
		defer sendWg.Done()
		for i := 0; i < totalMessages; i++ {
			app.routeMessage(nil, &mockMsg{topic: "test/bench", payload: []byte("data")})
		}
	}()

	// 等待发送协程结束
	sendWg.Wait()

	// 检查最终计数：给 5 秒宽限时间处理 2 万条消息
	assert.Eventually(t, func() bool {
		current := atomic.LoadInt64(&totalProcessed)
		return current == int64(totalMessages)
	}, 5*time.Second, 100*time.Millisecond, "应该处理完所有消息")

	// 清理
	cancel()
	app.Stop()
}

func TestAppService_PipelineError(t *testing.T) {
	app := NewAppService("tcp://mock:1883", "err-client", 1)

	errorTriggered := false
	app.AddFunctionsPipelineForTopics("ErrorPipe", []string{"test/error"},
		func(ctx context.Context, data interface{}) (interface{}, error) {
			return nil, fmt.Errorf("business error")
		},
		func(ctx context.Context, data interface{}) (interface{}, error) {
			// 这一步不应该被执行
			errorTriggered = true
			return data, nil
		},
	)

	// 启动 1 个 worker
	app.processor.wg.Add(1)
	go app.processor.worker(context.Background(), 0)

	// 发送消息
	app.routeMessage(nil, &mockMsg{topic: "test/error", payload: []byte("fail")})

	// 等待一小会儿确保执行过
	time.Sleep(100 * time.Millisecond)

	assert.False(t, errorTriggered, "发生错误后管道应立即中断")
	app.Stop()
}
