package transport

import (
	"errors"
	"net/url"
	"slices"
)

var ErrInvalidQuery = errors.New("invalid query parameters")

// ParseQuery rejects malformed escaping, unknown or repeated keys, and empty
// values. Callers retain their own domain error mapping and value validation.
func ParseQuery(raw string, allowed ...string) (url.Values, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return nil, ErrInvalidQuery
	}
	for key, entries := range values {
		if !slices.Contains(allowed, key) || len(entries) != 1 || entries[0] == "" {
			return nil, ErrInvalidQuery
		}
	}
	return values, nil
}
