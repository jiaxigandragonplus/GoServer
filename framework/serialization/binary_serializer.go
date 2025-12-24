package serialization

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"reflect"
	"time"
)

// BinarySerializer 二进制序列化器
type BinarySerializer struct {
	options Options
}

// NewBinarySerializer 创建新的二进制序列化器
func NewBinarySerializer(opts ...func(*Options)) *BinarySerializer {
	options := DefaultOptions
	ApplyOptions(&options, opts...)
	return &BinarySerializer{options: options}
}

// Serialize 序列化对象为二进制
func (s *BinarySerializer) Serialize(v interface{}) ([]byte, error) {
	if v == nil {
		return nil, ErrNilValue
	}

	// 检查是否实现了Serializable接口
	if serializable, ok := v.(Serializable); ok {
		return serializable.Serialize()
	}

	// 使用Gob编码作为默认二进制格式
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)

	if err := encoder.Encode(v); err != nil {
		return nil, fmt.Errorf("gob encode failed: %w", err)
	}

	data := buf.Bytes()

	// 应用压缩
	if s.options.Compression != CompressionNone {
		compressed, err := compress(data, s.options.Compression)
		if err != nil {
			return nil, fmt.Errorf("compression failed: %w", err)
		}
		data = compressed
	}

	// 应用加密
	if s.options.Encryption != EncryptionNone {
		encrypted, err := encrypt(data, s.options.Encryption)
		if err != nil {
			return nil, fmt.Errorf("encryption failed: %w", err)
		}
		data = encrypted
	}

	return data, nil
}

// Deserialize 从二进制反序列化对象
func (s *BinarySerializer) Deserialize(data []byte, v interface{}) error {
	if len(data) == 0 {
		return ErrInvalidData
	}

	if v == nil {
		return ErrNilValue
	}

	// 应用解密
	if s.options.Encryption != EncryptionNone {
		decrypted, err := decrypt(data, s.options.Encryption)
		if err != nil {
			return fmt.Errorf("decryption failed: %w", err)
		}
		data = decrypted
	}

	// 应用解压
	if s.options.Compression != CompressionNone {
		decompressed, err := decompress(data, s.options.Compression)
		if err != nil {
			return fmt.Errorf("decompression failed: %w", err)
		}
		data = decompressed
	}

	// 检查是否实现了Serializable接口
	if serializable, ok := v.(Serializable); ok {
		return serializable.Deserialize(data)
	}

	// 使用Gob解码
	decoder := gob.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("gob decode failed: %w", err)
	}

	return nil
}

// ContentType 返回内容类型
func (s *BinarySerializer) ContentType() string {
	return "application/octet-stream"
}

// Format 返回格式名称
func (s *BinarySerializer) Format() Format {
	return FormatBinary
}

// BinaryEncoder 二进制编码器
type BinaryEncoder struct {
	buffer  *bytes.Buffer
	encoder *gob.Encoder
	options Options
}

// NewBinaryEncoder 创建新的二进制编码器
func NewBinaryEncoder(opts ...func(*Options)) *BinaryEncoder {
	options := DefaultOptions
	ApplyOptions(&options, opts...)

	buffer := &bytes.Buffer{}
	encoder := gob.NewEncoder(buffer)

	return &BinaryEncoder{
		buffer:  buffer,
		encoder: encoder,
		options: options,
	}
}

// Encode 编码对象
func (e *BinaryEncoder) Encode(v interface{}) error {
	if v == nil {
		return ErrNilValue
	}

	return e.encoder.Encode(v)
}

// Bytes 获取编码后的字节
func (e *BinaryEncoder) Bytes() []byte {
	return e.buffer.Bytes()
}

// BinaryDecoder 二进制解码器
type BinaryDecoder struct {
	decoder *gob.Decoder
	options Options
}

// NewBinaryDecoder 创建新的二进制解码器
func NewBinaryDecoder(data []byte, opts ...func(*Options)) *BinaryDecoder {
	options := DefaultOptions
	ApplyOptions(&options, opts...)

	decoder := gob.NewDecoder(bytes.NewReader(data))

	return &BinaryDecoder{
		decoder: decoder,
		options: options,
	}
}

