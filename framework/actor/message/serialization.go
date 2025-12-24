package message

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"
)

// Serializer 序列化器接口
type Serializer interface {
	// Serialize 序列化消息
	Serialize(msg Message) ([]byte, error)
	// Deserialize 反序列化消息
	Deserialize(data []byte) (Message, error)
	// ContentType 返回内容类型
	ContentType() string
}

// SerializationFormat 序列化格式
type SerializationFormat string

const (
	// FormatJSON JSON格式
	FormatJSON SerializationFormat = "json"
	// FormatBinary 二进制格式
	FormatBinary SerializationFormat = "binary"
	// FormatProtobuf Protobuf格式
	FormatProtobuf SerializationFormat = "protobuf"
)

// JSONSerializer JSON序列化器
type JSONSerializer struct{}

// NewJSONSerializer 创建新的JSON序列化器
func NewJSONSerializer() *JSONSerializer {
	return &JSONSerializer{}
}

// Serialize 序列化消息为JSON
func (s *JSONSerializer) Serialize(msg Message) ([]byte, error) {
	if msg == nil {
		return nil, fmt.Errorf("message is nil")
	}

	// 创建序列化结构体
	serializable := &serializableMessage{
		Type:      msg.Type(),
		Data:      msg.Data(),
		Sender:    addressToString(msg.Sender()),
		Receiver:  addressToString(msg.Receiver()),
		Timestamp: msg.Timestamp(),
		ID:        msg.ID(),
		Priority:  int(msg.Priority()),
		TTL:       msg.TTL(),
		Headers:   msg.Headers(),
	}

	return json.Marshal(serializable)
}

// Deserialize 从JSON反序列化消息
func (s *JSONSerializer) Deserialize(data []byte) (Message, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data is empty")
	}

	var serializable serializableMessage
	if err := json.Unmarshal(data, &serializable); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// 创建消息
	msg := NewBaseMessage(serializable.Type, serializable.Data)
	msg.SetID(serializable.ID)
	msg.SetTimestamp(serializable.Timestamp)
	msg.SetPriority(Priority(serializable.Priority))
	msg.SetTTL(serializable.TTL)

	// 设置地址
	if serializable.Sender != "" {
		sender, err := ParseAddress(serializable.Sender)
		if err == nil {
			msg.SetSender(sender)
		}
	}

	if serializable.Receiver != "" {
		receiver, err := ParseAddress(serializable.Receiver)
		if err == nil {
			msg.SetReceiver(receiver)
		}
	}

	// 设置头部
	for k, v := range serializable.Headers {
		msg.SetHeader(k, v)
	}

	return msg, nil
}

// ContentType 返回内容类型
func (s *JSONSerializer) ContentType() string {
	return "application/json"
}

