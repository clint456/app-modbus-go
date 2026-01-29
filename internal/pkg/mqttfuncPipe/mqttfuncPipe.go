package mqttfuncPipe

import (
	"context"
	"log"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// --- 基础类型 ---

type PipelineFunc func(ctx context.Context, data interface{}) (interface{}, error)

// task 包装了消息及其对应的管道逻辑
type task struct {
	pipe *Pipeline
	msg  mqtt.Message
}

// Pipeline 线程安全的函数管道
type Pipeline struct {
	mu    sync.RWMutex
	steps []PipelineFunc
}

func NewPipeline() *Pipeline {
	return &Pipeline{}
}

func (p *Pipeline) AddStep(step PipelineFunc) *Pipeline {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.steps = append(p.steps, step)
	return p
}

func (p *Pipeline) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	p.mu.RLock()
	steps := p.steps
	p.mu.RUnlock()

	var err error
	current := input
	for _, step := range steps {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		current, err = step(ctx, current)
		if err != nil || current == nil {
			return current, err
		}
	}
	return current, nil
}

// --- AppService (EdgeX 风格抽象) ---
type AppService struct {
	processor *MQTTProcessor
	routes    map[string]*Pipeline
	mu        sync.RWMutex
}

func NewAppService(broker string, clientID string, workers int) *AppService {
	proc := &MQTTProcessor{
		msgChan: make(chan *task, 2048),
		workers: workers,
		timeout: 10 * time.Second,
	}

	opts := mqtt.NewClientOptions().AddBroker(broker).SetClientID(clientID)
	opts.SetAutoReconnect(true).SetResumeSubs(true).SetCleanSession(false)
	proc.client = mqtt.NewClient(opts)

	return &AppService{
		processor: proc,
		routes:    make(map[string]*Pipeline),
	}
}

// AddFunctionsPipelineForTopics 声明式绑定主题与处理函数
func (s *AppService) AddFunctionsPipelineForTopics(id string, topics []string, transforms ...PipelineFunc) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. 初始化管道
	pipe := NewPipeline()
	for _, f := range transforms {
		pipe.AddStep(f)
	}

	// 2. 映射路由
	topicMap := make(map[string]byte)
	for _, t := range topics {
		s.routes[t] = pipe
		topicMap[t] = 1
	}

	// 3. 核心：如果 client 存在且未连接，则连接并启动 Worker
	// 这种检查方式允许我们在测试中通过 nil client 跳过真实连接
	if s.processor.client != nil && !s.processor.client.IsConnected() {
		if token := s.processor.client.Connect(); token.Wait() && token.Error() != nil {
			return token.Error()
		}

		// 只有在真实连接时才由系统自动订阅
		token := s.processor.client.SubscribeMultiple(topicMap, s.routeMessage)
		token.Wait()
		if token.Error() != nil {
			return token.Error()
		}
	}

	return nil
}

// 辅助方法：手动启动 Worker (用于测试或手动控制场景)
func (s *AppService) StartWorkers(ctx context.Context) {
	for i := 0; i < s.processor.workers; i++ {
		s.processor.wg.Add(1)
		go s.processor.worker(ctx, i)
	}
}

// routeMessage 核心分发：根据 Topic 路由到对应的 Pipeline
func (s *AppService) routeMessage(c mqtt.Client, m mqtt.Message) {
	s.mu.RLock()
	// 注意：此处可扩展为通配符匹配逻辑
	pipe, ok := s.routes[m.Topic()]
	s.mu.RUnlock()

	if ok {
		select {
		case s.processor.msgChan <- &task{pipe: pipe, msg: m}:
		default:
			log.Printf("Dropped msg from %s (buffer full)", m.Topic())
		}
	}
}

func (s *AppService) Stop() {
	s.processor.Stop()
}

// --- 底层处理器 ---

type MQTTProcessor struct {
	client  mqtt.Client
	msgChan chan *task
	wg      sync.WaitGroup
	workers int
	timeout time.Duration
}

func (p *MQTTProcessor) worker(ctx context.Context, id int) {
	defer p.wg.Done()
	for t := range p.msgChan {
		p.processTask(ctx, t, id)
	}
}

func (p *MQTTProcessor) processTask(parentCtx context.Context, t *task, id int) {
	ctx, cancel := context.WithTimeout(parentCtx, p.timeout)
	defer cancel()

	_, err := t.pipe.Execute(ctx, t.msg.Payload())
	if err != nil {
		log.Printf("[Worker %d] Error on %s: %v", id, t.msg.Topic(), err)
	} else {
		t.msg.Ack()
	}
}

func (p *MQTTProcessor) Stop() {
	if p.client != nil && p.client.IsConnected() {
		p.client.Disconnect(500)
	}
	close(p.msgChan)
	p.wg.Wait()
}
