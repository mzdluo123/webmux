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
	// ErrWrongSide: the command needs the other side, e.g. a prompt while
	// the session is in the terminal (409 wrong_side).
	ErrWrongSide = errors.New("混合会话不在该端")
	// ErrAnswered: the request was already answered or withdrawn (409
	// answered).
	ErrAnswered = errors.New("该请求已被答复")
	// ErrAgent: the agent failed to start or to open its session (502). The
	// message may carry the agent's stderr tail, for the requesting user
	// only: never log it.
	ErrAgent = errors.New("agent 启动失败")
)
