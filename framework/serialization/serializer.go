package serialization

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// Serializer 序列化器接口
type Serializer interface {
	// Serialize 序列化任意对象
	Serialize(v interface{}) ([]byte, error)
	// Deserialize 反序列化到对象
	Deserialize(data []byte, v interface{}) error
	// ContentType 返回内容类型
	ContentType() string
	// Format 返回格式名称
	Format() Format
}

// Encoder 编码器接口
type Encoder interface {
	// Encode 编码对象
	Encode(v interface{}) error
	// Bytes 获取编码后的字节
	Bytes() []byte
}

// Decoder 解码器接口
type Decoder interface {
	// Decode 解码到对象
	Decode(v interface{}) error
	// Reset 重置解码器
	Reset(data []byte)
}

// Format 序列化格式
type Format string

const (
	// FormatJSON JSON格式
	FormatJSON Format = "json"
	// FormatBinary 二进制格式
	FormatBinary Format = "binary"
	// FormatProtobuf Protobuf格式
	FormatProtobuf Format = "protobuf"
	// FormatMsgPack MessagePack格式
	FormatMsgPack Format = "msgpack"
	// FormatYAML YAML格式
	FormatYAML Format = "yaml"
	// FormatXML XML格式
	FormatXML Format = "xml"
)

// Options 序列化选项
type Options struct {
	// Indent 缩进（用于JSON等格式）
	Indent string
	// EscapeHTML 转义HTML（用于JSON）
	EscapeHTML bool
	// UseNumber 使用数字类型（用于JSON）
	UseNumber bool
	// DisallowUnknownFields 不允许未知字段（用于JSON）
	DisallowUnknownFields bool
	// Compression 压缩算法
	Compression Compression
	// Encryption 加密算法
	Encryption Encryption
	// TagName 结构体标签名
	TagName string
}

// Compression 压缩算法
type Compression string

const (
	// CompressionNone 无压缩
	CompressionNone Compression = "none"
	// CompressionGzip Gzip压缩
	CompressionGzip Compression = "gzip"
	// CompressionZlib Zlib压缩
	CompressionZlib Compression = "zlib"
	// CompressionSnappy Snappy压缩
	CompressionSnappy Compression = "snappy"
)

// Encryption 加密算法
type Encryption string

const (
	// EncryptionNone 无加密
	EncryptionNone Encryption = "none"
	// EncryptionAES AES加密
	EncryptionAES Encryption = "aes"
	// EncryptionRSA RSA加密
	EncryptionRSA Encryption = "rsa"
)

// TypeRegistry 类型注册表，用于支持自定义类型的序列化
type TypeRegistry struct {
	types map[string]reflect.Type
	names map[reflect.Type]string
	mu    sync.RWMutex
}

// NewTypeRegistry 创建新的类型注册表
func NewTypeRegistry() *TypeRegistry {
	return &TypeRegistry{
		types: make(map[string]reflect.Type),
		names: make(map[reflect.Type]string),
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
	r.names[typ] = typeName
	return nil
}

// RegisterInstance 注册类型实例
func (r *TypeRegistry) RegisterInstance(typeName string, instance interface{}) error {
	typ := reflect.TypeOf(instance)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	return r.Register(typeName, typ)
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

// GetName 获取类型名称
func (r *TypeRegistry) GetName(typ reflect.Type) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	name, exists := r.names[typ]
	if !exists {
		return "", fmt.Errorf("type not registered: %v", typ)
	}

	return name, nil
}

// Unregister 注销类型
func (r *TypeRegistry) Unregister(typeName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	typ, exists := r.types[typeName]
	if !exists {
		return fmt.Errorf("type not found: %s", typeName)
	}

	delete(r.types, typeName)
	delete(r.names, typ)
	return nil
}

// Serializable 可序列化接口
type Serializable interface {
	// Serialize 序列化自身
	Serialize() ([]byte, error)
	// Deserialize 反序列化自身
	Deserialize(data []byte) error
}

// Error definitions
var (
	ErrNilValue           = errors.New("value is nil")
	ErrInvalidData        = errors.New("invalid data")
	ErrUnsupportedType    = errors.New("unsupported type")
	ErrTypeNotRegistered  = errors.New("type not registered")
	ErrSerializerNotFound = errors.New("serializer not found")
)

// SerializerFactory 序列化器工厂函数
type SerializerFactory func(options Options) Serializer

// DefaultOptions 默认选项
var DefaultOptions = Options{
	Indent:                "",
	EscapeHTML:            true,
	UseNumber:             false,
	DisallowUnknownFields: false,
	Compression:           CompressionNone,
	Encryption:            EncryptionNone,
	TagName:               "json",
}

// WithIndent 设置缩进选项
func WithIndent(indent string) func(*Options) {
	return func(o *Options) {
		o.Indent = indent
	}
}

// WithCompression 设置压缩选项
func WithCompression(compression Compression) func(*Options) {
	return func(o *Options) {
		o.Compression = compression
	}
}

// WithEncryption 设置加密选项
func WithEncryption(encryption Encryption) func(*Options) {
	return func(o *Options) {
		o.Encryption = encryption
	}
}

// WithTagName 设置标签名选项
func WithTagName(tagName string) func(*Options) {
	return func(o *Options) {
		o.TagName = tagName
	}
}

// ApplyOptions 应用选项
func ApplyOptions(options *Options, opts ...func(*Options)) {
	for _, opt := range opts {
		opt(options)
	}
}
