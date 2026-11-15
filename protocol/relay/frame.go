package relay

import (
	"encoding/binary"
	"encoding/json"
	"io"
)

// ReadFrame reads one length-prefixed JSON protocol frame without buffering any
// following HTTP/body bytes. max MUST be the limit for that frame kind. The
// transport sets deadlines/cancellation before calling; this codec has no clock.
func ReadFrame(r io.Reader, dst any, max int) error {
	var prefix [4]byte
	if _, err := io.ReadFull(r, prefix[:]); err != nil {
		return err
	}
	n := binary.BigEndian.Uint32(prefix[:])
	if max <= 0 || n == 0 || uint64(n) > uint64(max) {
		return Error{Code: Invalid}
	}
	data := make([]byte, int(n))
	if _, err := io.ReadFull(r, data); err != nil {
		return err
	}
	return Decode(data, dst, max)
}

func WriteFrame(w io.Writer, src any, max int) error {
	data, err := json.Marshal(src)
	if err != nil {
		return Error{Code: Invalid}
	}
	if max <= 0 || len(data) == 0 || len(data) > max || uint64(len(data)) > uint64(^uint32(0)) {
		return Error{Code: Invalid}
	}
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], uint32(len(data)))
	if err := writeAll(w, prefix[:]); err != nil {
		return err
	}
	return writeAll(w, data)
}

func writeAll(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if n < 0 || n > len(data) {
			return io.ErrShortWrite
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}
