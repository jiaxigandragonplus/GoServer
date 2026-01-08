package transports

import (
	"io"
	"sync"

	"go.uber.org/zap/zapcore"
)

// Transport 定义日志传输接口
// 任何实现了WriteSyncer接口的对象都可以作为日志传输
type Transport interface {
	zapcore.WriteSyncer
	// Close 关闭传输，释放资源
	Close() error
}

// TransportFactory 传输工厂接口，用于创建传输实例
type TransportFactory interface {
	// Create 创建传输实例
	Create() (Transport, error)
	// Name 返回传输名称
	Name() string
}

// baseTransport 基础传输实现，包装io.Writer
type baseTransport struct {
	writer io.Writer
	mu     sync.Mutex
}

// NewBaseTransport 创建基础传输
func NewBaseTransport(writer io.Writer) Transport {
	return &baseTransport{
		writer: writer,
	}
}

// Write 实现io.Writer接口
func (t *baseTransport) Write(p []byte) (n int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.writer.Write(p)
}

// Sync 实现zapcore.WriteSyncer接口
func (t *baseTransport) Sync() error {
	// 对于大多数writer，Sync是空操作
	if syncer, ok := t.writer.(zapcore.WriteSyncer); ok {
		return syncer.Sync()
	}
	return nil
}

// Close 关闭传输
func (t *baseTransport) Close() error {
	// 如果writer实现了Closer接口，则关闭它
	if closer, ok := t.writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// TransportRegistry 传输注册表，用于管理所有可用的传输
type TransportRegistry struct {
	transports map[string]TransportFactory
	mu         sync.RWMutex
}

// NewTransportRegistry 创建新的传输注册表
func NewTransportRegistry() *TransportRegistry {
	return &TransportRegistry{
		transports: make(map[string]TransportFactory),
	}
}

// Register 注册传输工厂
func (r *TransportRegistry) Register(factory TransportFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transports[factory.Name()] = factory
}

// Get 获取传输工厂
func (r *TransportRegistry) Get(name string) (TransportFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, ok := r.transports[name]
	return factory, ok
}

// List 列出所有已注册的传输名称
func (r *TransportRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.transports))
	for name := range r.transports {
		names = append(names, name)
	}
	return names
}

// DefaultRegistry 默认全局传输注册表
var DefaultRegistry = NewTransportRegistry()

// Register 向默认注册表注册传输工厂
func Register(factory TransportFactory) {
	DefaultRegistry.Register(factory)
}

// Get 从默认注册表获取传输工厂
func Get(name string) (TransportFactory, bool) {
	return DefaultRegistry.Get(name)
}

// List 列出默认注册表中的所有传输名称
func List() []string {
	return DefaultRegistry.List()
}
