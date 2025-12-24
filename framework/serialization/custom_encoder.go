package serialization

import (
	"fmt"
	"reflect"
	"sync"
)

// CustomEncoder 自定义编码器接口
type CustomEncoder interface {
	// Encode 编码对象
	Encode(v interface{}) ([]byte, error)
	// Decode 解码对象
	Decode(data []byte, v interface{}) error
	// Format 返回格式名称
	Format() Format
}

// CustomEncoderFactory 自定义编码器工厂函数
type CustomEncoderFactory func(options Options) CustomEncoder

// CustomSerializer 自定义序列化器（包装CustomEncoder实现Serializer接口）
type CustomSerializer struct {
	encoder CustomEncoder
	options Options
}

// NewCustomSerializer 创建新的自定义序列化器
func NewCustomSerializer(encoder CustomEncoder, opts ...func(*Options)) *CustomSerializer {
	options := DefaultOptions
	ApplyOptions(&options, opts...)
	return &CustomSerializer{
		encoder: encoder,
		options: options,
	}
}

// Serialize 序列化对象
func (s *CustomSerializer) Serialize(v interface{}) ([]byte, error) {
	if v == nil {
		return nil, ErrNilValue
	}
	return s.encoder.Encode(v)
}

// Deserialize 反序列化对象
func (s *CustomSerializer) Deserialize(data []byte, v interface{}) error {
	if len(data) == 0 {
		return ErrInvalidData
	}
	if v == nil {
		return ErrNilValue
	}
	return s.encoder.Decode(data, v)
}

// ContentType 返回内容类型
func (s *CustomSerializer) ContentType() string {
	return "application/octet-stream" // 自定义编码器通常使用二进制内容类型
}

// Format 返回格式名称
func (s *CustomSerializer) Format() Format {
	return s.encoder.Format()
}

// CustomEncoderRegistry 自定义编码器注册表
type CustomEncoderRegistry struct {
	factories map[Format]CustomEncoderFactory
	encoders  map[Format]CustomEncoder
	mu        sync.RWMutex
}

// NewCustomEncoderRegistry 创建新的自定义编码器注册表
func NewCustomEncoderRegistry() *CustomEncoderRegistry {
	return &CustomEncoderRegistry{
		factories: make(map[Format]CustomEncoderFactory),
		encoders:  make(map[Format]CustomEncoder),
	}
}

// RegisterFactory 注册自定义编码器工厂
func (r *CustomEncoderRegistry) RegisterFactory(format Format, factory CustomEncoderFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[format]; exists {
		return fmt.Errorf("custom encoder factory already registered for format: %s", format)
	}

	r.factories[format] = factory
	return nil
}

// GetEncoder 获取自定义编码器
func (r *CustomEncoderRegistry) GetEncoder(format Format, opts ...func(*Options)) (CustomEncoder, error) {
	r.mu.RLock()
	encoder, exists := r.encoders[format]
	factory, factoryExists := r.factories[format]
	r.mu.RUnlock()

	if exists {
		return encoder, nil
	}

	if factoryExists {
		options := DefaultOptions
		ApplyOptions(&options, opts...)

		encoder := factory(options)

		r.mu.Lock()
		r.encoders[format] = encoder
		r.mu.Unlock()

		return encoder, nil
	}

	return nil, fmt.Errorf("custom encoder not found for format: %s", format)
}

// UnregisterFactory 注销自定义编码器工厂
func (r *CustomEncoderRegistry) UnregisterFactory(format Format) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[format]; !exists {
		return fmt.Errorf("custom encoder factory not found for format: %s", format)
	}

	delete(r.factories, format)
	delete(r.encoders, format)
	return nil
}

// MessagePackEncoder MessagePack编码器示例
type MessagePackEncoder struct {
	options Options
}

// NewMessagePackEncoder 创建新的MessagePack编码器
func NewMessagePackEncoder(opts ...func(*Options)) *MessagePackEncoder {
	options := DefaultOptions
	ApplyOptions(&options, opts...)
	return &MessagePackEncoder{options: options}
}

// Encode 编码对象为MessagePack
func (e *MessagePackEncoder) Encode(v interface{}) ([]byte, error) {
	// 实际实现应使用github.com/vmihailenco/msgpack
	// 这里返回模拟数据
	return []byte("msgpack-mock-data"), nil
}

