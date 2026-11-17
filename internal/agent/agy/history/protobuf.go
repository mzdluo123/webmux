package history

import (
	"encoding/binary"
	"strings"
	"unicode/utf8"

	"github.com/rainchan/webmux/internal/historysource"
)

// These field numbers are descriptor evidence from the six-step native fixture,
// not guesses based on display text. Private/config/opaque fields are skipped.
type message struct {
	fields  map[int][][]byte
	ints    map[int]uint64
	unknown bool
}

func decode(data []byte, spec map[int]int) (message, error) {
	m := message{fields: make(map[int][][]byte), ints: make(map[int]uint64)}
	if len(data) > historysource.MaxRecordBytes {
		return m, historysource.ErrLimit
	}
	for count := 0; len(data) > 0; count++ {
		if count >= historysource.MaxRecords {
			return m, historysource.ErrLimit
		}
		key, n := binary.Uvarint(data)
		if n <= 0 || key>>3 == 0 || key>>3 > (1<<29)-1 {
			return m, historysource.ErrFormat
		}
		data = data[n:]
		field, wire := int(key>>3), int(key&7)
		var b []byte
		var v uint64
		switch wire {
		case 0:
			v, n = binary.Uvarint(data)
			if n <= 0 {
				return m, historysource.ErrFormat
			}
			data = data[n:]
		case 1, 5:
			size := 8
			if wire == 5 {
				size = 4
			}
			if len(data) < size {
				return m, historysource.ErrFormat
			}
			data = data[size:]
		case 2:
			size, used := binary.Uvarint(data)
			if used <= 0 || size > uint64(len(data)-used) {
				return m, historysource.ErrFormat
			}
			b = data[used : used+int(size)]
			data = data[used+int(size):]
		default:
			return m, historysource.ErrFormat
		}
		expected, known := spec[field]
		if !known {
			m.unknown = true
			continue
		}
		if expected != wire {
			return m, historysource.ErrFormat
		}
		if wire == 2 {
			m.fields[field] = append(m.fields[field], b)
		} else {
			m.ints[field] = v
		}
	}
	return m, nil
}

func (m message) text(field int) (string, error) {
	values := m.fields[field]
	if len(values) == 0 {
		return "", nil
	}
	if len(values) != 1 || !utf8.Valid(values[0]) {
		return "", historysource.ErrFormat
	}
	return string(values[0]), nil
}

func (m message) child(field int, spec map[int]int) (message, error) {
	values := m.fields[field]
	if len(values) > 1 {
		return message{}, historysource.ErrFormat
	}
	if len(values) == 0 {
		return decode(nil, spec)
	}
	return decode(values[0], spec)
}

var stepSpec = map[int]int{1: 0, 4: 0, 5: 2, 19: 2, 20: 2, 114: 2, 140: 2, 148: 2}
var metadataSpec = map[int]int{1: 2, 3: 0, 4: 2, 6: 2, 7: 2, 8: 2, 9: 2, 11: 0, 12: 2, 20: 2, 26: 2, 32: 2, 35: 2, 38: 0}
var callSpec = map[int]int{1: 2, 2: 2, 3: 2}
var userSpec = map[int]int{2: 2, 3: 2, 4: 2, 12: 2, 18: 2}
var plannerSpec = map[int]int{1: 2, 3: 2, 6: 2, 7: 2, 8: 2, 11: 2, 12: 0}
var systemSpec = map[int]int{1: 2, 2: 2, 3: 2, 4: 2}
var agentSpec = map[int]int{1: 2, 2: 2, 3: 2, 4: 2, 5: 0, 6: 2, 10: 2, 12: 2}
var genericSpec = map[int]int{1: 2, 2: 2}
var resultSpec = map[int]int{1: 2, 3: 2, 6: 2, 7: 2}

