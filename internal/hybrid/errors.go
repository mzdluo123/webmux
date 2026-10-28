package hybrid

import "errors"

// Error kinds; only internal/api/errors.go maps them to HTTP statuses.
var (
	// ErrNotFound: no hybrid session or record with that ID (404).
	ErrNotFound = errors.New("混合会话不存在")
	// ErrUnavailable: hybrid-sessions.json is locked, corrupt or unreadable,
	// or the service is shut down (503).
	ErrUnavailable = errors.New("混合会话不可用")
	// ErrLimit: the record limit is reached (409).
	ErrLimit = errors.New("混合会话记录已达上限，请先移除旧记录")
	// ErrConflict: the request conflicts with current state (409).
	ErrConflict = errors.New("混合会话状态冲突")
	// ErrInvalid: the request is malformed (400).
	ErrInvalid = errors.New("混合会话请求无效")
)
