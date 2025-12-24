package serialization

import (
	"fmt"
	"sync"
)

// SerializerRegistry 序列化器注册表
type SerializerRegistry struct {
	serializers map[Format]Serializer
	factories   map[Format]SerializerFactory
	mu          sync.RWMutex
}

// NewSerializerRegistry 创建新的序列化器注册表
func NewSerializerRegistry() *SerializerRegistry {
	registry := &SerializerRegistry{
		serializers: make(map[Format]Serializer),
		factories:   make(map[Format]SerializerFactory),
	}

	// 注册默认序列化器工厂
	registry.RegisterFactory(FormatJSON, func(options Options) Serializer {
		return NewJSONSerializer(WithIndent(options.Indent))
	})

	registry.RegisterFactory(FormatBinary, func(options Options) Serializer {
		return NewBinarySerializer(
			WithCompression(options.Compression),
			WithEncryption(options.Encryption),
		)
	})

	registry.RegisterFactory(FormatProtobuf, func(options Options) Serializer {
		return NewProtobufSerializer()
	})

	return registry
}

// Register 注册序列化器
func (r *SerializerRegistry) Register(format Format, serializer Serializer) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.serializers[format]; exists {
		return fmt.Errorf("serializer already registered for format: %s", format)
	}

	r.serializers[format] = serializer
	return nil
}

// RegisterFactory 注册序列化器工厂
func (r *SerializerRegistry) RegisterFactory(format Format, factory SerializerFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[format]; exists {
		return fmt.Errorf("serializer factory already registered for format: %s", format)
	}

	r.factories[format] = factory
	return nil
}

// Get 获取序列化器
func (r *SerializerRegistry) Get(format Format, opts ...func(*Options)) (Serializer, error) {
	// 首先检查已创建的序列化器
	r.mu.RLock()
	serializer, exists := r.serializers[format]
	factory, factoryExists := r.factories[format]
	r.mu.RUnlock()

	if exists {
		return serializer, nil
	}

	// 如果存在工厂，创建新的序列化器
	if factoryExists {
		options := DefaultOptions
		ApplyOptions(&options, opts...)

		serializer := factory(options)

		r.mu.Lock()
		r.serializers[format] = serializer
		r.mu.Unlock()

		return serializer, nil
	}

	return nil, ErrSerializerNotFound
}

// GetWithOptions 获取序列化器（带选项）
func (r *SerializerRegistry) GetWithOptions(format Format, options Options) (Serializer, error) {
	// 对于需要特定选项的情况，总是创建新的序列化器
	r.mu.RLock()
	factory, factoryExists := r.factories[format]
	r.mu.RUnlock()

	if factoryExists {
		return factory(options), nil
	}

	return nil, ErrSerializerNotFound
}

// Unregister 注销序列化器
func (r *SerializerRegistry) Unregister(format Format) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.serializers[format]; !exists {
		if _, factoryExists := r.factories[format]; !factoryExists {
			return fmt.Errorf("serializer not found for format: %s", format)
		}
	}

	delete(r.serializers, format)
	delete(r.factories, format)
	return nil
}

// List 列出所有支持的格式
func (r *SerializerRegistry) List() []Format {
	r.mu.RLock()
	defer r.mu.RUnlock()

	formats := make([]Format, 0, len(r.serializers)+len(r.factories))

	// 添加已注册的序列化器
	for format := range r.serializers {
		formats = append(formats, format)
	}

	// 添加有工厂的格式
	for format := range r.factories {
		// 避免重复
		found := false
		for _, f := range formats {
			if f == format {
				found = true
				break
			}
		}
		if !found {
			formats = append(formats, format)
		}
	}

	return formats
}

// Has 检查是否支持指定格式
func (r *SerializerRegistry) Has(format Format) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.serializers[format]
	_, factoryExists := r.factories[format]

	return exists || factoryExists
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

// Serialize 使用默认注册表序列化对象
func Serialize(v interface{}, format Format, opts ...func(*Options)) ([]byte, error) {
	registry := GetDefaultSerializerRegistry()
	serializer, err := registry.Get(format, opts...)
	if err != nil {
		return nil, err
	}
	return serializer.Serialize(v)
}

// Deserialize 使用默认注册表反序列化对象
func Deserialize(data []byte, format Format, v interface{}, opts ...func(*Options)) error {
	registry := GetDefaultSerializerRegistry()
	serializer, err := registry.Get(format, opts...)
	if err != nil {
		return err
	}
	return serializer.Deserialize(data, v)
}

