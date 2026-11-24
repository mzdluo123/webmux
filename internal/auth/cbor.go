package auth

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

const (
	cborMaxDepth = 16
	cborMaxItems = 256
)

// cborMap preserves decoded CBOR map entries (string or int64 keys) without
// relying on third-party packages.
type cborMap []cborEntry

type cborEntry struct {
	key any
	val any
}

func (m cborMap) stringKey(name string) (any, bool) {
	for _, entry := range m {
		if k, ok := entry.key.(string); ok && k == name {
			return entry.val, true
		}
	}
	return nil, false
}

func (m cborMap) intKey(id int64) (any, bool) {
	for _, entry := range m {
		if k, ok := entry.key.(int64); ok && k == id {
			return entry.val, true
		}
	}
	return nil, false
}

type cborReader struct {
	data []byte
	pos  int
}

// decodeCBOR decodes a single bounded RFC 8949 CBOR item and returns it along
// with the number of bytes consumed.
func decodeCBOR(data []byte) (any, int, error) {
	r := &cborReader{data: data}
	val, err := r.readValue(0)
	if err != nil {
		return nil, 0, err
	}
	return val, r.pos, nil
}

func (r *cborReader) readValue(depth int) (any, error) {
	if depth > cborMaxDepth {
		return nil, errors.New("cbor: nesting depth exceeded")
	}
	if r.pos >= len(r.data) {
		return nil, errors.New("cbor: unexpected end of input")
	}
	initial := r.data[r.pos]
	r.pos++
	major := initial >> 5
	additional := initial & 0x1f

	switch major {
	case 0: // unsigned integer
		u, err := r.readUint(additional)
		if err != nil {
			return nil, err
		}
		if u > math.MaxInt64 {
			return u, nil
		}
		return int64(u), nil
	case 1: // negative integer (-1 - n)
		u, err := r.readUint(additional)
		if err != nil {
			return nil, err
		}
		if u > math.MaxInt64 {
			return nil, errors.New("cbor: negative integer underflow")
		}
		return -1 - int64(u), nil
	case 2: // byte string
		if additional == 31 {
			return nil, errors.New("cbor: indefinite byte strings are not supported")
		}
		n, err := r.readLength(additional)
		if err != nil {
			return nil, err
		}
		out := make([]byte, n)
		copy(out, r.data[r.pos:r.pos+n])
		r.pos += n
		return out, nil
	case 3: // text string
		if additional == 31 {
			return nil, errors.New("cbor: indefinite text strings are not supported")
		}
		n, err := r.readLength(additional)
		if err != nil {
			return nil, err
		}
		s := string(r.data[r.pos : r.pos+n])
		r.pos += n
		return s, nil
	case 4: // array
		if additional == 31 {
			var items []any
			for {
				if r.pos >= len(r.data) {
					return nil, errors.New("cbor: unterminated indefinite array")
				}
				if r.data[r.pos] == 0xff {
					r.pos++
					return items, nil
				}
				if len(items) >= cborMaxItems {
					return nil, errors.New("cbor: array item limit exceeded")
				}
				v, err := r.readValue(depth + 1)
				if err != nil {
					return nil, err
				}
				items = append(items, v)
			}
		}
		n, err := r.readCount(additional)
		if err != nil {
			return nil, err
		}
		items := make([]any, n)
		for i := range n {
			v, err := r.readValue(depth + 1)
			if err != nil {
				return nil, err
			}
			items[i] = v
		}
		return items, nil
	case 5: // map
		if additional == 31 {
			var entries cborMap
			for {
				if r.pos >= len(r.data) {
					return nil, errors.New("cbor: unterminated indefinite map")
				}
				if r.data[r.pos] == 0xff {
					r.pos++
					return entries, nil
				}
				if len(entries) >= cborMaxItems {
					return nil, errors.New("cbor: map item limit exceeded")
				}
				k, err := r.readValue(depth + 1)
				if err != nil {
					return nil, err
				}
				v, err := r.readValue(depth + 1)
				if err != nil {
					return nil, err
				}
				entries = append(entries, cborEntry{key: k, val: v})
			}
		}
		n, err := r.readCount(additional)
		if err != nil {
			return nil, err
		}
		entries := make(cborMap, n)
		for i := range n {
			k, err := r.readValue(depth + 1)
			if err != nil {
				return nil, err
			}
			v, err := r.readValue(depth + 1)
			if err != nil {
				return nil, err
			}
			entries[i] = cborEntry{key: k, val: v}
		}
		return entries, nil
	case 6: // tag
		if _, err := r.readUint(additional); err != nil {
			return nil, err
		}
		return r.readValue(depth + 1)
	case 7: // simple value or float
		switch additional {
		case 20:
			return false, nil
		case 21:
			return true, nil
		case 22, 23:
			return nil, nil
		case 24:
			if r.pos >= len(r.data) {
				return nil, errors.New("cbor: truncated simple value")
			}
			b := r.data[r.pos]
			r.pos++
			return b, nil
		case 25:
			if r.pos+2 > len(r.data) {
				return nil, errors.New("cbor: truncated float16")
			}
			r.pos += 2
			return nil, nil
		case 26:
			if r.pos+4 > len(r.data) {
				return nil, errors.New("cbor: truncated float32")
			}
			r.pos += 4
			return nil, nil
		case 27:
			if r.pos+8 > len(r.data) {
				return nil, errors.New("cbor: truncated float64")
			}
			r.pos += 8
			return nil, nil
		default:
			if additional < 20 {
				return additional, nil
			}
			return nil, fmt.Errorf("cbor: unsupported simple value %d", additional)
		}
	default:
		return nil, fmt.Errorf("cbor: invalid major type %d", major)
	}
}

func (r *cborReader) readUint(additional byte) (uint64, error) {
	switch {
	case additional < 24:
		return uint64(additional), nil
	case additional == 24:
		if r.pos+1 > len(r.data) {
			return 0, errors.New("cbor: truncated uint8")
		}
		v := uint64(r.data[r.pos])
		r.pos++
		return v, nil
	case additional == 25:
		if r.pos+2 > len(r.data) {
			return 0, errors.New("cbor: truncated uint16")
		}
		v := uint64(binary.BigEndian.Uint16(r.data[r.pos : r.pos+2]))
		r.pos += 2
		return v, nil
	case additional == 26:
		if r.pos+4 > len(r.data) {
			return 0, errors.New("cbor: truncated uint32")
		}
		v := uint64(binary.BigEndian.Uint32(r.data[r.pos : r.pos+4]))
		r.pos += 4
		return v, nil
	case additional == 27:
		if r.pos+8 > len(r.data) {
			return 0, errors.New("cbor: truncated uint64")
		}
		v := binary.BigEndian.Uint64(r.data[r.pos : r.pos+8])
		r.pos += 8
		return v, nil
	default:
		return 0, fmt.Errorf("cbor: invalid additional info %d", additional)
	}
}

func (r *cborReader) readLength(additional byte) (int, error) {
	u, err := r.readUint(additional)
	if err != nil {
		return 0, err
	}
	remaining := uint64(len(r.data) - r.pos)
	if u > remaining {
		return 0, errors.New("cbor: length exceeds remaining input")
	}
	return int(u), nil
}

func (r *cborReader) readCount(additional byte) (int, error) {
	u, err := r.readUint(additional)
	if err != nil {
		return 0, err
	}
	if u > cborMaxItems {
		return 0, errors.New("cbor: container item limit exceeded")
	}
	return int(u), nil
}
