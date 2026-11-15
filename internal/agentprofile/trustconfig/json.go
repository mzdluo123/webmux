package trustconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// EncodeJSON is the only JSON encoder of this package.
func EncodeJSON(v any) (json.RawMessage, error) { return json.Marshal(v) }

// ObjectOf decodes a JSON object into its raw members; empty input and null
// are an empty object and any other JSON type is an error.
func ObjectOf(raw []byte) (map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return map[string]json.RawMessage{}, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &object); err != nil {
		return nil, fmt.Errorf("not a JSON object: %w", err)
	}
	return object, nil
}

// IndentJSON renders root the way the agents do: two spaces and a final
// newline. Members other than the edited ones keep their original bytes.
func IndentJSON(root map[string]json.RawMessage) ([]byte, error) {
	encoded, err := EncodeJSON(root)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err = json.Indent(&out, encoded, "", "  "); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

// AppendPath adds dir to the JSON string array root[key], creating the array
// when it is missing. changed is false when dir is already listed.
func AppendPath(root map[string]json.RawMessage, key, dir string) (changed bool, err error) {
	var paths []json.RawMessage
	if raw := root[key]; len(raw) > 0 && string(raw) != "null" {
		if err = json.Unmarshal(raw, &paths); err != nil {
			return false, fmt.Errorf("%s: %w", key, err)
		}
	}
	for _, path := range paths {
		var existing string
		if json.Unmarshal(path, &existing) == nil && existing == dir {
			return false, nil
		}
	}
	entry, err := EncodeJSON(dir)
	if err != nil {
		return false, err
	}
	if root[key], err = EncodeJSON(append(paths, entry)); err != nil {
		return false, err
	}
	return true, nil
}
