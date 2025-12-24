package serialization

import (
	"bytes"
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"
)

// ProtobufSerializer Protobuf序列化器
type ProtobufSerializer struct {
	options Options
}

// NewProtobufSerializer 创建新的Protobuf序列化器
func NewProtobufSerializer(opts ...func(*Options)) *ProtobufSerializer {
	options := DefaultOptions
	ApplyOptions(&options, opts...)
	return &ProtobufSerializer{options: options}
}

// Serialize 序列化对象为Protobuf
func (s *ProtobufSerializer) Serialize(v interface{}) ([]byte, error) {
	if v == nil {
		return nil, ErrNilValue
	}

	// 检查是否实现了proto.Message接口
	msg, ok := v.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("value does not implement proto.Message: %T", v)
	}

	// 使用proto.Marshal进行序列化
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("proto.Marshal failed: %w", err)
	}

	return data, nil
}

// Deserialize 从Protobuf反序列化对象
func (s *ProtobufSerializer) Deserialize(data []byte, v interface{}) error {
	if len(data) == 0 {
		return ErrInvalidData
	}

	if v == nil {
		return ErrNilValue
	}

	// 检查是否实现了proto.Message接口
	msg, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("value does not implement proto.Message: %T", v)
	}

	// 使用proto.Unmarshal进行反序列化
	if err := proto.Unmarshal(data, msg); err != nil {
		return fmt.Errorf("proto.Unmarshal failed: %w", err)
	}

	return nil
}

// ContentType 返回内容类型
func (s *ProtobufSerializer) ContentType() string {
	return "application/protobuf"
}

// Format 返回格式名称
func (s *ProtobufSerializer) Format() Format {
	return FormatProtobuf
}

// ProtobufEncoder Protobuf编码器
type ProtobufEncoder struct {
	buffer *bytes.Buffer
}

// NewProtobufEncoder 创建新的Protobuf编码器
func NewProtobufEncoder() *ProtobufEncoder {
	return &ProtobufEncoder{
		buffer: &bytes.Buffer{},
	}
}

// Encode 编码对象
func (e *ProtobufEncoder) Encode(v interface{}) ([]byte, error) {
	if v == nil {
		return nil, ErrNilValue
	}

	msg, ok := v.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("value does not implement proto.Message: %T", v)
	}

	return proto.Marshal(msg)
}

// Bytes 获取编码后的字节
func (e *ProtobufEncoder) Bytes() []byte {
	return e.buffer.Bytes()
}

// ProtobufDecoder Protobuf解码器
type ProtobufDecoder struct {
	data []byte
}

// NewProtobufDecoder 创建新的Protobuf解码器
func NewProtobufDecoder(data []byte) *ProtobufDecoder {
	return &ProtobufDecoder{data: data}
}

// Decode 解码到对象
func (d *ProtobufDecoder) Decode(v interface{}) error {
	if v == nil {
		return ErrNilValue
	}

	msg, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("value does not implement proto.Message: %T", v)
	}

	return proto.Unmarshal(d.data, msg)
}

// Reset 重置解码器
func (d *ProtobufDecoder) Reset(data []byte) {
	d.data = data
}

// ProtobufStreamEncoder Protobuf流编码器
type ProtobufStreamEncoder struct {
	writer io.Writer
}

// NewProtobufStreamEncoder 创建新的Protobuf流编码器
func NewProtobufStreamEncoder(writer io.Writer) *ProtobufStreamEncoder {
	return &ProtobufStreamEncoder{writer: writer}
}

// Write 写入对象到流
func (e *ProtobufStreamEncoder) Write(v interface{}) error {
	if v == nil {
		return ErrNilValue
	}

	msg, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("value does not implement proto.Message: %T", v)
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	// 先写入消息长度
	length := uint32(len(data))
	if err := writeUint32(e.writer, length); err != nil {
		return err
	}

	// 再写入消息内容
	_, err = e.writer.Write(data)
	return err
}

// ProtobufStreamDecoder Protobuf流解码器
type ProtobufStreamDecoder struct {
	reader io.Reader
}

// NewProtobufStreamDecoder 创建新的Protobuf流解码器
func NewProtobufStreamDecoder(reader io.Reader) *ProtobufStreamDecoder {
	return &ProtobufStreamDecoder{reader: reader}
}

// Read 从流读取对象
func (d *ProtobufStreamDecoder) Read(v interface{}) error {
	if v == nil {
		return ErrNilValue
	}

	msg, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("value does not implement proto.Message: %T", v)
	}

	// 先读取消息长度
	length, err := readUint32(d.reader)
	if err != nil {
		return err
	}

	// 读取消息内容
	data := make([]byte, length)
	if _, err := io.ReadFull(d.reader, data); err != nil {
		return err
	}

	return proto.Unmarshal(data, msg)
}

// 辅助函数

// writeUint32 写入uint32到writer
func writeUint32(w io.Writer, v uint32) error {
	var buf [4]byte
	buf[0] = byte(v >> 24)
	buf[1] = byte(v >> 16)
	buf[2] = byte(v >> 8)
	buf[3] = byte(v)
	_, err := w.Write(buf[:])
	return err
}

// readUint32 从reader读取uint32
func readUint32(r io.Reader) (uint32, error) {
	var buf [4]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3]), nil
}

// ProtobufCodec Protobuf编解码器（简化版，不依赖类型注册）
type ProtobufCodec struct{}

// NewProtobufCodec 创建新的Protobuf编解码器
func NewProtobufCodec() *ProtobufCodec {
	return &ProtobufCodec{}
}

// Encode 编码
func (c *ProtobufCodec) Encode(v interface{}) ([]byte, error) {
	if v == nil {
		return nil, ErrNilValue
	}

	msg, ok := v.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("value does not implement proto.Message: %T", v)
	}

	return proto.Marshal(msg)
}

// Decode 解码
func (c *ProtobufCodec) Decode(data []byte, v interface{}) error {
	if len(data) == 0 {
		return ErrInvalidData
	}

	if v == nil {
		return ErrNilValue
	}

	msg, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("value does not implement proto.Message: %T", v)
	}

	return proto.Unmarshal(data, msg)
}

// MarshalProtobuf 通用Protobuf序列化函数
func MarshalProtobuf(v interface{}) ([]byte, error) {
	if v == nil {
		return nil, ErrNilValue
	}

	msg, ok := v.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("value does not implement proto.Message: %T", v)
	}

	return proto.Marshal(msg)
}

// UnmarshalProtobuf 通用Protobuf反序列化函数
func UnmarshalProtobuf(data []byte, v interface{}) error {
	if len(data) == 0 {
		return ErrInvalidData
	}

	if v == nil {
		return ErrNilValue
	}

	msg, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("value does not implement proto.Message: %T", v)
	}

	return proto.Unmarshal(data, msg)
}