// Decode 解码到对象
func (d *BinaryDecoder) Decode(v interface{}) error {
	if v == nil {
		return ErrNilValue
	}

	return d.decoder.Decode(v)
}

// Reset 重置解码器
func (d *BinaryDecoder) Reset(data []byte) {
	d.decoder = gob.NewDecoder(bytes.NewReader(data))
}

// BinaryWriter 二进制写入器
type BinaryWriter struct {
	buffer *bytes.Buffer
	order  binary.ByteOrder
}

// NewBinaryWriter 创建新的二进制写入器
func NewBinaryWriter(order binary.ByteOrder) *BinaryWriter {
	return &BinaryWriter{
		buffer: &bytes.Buffer{},
		order:  order,
	}
}

// WriteUint8 写入uint8
func (w *BinaryWriter) WriteUint8(v uint8) error {
	return binary.Write(w.buffer, w.order, v)
}

// WriteUint16 写入uint16
func (w *BinaryWriter) WriteUint16(v uint16) error {
	return binary.Write(w.buffer, w.order, v)
}

// WriteUint32 写入uint32
func (w *BinaryWriter) WriteUint32(v uint32) error {
	return binary.Write(w.buffer, w.order, v)
}

// WriteUint64 写入uint64
func (w *BinaryWriter) WriteUint64(v uint64) error {
	return binary.Write(w.buffer, w.order, v)
}

// WriteInt8 写入int8
func (w *BinaryWriter) WriteInt8(v int8) error {
	return binary.Write(w.buffer, w.order, v)
}

// WriteInt16 写入int16
func (w *BinaryWriter) WriteInt16(v int16) error {
	return binary.Write(w.buffer, w.order, v)
}

// WriteInt32 写入int32
func (w *BinaryWriter) WriteInt32(v int32) error {
	return binary.Write(w.buffer, w.order, v)
}

// WriteInt64 写入int64
func (w *BinaryWriter) WriteInt64(v int64) error {
	return binary.Write(w.buffer, w.order, v)
}

// WriteFloat32 写入float32
func (w *BinaryWriter) WriteFloat32(v float32) error {
	return binary.Write(w.buffer, w.order, v)
}

// WriteFloat64 写入float64
func (w *BinaryWriter) WriteFloat64(v float64) error {
	return binary.Write(w.buffer, w.order, v)
}

// WriteBool 写入bool
func (w *BinaryWriter) WriteBool(v bool) error {
	var b uint8
	if v {
		b = 1
	}
	return binary.Write(w.buffer, w.order, b)
}

// WriteString 写入字符串
func (w *BinaryWriter) WriteString(v string) error {
	// 先写入长度
	if err := w.WriteUint32(uint32(len(v))); err != nil {
		return err
	}
	// 再写入字符串内容
	_, err := w.buffer.WriteString(v)
	return err
}

// WriteBytes 写入字节切片
func (w *BinaryWriter) WriteBytes(v []byte) error {
	// 先写入长度
	if err := w.WriteUint32(uint32(len(v))); err != nil {
		return err
	}
	// 再写入字节内容
	_, err := w.buffer.Write(v)
	return err
}

// WriteTime 写入时间
func (w *BinaryWriter) WriteTime(v time.Time) error {
	// 将时间转换为Unix纳秒
	return w.WriteInt64(v.UnixNano())
}

// Bytes 获取写入的字节
func (w *BinaryWriter) Bytes() []byte {
	return w.buffer.Bytes()
}

// BinaryReader 二进制读取器
type BinaryReader struct {
	buffer *bytes.Reader
	order  binary.ByteOrder
}

// NewBinaryReader 创建新的二进制读取器
func NewBinaryReader(data []byte, order binary.ByteOrder) *BinaryReader {
	return &BinaryReader{
		buffer: bytes.NewReader(data),
		order:  order,
	}
}

