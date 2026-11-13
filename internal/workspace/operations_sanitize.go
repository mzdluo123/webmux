package workspace

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	authHeaderPattern = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*(?:bearer|basic|token)?\s*)[^\s'"]+`)
	extraHdrPattern   = regexp.MustCompile(`(?i)(extraheader\s*[:=]\s*)[^\r\n]+`)
	knownTokenPattern = regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9_]{16,}|github_pat_[A-Za-z0-9_]{16,}|glpat-[A-Za-z0-9_-]{12,})\b`)
)

func validateOperationLedger(stored gitOperationLedger) error {
	if stored.V != gitOperationsVersion || stored.Operations == nil {
		return errors.New("不支持或损坏的 Git 操作账本")
	}
	if len(stored.Operations) > MaxOperationsGlobal {
		return fmt.Errorf("操作账本超过 %d 条记录", MaxOperationsGlobal)
	}
	ids := make(map[string]bool, len(stored.Operations))
	reqKeys := make(map[string]bool, len(stored.Operations))
	for _, op := range stored.Operations {
		if op.ID == "" || op.RepositoryID == "" || ids[op.ID] {
			return errors.New("操作记录身份无效或重复")
		}
		if !validOpKind(op.Kind) || !validOpStatus(op.Status) {
			return errors.New("操作记录类型或状态无效")
		}
		if len(op.SanitizedOutput) > MaxOperationOutputBytes {
			return errors.New("操作记录输出摘要超出上限")
		}
		ids[op.ID] = true
		if op.ClientRequestID != "" {
			if err := validateClientRequestID(op.ClientRequestID); err != nil {
				return errors.New("操作记录 clientRequestId 无效")
			}
			key := op.RepositoryID + "\x00" + op.ClientRequestID
			if reqKeys[key] {
				return errors.New("操作记录 clientRequestId 重复")
			}
			reqKeys[key] = true
		}
	}
	return nil
}

func validOpKind(kind string) bool {
	switch kind {
	case GitOpCommit, GitOpPush, GitOpFetch, GitOpFastForward, GitOpSwitch,
		GitOpMerge, GitOpRebase, GitOpCherryPick, GitOpRevert, GitOpContinue, GitOpAbort,
		GitOpStashPush, GitOpStashApply, GitOpStashPop, GitOpStashDrop,
		GitOpBranchRename, GitOpBranchUpstream,
		GitOpTagCreate, GitOpTagDelete,
		GitOpRemoteAdd, GitOpRemoteUpdate, GitOpRemoteRemove,
		GitOpDiscard, GitOpSmartCheckout:
		return true
	}
	return false
}

func validOpStatus(status string) bool {
	switch status {
	case GitOpPending, GitOpRunning, GitOpSucceeded, GitOpNeedsReview, GitOpFailed, GitOpUncertain:
		return true
	}
	return false
}

func validateClientRequestID(id string) error {
	if id == "" {
		return nil
	}
	if len(id) > MaxClientRequestIDLen {
		return problemf(ErrInvalid, "clientRequestId 长度不能超过 %d 个字符", MaxClientRequestIDLen)
	}
	for i := 0; i < len(id); i++ {
		if id[i] < 0x21 || id[i] > 0x7e {
			return problemf(ErrInvalid, "clientRequestId 包含非法字符")
		}
	}
	return nil
}

func hashOperationRequest(parts ...string) string {
	h := sha256.New()
	var buf [8]byte
	for _, p := range parts {
		binary.BigEndian.PutUint64(buf[:], uint64(len(p)))
		_, _ = h.Write(buf[:])
		_, _ = h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func sanitizeOperationOutput(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	rawLines := strings.Split(raw, "\n")
	cleaned := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimRight(line, "\r")
		if idx := strings.LastIndexByte(line, '\r'); idx >= 0 {
			line = line[idx+1:]
		}
		line = redactLine(line)
		line = authHeaderPattern.ReplaceAllString(line, "${1}"+redactedSecret)
		line = extraHdrPattern.ReplaceAllString(line, "${1}"+redactedSecret)
		line = knownTokenPattern.ReplaceAllString(line, redactedSecret)
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	out := strings.TrimSpace(strings.Join(cleaned, "\n"))
	if len(out) <= MaxOperationOutputBytes {
		return out
	}
	const ellipsis = "…"
	limit := MaxOperationOutputBytes - len(ellipsis)
	cut := strings.ToValidUTF8(out[:limit], "")
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut + ellipsis
}