// AutoDetectFormat 自动检测格式
func AutoDetectFormat(data []byte, contentType string) Format {
	// 根据内容类型检测格式
	switch contentType {
	case "application/json":
		return FormatJSON
	case "application/octet-stream":
		return FormatBinary
	case "application/protobuf":
		return FormatProtobuf
	case "application/x-msgpack":
		return FormatMsgPack
	case "application/x-yaml", "text/yaml":
		return FormatYAML
	case "application/xml", "text/xml":
		return FormatXML
	default:
		// 尝试根据数据内容检测
		if len(data) > 0 {
			// 检查是否为JSON
			if data[0] == '{' || data[0] == '[' {
				return FormatJSON
			}
			// 检查是否为XML
			if len(data) >= 5 && string(data[:5]) == "<?xml" {
				return FormatXML
			}
		}
		return FormatBinary
	}
}

// Codec 编解码器
type Codec struct {
	registry *SerializerRegistry
}

// NewCodec 创建新的编解码器
func NewCodec(registry *SerializerRegistry) *Codec {
	if registry == nil {
		registry = GetDefaultSerializerRegistry()
	}
	return &Codec{registry: registry}
}

// Encode 编码
func (c *Codec) Encode(v interface{}, format Format, opts ...func(*Options)) ([]byte, error) {
	return Serialize(v, format, opts...)
}

// Decode 解码
func (c *Codec) Decode(data []byte, format Format, v interface{}, opts ...func(*Options)) error {
	return Deserialize(data, format, v, opts...)
}

// AutoDecode 自动解码（根据内容类型）
func (c *Codec) AutoDecode(data []byte, contentType string, v interface{}, opts ...func(*Options)) error {
	format := AutoDetectFormat(data, contentType)
	return c.Decode(data, format, v, opts...)
}

// MultiCodec 多格式编解码器
type MultiCodec struct {
	codecs map[Format]*Codec
}

// NewMultiCodec 创建新的多格式编解码器
func NewMultiCodec() *MultiCodec {
	return &MultiCodec{
		codecs: make(map[Format]*Codec),
	}
}

// GetCodec 获取指定格式的编解码器
func (m *MultiCodec) GetCodec(format Format) *Codec {
	if codec, exists := m.codecs[format]; exists {
		return codec
	}

	codec := NewCodec(nil)
	m.codecs[format] = codec
	return codec
}

// Encode 编码
func (m *MultiCodec) Encode(v interface{}, format Format, opts ...func(*Options)) ([]byte, error) {
	return m.GetCodec(format).Encode(v, format, opts...)
}

// Decode 解码
func (m *MultiCodec) Decode(data []byte, format Format, v interface{}, opts ...func(*Options)) error {
	return m.GetCodec(format).Decode(data, format, v, opts...)
}

// RegistryBuilder 注册表构建器
type RegistryBuilder struct {
	registry *SerializerRegistry
}

// NewRegistryBuilder 创建新的注册表构建器
func NewRegistryBuilder() *RegistryBuilder {
	return &RegistryBuilder{
		registry: NewSerializerRegistry(),
	}
}

// WithJSON 添加JSON序列化器
func (b *RegistryBuilder) WithJSON(opts ...func(*Options)) *RegistryBuilder {
	b.registry.RegisterFactory(FormatJSON, func(options Options) Serializer {
		mergedOpts := append([]func(*Options){WithIndent(options.Indent)}, opts...)
		return NewJSONSerializer(mergedOpts...)
	})
	return b
}

// WithBinary 添加二进制序列化器
func (b *RegistryBuilder) WithBinary(opts ...func(*Options)) *RegistryBuilder {
	b.registry.RegisterFactory(FormatBinary, func(options Options) Serializer {
		mergedOpts := append([]func(*Options){
			WithCompression(options.Compression),
			WithEncryption(options.Encryption),
		}, opts...)
		return NewBinarySerializer(mergedOpts...)
	})
	return b
}

// WithProtobuf 添加Protobuf序列化器
func (b *RegistryBuilder) WithProtobuf(opts ...func(*Options)) *RegistryBuilder {
	b.registry.RegisterFactory(FormatProtobuf, func(options Options) Serializer {
		return NewProtobufSerializer(opts...)
	})
	return b
}

// WithCustom 添加自定义序列化器
func (b *RegistryBuilder) WithCustom(format Format, factory SerializerFactory) *RegistryBuilder {
	b.registry.RegisterFactory(format, factory)
	return b
}

// Build 构建注册表
func (b *RegistryBuilder) Build() *SerializerRegistry {
	return b.registry
}