// Decode 从MessagePack解码对象
func (e *MessagePackEncoder) Decode(data []byte, v interface{}) error {
	// 实际实现应使用github.com/vmihailenco/msgpack
	// 这里模拟成功
	return nil
}

// Format 返回格式名称
func (e *MessagePackEncoder) Format() Format {
	return FormatMsgPack
}

// YAMLEncoder YAML编码器示例
type YAMLEncoder struct {
	options Options
}

// NewYAMLEncoder 创建新的YAML编码器
func NewYAMLEncoder(opts ...func(*Options)) *YAMLEncoder {
	options := DefaultOptions
	ApplyOptions(&options, opts...)
	return &YAMLEncoder{options: options}
}

// Encode 编码对象为YAML
func (e *YAMLEncoder) Encode(v interface{}) ([]byte, error) {
	// 实际实现应使用gopkg.in/yaml.v3
	// 这里返回模拟数据
	return []byte("yaml-mock-data"), nil
}

// Decode 从YAML解码对象
func (e *YAMLEncoder) Decode(data []byte, v interface{}) error {
	// 实际实现应使用gopkg.in/yaml.v3
	// 这里模拟成功
	return nil
}

// Format 返回格式名称
func (e *YAMLEncoder) Format() Format {
	return FormatYAML
}

// XMLEncoder XML编码器示例
type XMLEncoder struct {
	options Options
}

// NewXMLEncoder 创建新的XML编码器
func NewXMLEncoder(opts ...func(*Options)) *XMLEncoder {
	options := DefaultOptions
	ApplyOptions(&options, opts...)
	return &XMLEncoder{options: options}
}

// Encode 编码对象为XML
func (e *XMLEncoder) Encode(v interface{}) ([]byte, error) {
	// 实际实现应使用encoding/xml
	// 这里返回模拟数据
	return []byte("<xml>mock-data</xml>"), nil
}

// Decode 从XML解码对象
func (e *XMLEncoder) Decode(data []byte, v interface{}) error {
	// 实际实现应使用encoding/xml
	// 这里模拟成功
	return nil
}

// Format 返回格式名称
func (e *XMLEncoder) Format() Format {
	return FormatXML
}

// ChainEncoder 链式编码器（支持多个编码器串联）
type ChainEncoder struct {
	encoders []CustomEncoder
	options  Options
}

// NewChainEncoder 创建新的链式编码器
func NewChainEncoder(encoders ...CustomEncoder) *ChainEncoder {
	return &ChainEncoder{
		encoders: encoders,
	}
}

// Encode 使用链式编码器编码对象
func (c *ChainEncoder) Encode(v interface{}) ([]byte, error) {
	var data []byte
	var err error

	for _, encoder := range c.encoders {
		data, err = encoder.Encode(v)
		if err != nil {
			return nil, err
		}
		// 将编码后的数据作为下一个编码器的输入
		// 注意：实际链式编码可能需要不同的处理逻辑
	}

	return data, nil
}

// Decode 使用链式编码器解码对象
func (c *ChainEncoder) Decode(data []byte, v interface{}) error {
	// 反向解码
	for i := len(c.encoders) - 1; i >= 0; i-- {
		encoder := c.encoders[i]
		// 注意：实际链式解码可能需要不同的处理逻辑
		if err := encoder.Decode(data, v); err != nil {
			return err
		}
	}

	return nil
}

// Format 返回格式名称
func (c *ChainEncoder) Format() Format {
	if len(c.encoders) > 0 {
		return c.encoders[0].Format()
	}
	return FormatBinary
}

// Transformer 数据转换器接口
type Transformer interface {
	// Transform 转换数据
	Transform(data []byte) ([]byte, error)
	// Inverse 逆转换
	Inverse(data []byte) ([]byte, error)
}

// TransformingEncoder 带转换器的编码器
type TransformingEncoder struct {
	encoder     CustomEncoder
	transformer Transformer
	options     Options
}

// NewTransformingEncoder 创建新的带转换器的编码器
func NewTransformingEncoder(encoder CustomEncoder, transformer Transformer) *TransformingEncoder {
	return &TransformingEncoder{
		encoder:     encoder,
		transformer: transformer,
	}
}

