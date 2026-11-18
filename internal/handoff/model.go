package handoff

import (
	"context"
	"errors"
	"time"

	"github.com/rainchan/webmux/internal/historyimport"
	"github.com/rainchan/webmux/internal/historyprojection"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/launch"
	"github.com/rainchan/webmux/internal/sessioncreate"
)

var (
	ErrInvalid     = errors.New("invalid handoff request")
	ErrExpired     = errors.New("handoff preview expired; create a new preview")
	ErrConflict    = errors.New("handoff request conflicts with the consumed preview")
	ErrBudget      = errors.New("交接内容超过目标上下文预算")
	ErrLimit       = errors.New("handoff memory capacity reached")
	ErrUnavailable = errors.New("handoff unavailable")
)

type Runtime interface {
	Record(string) (hybrid.Record, error)
	Prompt(context.Context, string, hybrid.PromptInput) (hybrid.PromptResult, error)
}
type Configs interface {
	Get(string) (launch.Config, error)
}
type Options struct {
	Hybrid                                       Runtime
	Launch                                       Configs
	NativeHistory                                *historyimport.Service
	HistoryProjection                            *historyprojection.Service
	Create                                       func(context.Context, sessioncreate.Input) (sessioncreate.Result, error)
	TokenCap, MaxPromptBytes, MaxPromptWireBytes int64
}
type Location struct{ Cwd, RepositoryID, WorktreeID, NewWorktreeBase, NewWorktreeBranch string }
type Request struct {
	ConfigID, TargetSessionID, UserText string
	Location                            Location
	AgentConfig                         map[string]string
	Format                              string
	MaxPages                            int
}
type Preview struct {
	sourceIdentity                                                           string
	ID, InstanceID, OperationID, SourceSessionID, SnapshotID, Text, UserText string
	Request                                                                  Request
	CreatedAt, ExpiresAt                                                     time.Time
	SelectedCount, OmittedCount, GapCount                                    int
	Budget                                                                   Budget
	targetIdentity, configHash                                               string
	imageTextOffset                                                          int
	consumed                                                                 bool
	Format, BlockedReason, Capability, HistoryText                           string
	Pages                                                                    []ImagePage
	ImageBudget                                                              ImageBudget
}

type ImagePage struct {
	Index                  int
	MimeType, Data, SHA256 string
	Width, Height, Bytes   int
}

type ImageBudget struct {
	MaxPages, Pages, Bytes, MaxBytes, MaxRequestBytes int
	ProjectedRunes, RetainedRunes                     int
	EstimatedTokens                                   int64
	TokenEstimateKnown                                bool
	TokenBasis, Shape                                 string
}
type Operation struct {
	ID, PreviewID, InstanceID, State, TargetSessionID, Error string
	Location                                                 *sessioncreate.Location
	WorktreeUpstream                                         *sessioncreate.WorktreeUpstream
	StartedAt                                                time.Time
	FinishedAt                                               *time.Time
}