// ReadUint8 读取uint8
func (r *BinaryReader) ReadUint8() (uint8, error) {
	var v uint8
	err := binary.Read(r.buffer, r.order, &v)
	return v, err
}

// ReadUint16 读取uint16
func (r *BinaryReader) ReadUint16() (uint16, error) {
	var v uint16
	err := binary.Read(r.buffer, r.order, &v)
	return v, err
}

// ReadUint32 读取uint32
func (r *BinaryReader) ReadUint32() (uint32, error) {
	var v uint32
	err := binary.Read(r.buffer, r.order, &v)
	return v, err
}

// ReadUint64 读取uint64
func (r *BinaryReader) ReadUint64() (uint64, error) {
	var v uint64
	err := binary.Read(r.buffer, r.order, &v)
	return v, err
}

// ReadInt8 读取int8
func (r *BinaryReader) ReadInt8() (int8, error) {
	var v int8
	err := binary.Read(r.buffer, r.order, &v)
	return v, err
}

// ReadInt16 读取int16
func (r *BinaryReader) ReadInt16() (int16, error) {
	var v int16
	err := binary.Read(r.buffer, r.order, &v)
	return v, err
}

// ReadInt32 读取int32
func (r *BinaryReader) ReadInt32() (int32, error) {
	var v int32
	err := binary.Read(r.buffer, r.order, &v)
	return v, err
}

// ReadInt64 读取int64
func (r *BinaryReader) ReadInt64() (int64, error) {
	var v int64
	err := binary.Read(r.buffer, r.order, &v)
	return v, err
}

// ReadFloat32 读取float32
func (r *BinaryReader) ReadFloat32() (float32, error) {
	var v float32
	err := binary.Read(r.buffer, r.order, &v)
	return v, err
}

// ReadFloat64 读取float64
func (r *BinaryReader) ReadFloat64() (float64, error) {
	var v float64
	err := binary.Read(r.buffer, r.order, &v)
	return v, err
}

// ReadBool 读取bool
func (r *BinaryReader) ReadBool() (bool, error) {
	var b uint8
	if err := binary.Read(r.buffer, r.order, &b); err != nil {
		return false, err
	}
	return b != 0, nil
}

// ReadString 读取字符串
func (r *BinaryReader) ReadString() (string, error) {
	// 先读取长度
	length, err := r.ReadUint32()
	if err != nil {
		return "", err
	}

	// 读取字符串内容
	buf := make([]byte, length)
	if _, err := io.ReadFull(r.buffer, buf); err != nil {
		return "", err
	}

	return string(buf), nil
}

// ReadBytes 读取字节切片
func (r *BinaryReader) ReadBytes() ([]byte, error) {
	// 先读取长度
	length, err := r.ReadUint32()
	if err != nil {
		return nil, err
	}

	// 读取字节内容
	buf := make([]byte, length)
	if _, err := io.ReadFull(r.buffer, buf); err != nil {
		return nil, err
	}

	return buf, nil
}

// ReadTime 读取时间
func (r *BinaryReader) ReadTime() (time.Time, error) {
	nanos, err := r.ReadInt64()
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(0, nanos), nil
}

// Size 返回剩余字节数
func (r *BinaryReader) Size() int64 {
	return r.buffer.Size()
}

// Position 返回当前位置
func (r *BinaryReader) Position() int64 {
	pos, _ := r.buffer.Seek(0, io.SeekCurrent)
	return pos
}

// Seek 移动位置
func (r *BinaryReader) Seek(offset int64, whence int) (int64, error) {
	return r.buffer.Seek(offset, whence)
}

// compress 压缩数据
func compress(data []byte, compression Compression) ([]byte, error) {
	switch compression {
	case CompressionGzip:
		return compressGzip(data)
	case CompressionZlib:
		return compressZlib(data)
	case CompressionSnappy:
		return compressSnappy(data)
	default:
		return data, nil
	}
}

// decompress 解压数据
func decompress(data []byte, compression Compression) ([]byte, error) {
	switch compression {
	case CompressionGzip:
		return decompressGzip(data)
	case CompressionZlib:
		return decompressZlib(data)
	case CompressionSnappy:
		return decompressSnappy(data)
	default:
		return data, nil
	}
}