// Encode 编码并转换对象
func (e *TransformingEncoder) Encode(v interface{}) ([]byte, error) {
	data, err := e.encoder.Encode(v)
	if err != nil {
		return nil, err
	}

	return e.transformer.Transform(data)
}

// Decode 转换并解码对象
func (e *TransformingEncoder) Decode(data []byte, v interface{}) error {
	transformed, err := e.transformer.Inverse(data)
	if err != nil {
		return err
	}

	return e.encoder.Decode(transformed, v)
}

// Format 返回格式名称
func (e *TransformingEncoder) Format() Format {
	return e.encoder.Format()
}

// CompressionTransformer 压缩转换器
type CompressionTransformer struct {
	compression Compression
}

// NewCompressionTransformer 创建新的压缩转换器
func NewCompressionTransformer(compression Compression) *CompressionTransformer {
	return &CompressionTransformer{
		compression: compression,
	}
}

// Transform 压缩数据
func (t *CompressionTransformer) Transform(data []byte) ([]byte, error) {
	return compress(data, t.compression)
}

// Inverse 解压数据
func (t *CompressionTransformer) Inverse(data []byte) ([]byte, error) {
	return decompress(data, t.compression)
}

// EncryptionTransformer 加密转换器
type EncryptionTransformer struct {
	encryption Encryption
}

// NewEncryptionTransformer 创建新的加密转换器
func NewEncryptionTransformer(encryption Encryption) *EncryptionTransformer {
	return &EncryptionTransformer{
		encryption: encryption,
	}
}

// Transform 加密数据
func (t *EncryptionTransformer) Transform(data []byte) ([]byte, error) {
	return encrypt(data, t.encryption)
}

// Inverse 解密数据
func (t *EncryptionTransformer) Inverse(data []byte) ([]byte, error) {
	return decrypt(data, t.encryption)
}

// CustomTypeEncoder 自定义类型编码器
type CustomTypeEncoder struct {
	encodeFunc func(interface{}) ([]byte, error)
	decodeFunc func([]byte, interface{}) error
	format     Format
}

// NewCustomTypeEncoder 创建新的自定义类型编码器
func NewCustomTypeEncoder(
	encodeFunc func(interface{}) ([]byte, error),
	decodeFunc func([]byte, interface{}) error,
	format Format,
) *CustomTypeEncoder {
	return &CustomTypeEncoder{
		encodeFunc: encodeFunc,
		decodeFunc: decodeFunc,
		format:     format,
	}
}

// Encode 编码对象
func (e *CustomTypeEncoder) Encode(v interface{}) ([]byte, error) {
	return e.encodeFunc(v)
}

// Decode 解码对象
func (e *CustomTypeEncoder) Decode(data []byte, v interface{}) error {
	return e.decodeFunc(data, v)
}

// Format 返回格式名称
func (e *CustomTypeEncoder) Format() Format {
	return e.format
}

// DynamicEncoder 动态编码器（根据类型选择编码器）
type DynamicEncoder struct {
	encoders       map[reflect.Type]CustomEncoder
	defaultEncoder CustomEncoder
	options        Options
}

// NewDynamicEncoder 创建新的动态编码器
func NewDynamicEncoder(defaultEncoder CustomEncoder) *DynamicEncoder {
	return &DynamicEncoder{
		encoders:       make(map[reflect.Type]CustomEncoder),
		defaultEncoder: defaultEncoder,
	}
}

// Register 注册类型编码器
func (e *DynamicEncoder) Register(typ reflect.Type, encoder CustomEncoder) {
	e.encoders[typ] = encoder
}

// Encode 动态编码对象
func (e *DynamicEncoder) Encode(v interface{}) ([]byte, error) {
	typ := reflect.TypeOf(v)

	if encoder, exists := e.encoders[typ]; exists {
		return encoder.Encode(v)
	}

	return e.defaultEncoder.Encode(v)
}

// Decode 动态解码对象
func (e *DynamicEncoder) Decode(data []byte, v interface{}) error {
	typ := reflect.TypeOf(v)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	if encoder, exists := e.encoders[typ]; exists {
		return encoder.Decode(data, v)
	}

	return e.defaultEncoder.Decode(data, v)
}

// Format 返回格式名称
func (e *DynamicEncoder) Format() Format {
	return e.defaultEncoder.Format()
}
