package serialization

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
)

// JSONSerializer JSON序列化器
type JSONSerializer struct {
	options Options
}

// NewJSONSerializer 创建新的JSON序列化器
func NewJSONSerializer(opts ...func(*Options)) *JSONSerializer {
	options := DefaultOptions
	ApplyOptions(&options, opts...)
	return &JSONSerializer{options: options}
}

// Serialize 序列化对象为JSON
func (s *JSONSerializer) Serialize(v interface{}) ([]byte, error) {
	if v == nil {
		return nil, ErrNilValue
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)

	// 配置编码器
	encoder.SetEscapeHTML(s.options.EscapeHTML)

	if s.options.Indent != "" {
		// 对于缩进，使用json.MarshalIndent
		data, err := json.MarshalIndent(v, "", s.options.Indent)
		if err != nil {
			return nil, fmt.Errorf("JSON marshal indent failed: %w", err)
		}
		return data, nil
	}

	if err := encoder.Encode(v); err != nil {
		return nil, fmt.Errorf("JSON encode failed: %w", err)
	}

	// 移除末尾的换行符
	data := buf.Bytes()
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}

	return data, nil
}

// Deserialize 从JSON反序列化对象
func (s *JSONSerializer) Deserialize(data []byte, v interface{}) error {
	if len(data) == 0 {
		return ErrInvalidData
	}

	if v == nil {
		return ErrNilValue
	}

	decoder := json.NewDecoder(bytes.NewReader(data))

	// 配置解码器
	if s.options.UseNumber {
		decoder.UseNumber()
	}

	if s.options.DisallowUnknownFields {
		decoder.DisallowUnknownFields()
	}

	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("JSON decode failed: %w", err)
	}

	return nil
}

// ContentType 返回内容类型
func (s *JSONSerializer) ContentType() string {
	return "application/json"
}

// Format 返回格式名称
func (s *JSONSerializer) Format() Format {
	return FormatJSON
}

// JSONEncoder JSON编码器
type JSONEncoder struct {
	buffer  *bytes.Buffer
	encoder *json.Encoder
	options Options
}

// NewJSONEncoder 创建新的JSON编码器
func NewJSONEncoder(opts ...func(*Options)) *JSONEncoder {
	options := DefaultOptions
	ApplyOptions(&options, opts...)

	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(options.EscapeHTML)

	return &JSONEncoder{
		buffer:  buffer,
		encoder: encoder,
		options: options,
	}
}

// Encode 编码对象
func (e *JSONEncoder) Encode(v interface{}) error {
	if v == nil {
		return ErrNilValue
	}

	return e.encoder.Encode(v)
}

// Bytes 获取编码后的字节
func (e *JSONEncoder) Bytes() []byte {
	data := e.buffer.Bytes()
	// 移除末尾的换行符
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}
	return data
}

// JSONDecoder JSON解码器
type JSONDecoder struct {
	decoder *json.Decoder
	options Options
}

// NewJSONDecoder 创建新的JSON解码器
func NewJSONDecoder(data []byte, opts ...func(*Options)) *JSONDecoder {
	options := DefaultOptions
	ApplyOptions(&options, opts...)

	decoder := json.NewDecoder(bytes.NewReader(data))

	if options.UseNumber {
		decoder.UseNumber()
	}

	if options.DisallowUnknownFields {
		decoder.DisallowUnknownFields()
	}

	return &JSONDecoder{
		decoder: decoder,
		options: options,
	}
}

// Decode 解码到对象
func (d *JSONDecoder) Decode(v interface{}) error {
	if v == nil {
		return ErrNilValue
	}

	return d.decoder.Decode(v)
}

// Reset 重置解码器
func (d *JSONDecoder) Reset(data []byte) {
	d.decoder = json.NewDecoder(bytes.NewReader(data))

	if d.options.UseNumber {
		d.decoder.UseNumber()
	}

	if d.options.DisallowUnknownFields {
		d.decoder.DisallowUnknownFields()
	}
}

// MarshalJSON 通用JSON序列化函数
func MarshalJSON(v interface{}, opts ...func(*Options)) ([]byte, error) {
	serializer := NewJSONSerializer(opts...)
	return serializer.Serialize(v)
}

// UnmarshalJSON 通用JSON反序列化函数
func UnmarshalJSON(data []byte, v interface{}, opts ...func(*Options)) error {
	serializer := NewJSONSerializer(opts...)
	return serializer.Deserialize(data, v)
}

// PrettyJSON 格式化JSON为可读格式
func PrettyJSON(v interface{}) ([]byte, error) {
	return MarshalJSON(v, WithIndent("  "))
}

// CompactJSON 压缩JSON（移除空白字符）
func CompactJSON(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// IsValidJSON 检查是否为有效的JSON
func IsValidJSON(data []byte) bool {
	var js json.RawMessage
	return json.Unmarshal(data, &js) == nil
}

// JSONStreamEncoder JSON流编码器
type JSONStreamEncoder struct {
	writer  io.Writer
	encoder *json.Encoder
	first   bool
}

// NewJSONStreamEncoder 创建新的JSON流编码器
func NewJSONStreamEncoder(writer io.Writer) *JSONStreamEncoder {
	encoder := json.NewEncoder(writer)
	return &JSONStreamEncoder{
		writer:  writer,
		encoder: encoder,
		first:   true,
	}
}

// Write 写入对象到流
func (e *JSONStreamEncoder) Write(v interface{}) error {
	if e.first {
		e.first = false
	} else {
		if _, err := e.writer.Write([]byte("\n")); err != nil {
			return err
		}
	}

	return e.encoder.Encode(v)
}

// JSONStreamDecoder JSON流解码器
type JSONStreamDecoder struct {
	decoder *json.Decoder
}

// NewJSONStreamDecoder 创建新的JSON流解码器
func NewJSONStreamDecoder(reader io.Reader) *JSONStreamDecoder {
	decoder := json.NewDecoder(reader)
	return &JSONStreamDecoder{decoder: decoder}
}

// Next 读取下一个对象
func (d *JSONStreamDecoder) Next(v interface{}) error {
	return d.decoder.Decode(v)
}

// More 检查是否还有更多数据
func (d *JSONStreamDecoder) More() bool {
	return d.decoder.More()
}

// JSONTypeInfo JSON类型信息
type JSONTypeInfo struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

// MarshalWithType 序列化带类型信息
func MarshalWithType(v interface{}, typeName string) ([]byte, error) {
	info := JSONTypeInfo{
		Type:  typeName,
		Value: v,
	}
	return json.Marshal(info)
}

// UnmarshalWithType 反序列化带类型信息
func UnmarshalWithType(data []byte, registry *TypeRegistry) (interface{}, error) {
	var info JSONTypeInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	typ, err := registry.Get(info.Type)
	if err != nil {
		return nil, err
	}

	value := reflect.New(typ).Interface()

	// 反序列化值
	valueData, err := json.Marshal(info.Value)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(valueData, value); err != nil {
		return nil, err
	}

	return reflect.ValueOf(value).Elem().Interface(), nil
}