// encrypt 加密数据
func encrypt(data []byte, encryption Encryption) ([]byte, error) {
	switch encryption {
	case EncryptionAES:
		return encryptAES(data)
	case EncryptionRSA:
		return encryptRSA(data)
	default:
		return data, nil
	}
}

// decrypt 解密数据
func decrypt(data []byte, encryption Encryption) ([]byte, error) {
	switch encryption {
	case EncryptionAES:
		return decryptAES(data)
	case EncryptionRSA:
		return decryptRSA(data)
	default:
		return data, nil
	}
}

// 压缩/解压和加密/解密的实现占位符
func compressGzip(data []byte) ([]byte, error) {
	// 实际实现应使用compress/gzip
	return data, nil
}

func decompressGzip(data []byte) ([]byte, error) {
	// 实际实现应使用compress/gzip
	return data, nil
}

func compressZlib(data []byte) ([]byte, error) {
	// 实际实现应使用compress/zlib
	return data, nil
}

func decompressZlib(data []byte) ([]byte, error) {
	// 实际实现应使用compress/zlib
	return data, nil
}

func compressSnappy(data []byte) ([]byte, error) {
	// 实际实现应使用github.com/golang/snappy
	return data, nil
}

func decompressSnappy(data []byte) ([]byte, error) {
	// 实际实现应使用github.com/golang/snappy
	return data, nil
}

func encryptAES(data []byte) ([]byte, error) {
	// 实际实现应使用crypto/aes
	return data, nil
}

func decryptAES(data []byte) ([]byte, error) {
	// 实际实现应使用crypto/aes
	return data, nil
}

func encryptRSA(data []byte) ([]byte, error) {
	// 实际实现应使用crypto/rsa
	return data, nil
}

func decryptRSA(data []byte) ([]byte, error) {
	// 实际实现应使用crypto/rsa
	return data, nil
}

// BinarySize 计算二进制大小
func BinarySize(v interface{}) (int, error) {
	if v == nil {
		return 0, nil
	}

	// 使用反射计算大致大小
	val := reflect.ValueOf(v)
	return binarySizeValue(val), nil
}

func binarySizeValue(val reflect.Value) int {
	if !val.IsValid() {
		return 0
	}

	typ := val.Type()

	switch typ.Kind() {
	case reflect.Bool:
		return 1
	case reflect.Int8, reflect.Uint8:
		return 1
	case reflect.Int16, reflect.Uint16:
		return 2
	case reflect.Int32, reflect.Uint32, reflect.Float32:
		return 4
	case reflect.Int64, reflect.Uint64, reflect.Float64, reflect.Complex64:
		return 8
	case reflect.Complex128:
		return 16
	case reflect.String:
		return 4 + val.Len() // 长度 + 字符串内容
	case reflect.Slice, reflect.Array:
		if typ.Elem().Kind() == reflect.Uint8 {
			return 4 + val.Len() // 长度 + 字节内容
		}
		// 其他类型数组
		size := 4 // 长度
		for i := 0; i < val.Len(); i++ {
			size += binarySizeValue(val.Index(i))
		}
		return size
	case reflect.Map:
		size := 4 // 长度
		iter := val.MapRange()
		for iter.Next() {
			size += binarySizeValue(iter.Key())
			size += binarySizeValue(iter.Value())
		}
		return size
	case reflect.Struct:
		size := 0
		for i := 0; i < val.NumField(); i++ {
			size += binarySizeValue(val.Field(i))
		}
		return size
	case reflect.Ptr:
		if val.IsNil() {
			return 1 // 空指针标记
		}
		return 1 + binarySizeValue(val.Elem()) // 非空标记 + 内容
	case reflect.Interface:
		if val.IsNil() {
			return 1 // 空接口标记
		}
		return 1 + binarySizeValue(val.Elem()) // 非空标记 + 内容
	default:
		return 0
	}
}