func project(blob []byte, typ, status int64, record historysource.Record) ([]historysource.Record, []historysource.Gap, error) {
	root, err := decode(blob, stepSpec)
	if err != nil {
		return nil, nil, err
	}
	if root.ints[1] != uint64(typ) || root.ints[4] != uint64(status) {
		return nil, nil, historysource.ErrFormat
	}
	unknown := root.unknown
	var gaps []historysource.Gap
	gap := func(category string) {
		gaps = append(gaps, historysource.Gap{RecordID: record.ID, Category: category, Offset: record.Offset})
	}
	meta, err := root.child(5, metadataSpec)
	if err != nil {
		return nil, nil, err
	}
	unknown = unknown || meta.unknown
	setCall := func(m message, r *historysource.Record) error {
		unknown = unknown || m.unknown
		var e error
		r.CallID, e = m.text(1)
		if e != nil {
			return e
		}
		r.ToolName, e = m.text(2)
		if e != nil {
			return e
		}
		r.Text, e = m.text(3)
		return e
	}
	var extras []historysource.Record
	switch typ {
	case 14:
		m, e := root.child(19, userSpec)
		if e != nil {
			return nil, nil, e
		}
		unknown = unknown || m.unknown
		record.Kind, record.Role = "message", "user"
		record.Text, err = m.text(2)
		if err == nil && record.Text == "" {
			var text strings.Builder
			for _, b := range m.fields[3] {
				item, e := decode(b, map[int]int{1: 2})
				if e != nil {
					return nil, nil, e
				}
				unknown = unknown || item.unknown
				t, e := item.text(1)
				if e != nil {
					return nil, nil, e
				}
				if text.Len() != 0 {
					text.WriteByte('\n')
				}
				text.WriteString(t)
			}
			record.Text = text.String()
		}
	case 15:
		m, e := root.child(20, plannerSpec)
		if e != nil {
			return nil, nil, e
		}
		unknown = unknown || m.unknown
		record.Kind, record.Role = "message", "assistant"
		record.Text, err = m.text(8)
		if err == nil && record.Text == "" {
			record.Text, err = m.text(1)
		}
		if len(m.fields[3]) > 0 {
			gap("reasoning-omitted")
		}
		for i, b := range m.fields[7] {
			call, e := decode(b, callSpec)
			if e != nil {
				return nil, nil, e
			}
			r := historysource.Record{ID: record.ID + "/call/" + decimal(int64(i)), ParentID: record.ID, Kind: "tool_call", Role: "assistant", Status: record.Status, Offset: record.Offset, End: record.End}
			if e = setCall(call, &r); e != nil {
				return nil, nil, e
			}
			extras = append(extras, r)
		}
	case 132:
		m, e := root.child(140, genericSpec)
		if e != nil {
			return nil, nil, e
		}
		unknown = unknown || m.unknown
		result, e := m.child(2, resultSpec)
		if e != nil {
			return nil, nil, e
		}
		unknown = unknown || result.unknown
		record.Kind, record.Role = "tool_result", "tool"
		call, e := meta.child(4, callSpec)
		if e != nil {
			return nil, nil, e
		}
		if e = setCall(call, &record); e != nil {
			return nil, nil, e
		}
		record.Text, err = result.text(1)
		if len(result.fields[6]) > 0 {
			gap("opaque-payload-omitted")
		}
	case 101:
		m, e := root.child(114, systemSpec)
		if e != nil {
			return nil, nil, e
		}
		unknown = unknown || m.unknown
		record.Kind, record.Role = "message", "system"
		record.Text, err = m.text(1)
		a, e := m.child(4, agentSpec)
		if e != nil {
			return nil, nil, e
		}
		unknown = unknown || a.unknown
		if record.Text == "" && err == nil {
			record.Text, err = a.text(4)
		}
		source, e := a.child(12, map[int]int{1: 2})
		if e != nil {
			return nil, nil, e
		}
		unknown = unknown || source.unknown
		tool, e := source.child(1, map[int]int{1: 2, 2: 0, 3: 2})
		if e != nil {
			return nil, nil, e
		}
		unknown = unknown || tool.unknown
		call, e := tool.child(3, callSpec)
		if e != nil {
			return nil, nil, e
		}
		var ref historysource.Record
		if e = setCall(call, &ref); e != nil {
			return nil, nil, e
		}
		record.CallID, record.ToolName = ref.CallID, ref.ToolName
	default:
		record.Kind = "gap"
		gap("unsupported-step-type")
	}
	if err != nil {
		return nil, nil, err
	}
	if unknown {
		gap("unknown-protobuf-field")
	}
	return append([]historysource.Record{record}, extras...), gaps, nil
}