// serializableMessage 可序列化的消息结构
type serializableMessage struct {
	Type      string            `json:"type"`
	Data      interface{}       `json:"data"`
	Sender    string            `json:"sender,omitempty"`
	Receiver  string            `json:"receiver,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	ID        string            `json:"id"`
	Priority  int               `json:"priority"`
	TTL       time.Duration     `json:"ttl"`
	Headers   map[string]string `json:"headers,omitempty"`
}

// addressToString 地址转字符串
func addressToString(addr Address) string {
	if addr == nil {
		return ""
	}
	return addr.String()
}

// BinarySerializer 二进制序列化器
type BinarySerializer struct {
	// 简化实现，实际应用中应该实现更高效的二进制编码
}

// NewBinarySerializer 创建新的二进制序列化器
func NewBinarySerializer() *BinarySerializer {
	return &BinarySerializer{}
}

// Serialize 序列化消息为二进制
func (s *BinarySerializer) Serialize(msg Message) ([]byte, error) {
	// 简化实现：使用JSON作为中间格式
	jsonSerializer := NewJSONSerializer()
	return jsonSerializer.Serialize(msg)
}

// Deserialize 从二进制反序列化消息
func (s *BinarySerializer) Deserialize(data []byte) (Message, error) {
	// 简化实现：使用JSON作为中间格式
	jsonSerializer := NewJSONSerializer()
	return jsonSerializer.Deserialize(data)
}

// ContentType 返回内容类型
func (s *BinarySerializer) ContentType() string {
	return "application/octet-stream"
}

// SerializerRegistry 序列化器注册表
type SerializerRegistry struct {
	serializers map[SerializationFormat]Serializer
	mu          sync.RWMutex
}

// NewSerializerRegistry 创建新的序列化器注册表
func NewSerializerRegistry() *SerializerRegistry {
	registry := &SerializerRegistry{
		serializers: make(map[SerializationFormat]Serializer),
	}

	// 注册默认序列化器
	registry.Register(FormatJSON, NewJSONSerializer())
	registry.Register(FormatBinary, NewBinarySerializer())

	return registry
}

// Register 注册序列化器
func (r *SerializerRegistry) Register(format SerializationFormat, serializer Serializer) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.serializers[format]; exists {
		return fmt.Errorf("serializer already registered for format: %s", format)
	}

	r.serializers[format] = serializer
	return nil
}

// Get 获取序列化器
func (r *SerializerRegistry) Get(format SerializationFormat) (Serializer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	serializer, exists := r.serializers[format]
	if !exists {
		return nil, fmt.Errorf("serializer not found for format: %s", format)
	}

	return serializer, nil
}

// Unregister 注销序列化器
func (r *SerializerRegistry) Unregister(format SerializationFormat) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.serializers[format]; !exists {
		return fmt.Errorf("serializer not found for format: %s", format)
	}

	delete(r.serializers, format)
	return nil
}

// DefaultSerializerRegistry 默认序列化器注册表
var (
	defaultSerializerRegistry     *SerializerRegistry
	defaultSerializerRegistryOnce sync.Once
)

// GetDefaultSerializerRegistry 获取默认序列化器注册表
func GetDefaultSerializerRegistry() *SerializerRegistry {
	defaultSerializerRegistryOnce.Do(func() {
		defaultSerializerRegistry = NewSerializerRegistry()
	})
	return defaultSerializerRegistry
}

// MessageCodec 消息编解码器
type MessageCodec struct {
	registry *SerializerRegistry
}

// NewMessageCodec 创建新的消息编解码器
func NewMessageCodec(registry *SerializerRegistry) *MessageCodec {
	if registry == nil {
		registry = GetDefaultSerializerRegistry()
	}
	return &MessageCodec{registry: registry}
}

// Encode 编码消息
func (c *MessageCodec) Encode(msg Message, format SerializationFormat) ([]byte, error) {
	serializer, err := c.registry.Get(format)
	if err != nil {
		return nil, err
	}

	return serializer.Serialize(msg)
}

// Decode 解码消息
func (c *MessageCodec) Decode(data []byte, format SerializationFormat) (Message, error) {
	serializer, err := c.registry.Get(format)
	if err != nil {
		return nil, err
	}

	return serializer.Deserialize(data)
}

// AutoDecode 自动解码消息（根据内容类型）
func (c *MessageCodec) AutoDecode(data []byte, contentType string) (Message, error) {
	// 根据内容类型确定格式
	var format SerializationFormat
	switch contentType {
	case "application/json":
		format = FormatJSON
	case "application/octet-stream":
		format = FormatBinary
	case "application/protobuf":
		format = FormatProtobuf
	default:
		// 尝试JSON
		format = FormatJSON
	}

	return c.Decode(data, format)
}

// TypeRegistry 类型注册表，用于支持自定义类型的序列化
type TypeRegistry struct {
	types map[string]reflect.Type
	mu    sync.RWMutex
}

// NewTypeRegistry 创建新的类型注册表
func NewTypeRegistry() *TypeRegistry {
	return &TypeRegistry{
		types: make(map[string]reflect.Type),
	}
}

// Register 注册类型
func (r *TypeRegistry) Register(typeName string, typ reflect.Type) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.types[typeName]; exists {
		return fmt.Errorf("type already registered: %s", typeName)
	}

	r.types[typeName] = typ
	return nil
}

// Get 获取类型
func (r *TypeRegistry) Get(typeName string) (reflect.Type, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	typ, exists := r.types[typeName]
	if !exists {
		return nil, fmt.Errorf("type not found: %s", typeName)
	}

	return typ, nil
}

// Unregister 注销类型
func (r *TypeRegistry) Unregister(typeName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.types[typeName]; !exists {
		return fmt.Errorf("type not found: %s", typeName)
	}

	delete(r.types, typeName)
	return nil
}
