package models

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

const ggufMagic = "GGUF"

// GGUFHeader holds parsed GGUF file header metadata.
type GGUFHeader struct {
	Version     uint32
	TensorCount uint64
	KVCount     uint64
	// Metadata contains string-representable key-value pairs from the header.
	Metadata map[string]string
}

// ReadGGUFHeader opens path and parses the GGUF header + metadata KV pairs.
// Supports GGUF versions 2 and 3.
func ReadGGUFHeader(path string) (*GGUFHeader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Magic
	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return nil, fmt.Errorf("read magic: %w", err)
	}
	if string(magic) != ggufMagic {
		return nil, fmt.Errorf("not a GGUF file (magic=%q)", string(magic))
	}

	var version uint32
	if err := binary.Read(f, binary.LittleEndian, &version); err != nil {
		return nil, fmt.Errorf("read version: %w", err)
	}
	if version < 2 || version > 3 {
		return nil, fmt.Errorf("unsupported GGUF version %d (supported: 2, 3)", version)
	}

	var nTensors, nKV uint64
	if err := binary.Read(f, binary.LittleEndian, &nTensors); err != nil {
		return nil, fmt.Errorf("read n_tensors: %w", err)
	}
	if err := binary.Read(f, binary.LittleEndian, &nKV); err != nil {
		return nil, fmt.Errorf("read n_kv: %w", err)
	}

	meta := make(map[string]string)
	for i := uint64(0); i < nKV; i++ {
		key, err := ggufReadString(f)
		if err != nil {
			break
		}
		val, err := ggufReadValue(f)
		if err != nil {
			break
		}
		if val != "" {
			meta[key] = val
		}
	}

	return &GGUFHeader{
		Version:     version,
		TensorCount: nTensors,
		KVCount:     nKV,
		Metadata:    meta,
	}, nil
}

// GGUF metadata value type constants.
const (
	ggufUint8   uint32 = 0
	ggufInt8    uint32 = 1
	ggufUint16  uint32 = 2
	ggufInt16   uint32 = 3
	ggufUint32  uint32 = 4
	ggufInt32   uint32 = 5
	ggufFloat32 uint32 = 6
	ggufBool    uint32 = 7
	ggufString  uint32 = 8
	ggufArray   uint32 = 9
	ggufUint64  uint32 = 10
	ggufInt64   uint32 = 11
	ggufFloat64 uint32 = 12
)

func ggufReadString(r io.Reader) (string, error) {
	var length uint64
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return "", err
	}
	if length > 1<<20 { // 1 MiB sanity cap
		return "", fmt.Errorf("string length %d exceeds sanity limit", length)
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

// ggufReadValue reads a type tag + value, returning a string representation.
// Returns "" (without error) for arrays — caller stores nothing for those.
func ggufReadValue(r io.Reader) (string, error) {
	var vtype uint32
	if err := binary.Read(r, binary.LittleEndian, &vtype); err != nil {
		return "", err
	}
	return ggufReadValueByType(r, vtype)
}

func ggufReadValueByType(r io.Reader, vtype uint32) (string, error) {
	switch vtype {
	case ggufUint8:
		var v uint8
		return fmt.Sprintf("%d", v), binary.Read(r, binary.LittleEndian, &v)
	case ggufInt8:
		var v int8
		return fmt.Sprintf("%d", v), binary.Read(r, binary.LittleEndian, &v)
	case ggufUint16:
		var v uint16
		err := binary.Read(r, binary.LittleEndian, &v)
		return fmt.Sprintf("%d", v), err
	case ggufInt16:
		var v int16
		err := binary.Read(r, binary.LittleEndian, &v)
		return fmt.Sprintf("%d", v), err
	case ggufUint32:
		var v uint32
		err := binary.Read(r, binary.LittleEndian, &v)
		return fmt.Sprintf("%d", v), err
	case ggufInt32:
		var v int32
		err := binary.Read(r, binary.LittleEndian, &v)
		return fmt.Sprintf("%d", v), err
	case ggufFloat32:
		var v float32
		err := binary.Read(r, binary.LittleEndian, &v)
		return fmt.Sprintf("%.4g", v), err
	case ggufBool:
		var v uint8
		err := binary.Read(r, binary.LittleEndian, &v)
		if v != 0 {
			return "true", err
		}
		return "false", err
	case ggufString:
		return ggufReadString(r)
	case ggufArray:
		return "", ggufSkipArray(r)
	case ggufUint64:
		var v uint64
		err := binary.Read(r, binary.LittleEndian, &v)
		return fmt.Sprintf("%d", v), err
	case ggufInt64:
		var v int64
		err := binary.Read(r, binary.LittleEndian, &v)
		return fmt.Sprintf("%d", v), err
	case ggufFloat64:
		var v float64
		err := binary.Read(r, binary.LittleEndian, &v)
		return fmt.Sprintf("%.4g", v), err
	default:
		return "", fmt.Errorf("unknown GGUF value type %d", vtype)
	}
}

func ggufSkipArray(r io.Reader) error {
	var elemType uint32
	if err := binary.Read(r, binary.LittleEndian, &elemType); err != nil {
		return err
	}
	var count uint64
	if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
		return err
	}
	for i := uint64(0); i < count; i++ {
		if _, err := ggufReadValueByType(r, elemType); err != nil {
			return err
		}
	}
	return nil
}
